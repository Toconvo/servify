package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"servify/apps/server/internal/config"

	"github.com/gin-gonic/gin"
)

// validateHS256JWT verifies an HS256 JWT and returns its payload as a generic map.
// It performs minimal validation:
// - signature (HS256) using cfg JWT secret
// - exp/nbf/iat (if present)
// - returns claims map for caller to extract useful fields (e.g. sub/user_id/roles)
func validateHS256JWT(token, secret string, now time.Time) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}
	headerB64, payloadB64, sigB64 := parts[0], parts[1], parts[2]

	// Decode header and verify alg == HS256 (optional but recommended)
	headerJSON, err := base64.RawURLEncoding.DecodeString(headerB64)
	if err != nil {
		return nil, errors.New("invalid header encoding")
	}
	var header map[string]interface{}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, errors.New("invalid header json")
	}
	if alg, _ := header["alg"].(string); alg != "" && alg != "HS256" {
		return nil, errors.New("unsupported alg")
	}

	// Verify signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(headerB64 + "." + payloadB64))
	expected := mac.Sum(nil)
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, errors.New("invalid signature encoding")
	}
	if !hmac.Equal(sig, expected) {
		return nil, errors.New("invalid signature")
	}

	// Decode payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, errors.New("invalid payload encoding")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, errors.New("invalid payload json")
	}

	// Validate time-based claims (if present)
	checkTime := func(key string, cmp func(int64) bool) error {
		if v, ok := payload[key]; ok {
			switch t := v.(type) {
			case float64:
				if !cmp(int64(t)) {
					return errors.New("token time constraint failed: " + key)
				}
			case json.Number:
				sec, _ := t.Int64()
				if !cmp(sec) {
					return errors.New("token time constraint failed: " + key)
				}
			}
		}
		return nil
	}
	nowSec := now.Unix()
	if err := checkTime("nbf", func(sec int64) bool { return nowSec >= sec }); err != nil {
		return nil, err
	}
	if err := checkTime("iat", func(sec int64) bool { return nowSec >= sec }); err != nil {
		return nil, err
	}
	if err := checkTime("exp", func(sec int64) bool { return nowSec < sec }); err != nil {
		return nil, err
	}

	return payload, nil
}

// AuthMiddleware enforces Authorization: Bearer <jwt> on protected routes.
// On success, it injects "user_id" and "roles" into gin.Context for handlers.
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	secret := ""
	var rbac config.RBACConfig
	if cfg != nil {
		secret = cfg.JWT.Secret
		rbac = cfg.Security.RBAC
	}
	return func(c *gin.Context) {
		ah := c.GetHeader("Authorization")
		if !strings.HasPrefix(strings.ToLower(ah), "bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "missing bearer token",
			})
			return
		}
		token := strings.TrimSpace(ah[len("Bearer "):])
		if token == "" || secret == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "invalid token or server misconfig",
			})
			return
		}
		claims, err := validateHS256JWT(token, secret, time.Now())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": err.Error(),
			})
			return
		}
		// Extract common fields
		// sub/user_id as uint if possible, fallback to string presence
		var uidAny interface{}
		if v, ok := claims["user_id"]; ok {
			uidAny = v
		} else if v, ok := claims["sub"]; ok {
			uidAny = v
		}
		// Attempt to normalize "user_id" to uint (if numeric)
		switch t := uidAny.(type) {
		case float64:
			c.Set("user_id", uint(t))
		case json.Number:
			if n, err := t.Int64(); err == nil {
				c.Set("user_id", uint(n))
			}
		default:
			// keep as-is (string/other), some handlers may not strictly require uint
			if uidAny != nil {
				c.Set("user_id_raw", uidAny)
			}
		}

		roles := normalizeStringList(claims["roles"])
		if len(roles) > 0 {
			c.Set("roles", roles)
		}

		// permissions (RBAC)
		// - supports explicit claim: perms/permissions
		// - expands role -> permissions from config.security.rbac.roles
		perms := normalizeStringList(firstNonNil(claims["perms"], claims["permissions"]))
		if rbac.Enabled {
			for _, role := range roles {
				for _, p := range rbac.Roles[role] {
					if s := strings.TrimSpace(p); s != "" {
						perms = append(perms, s)
					}
				}
			}
		} else {
			// Backward-compatible defaults when RBAC is not explicitly enabled
			for _, role := range roles {
				switch role {
				case "admin":
					perms = append(perms, "*")
				case "agent":
					perms = append(perms,
						"tickets.read", "tickets.write",
						"customers.read",
						"agents.read",
						"custom_fields.read",
						"session_transfer.read", "session_transfer.write",
						"satisfaction.read", "satisfaction.write",
						"workspace.read",
						"macros.read",
						"integrations.read",
					)
				}
			}
		}
		perms = dedupeStrings(perms)
		if len(perms) > 0 {
			c.Set("permissions", perms)
		}

		c.Next()
	}
}

func firstNonNil(vals ...interface{}) interface{} {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

func normalizeStringList(v interface{}) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []string:
		out := make([]string, 0, len(t))
		for _, s := range t {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []interface{}:
		var out []string
		for _, it := range t {
			if s, ok := it.(string); ok {
				if s = strings.TrimSpace(s); s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		parts := strings.Split(t, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
