package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// HasPermission returns true if `required` is satisfied by any permission in `granted`.
// Supported patterns:
// - "*" matches everything
// - "resource.*" matches "resource.<anything>"
// - exact match
func HasPermission(granted []string, required string) bool {
	required = strings.TrimSpace(required)
	if required == "" {
		return true
	}
	for _, p := range granted {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "*" {
			return true
		}
		if p == required {
			return true
		}
		if strings.HasSuffix(p, ".*") {
			prefix := strings.TrimSuffix(p, ".*")
			if prefix != "" && (required == prefix || strings.HasPrefix(required, prefix+".")) {
				return true
			}
		}
	}
	return false
}

func getGrantedPermissions(c *gin.Context) []string {
	if v, ok := c.Get("permissions"); ok {
		if perms, ok := v.([]string); ok {
			return perms
		}
	}
	return nil
}

// RequirePermissionsAny requires the caller to have at least one of the listed permissions.
func RequirePermissionsAny(required ...string) gin.HandlerFunc {
	req := make([]string, 0, len(required))
	for _, r := range required {
		if s := strings.TrimSpace(r); s != "" {
			req = append(req, s)
		}
	}
	return func(c *gin.Context) {
		granted := getGrantedPermissions(c)
		for _, r := range req {
			if HasPermission(granted, r) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "Forbidden",
			"message": "insufficient permission",
		})
	}
}

// RequirePermissionsAll requires the caller to have all listed permissions.
func RequirePermissionsAll(required ...string) gin.HandlerFunc {
	req := make([]string, 0, len(required))
	for _, r := range required {
		if s := strings.TrimSpace(r); s != "" {
			req = append(req, s)
		}
	}
	return func(c *gin.Context) {
		granted := getGrantedPermissions(c)
		for _, r := range req {
			if !HasPermission(granted, r) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error":   "Forbidden",
					"message": "insufficient permission",
				})
				return
			}
		}
		c.Next()
	}
}

// RequireResourcePermission enforces "<resource>.read" for safe methods and "<resource>.write" for mutating methods.
func RequireResourcePermission(resource string) gin.HandlerFunc {
	resource = strings.TrimSpace(resource)
	return func(c *gin.Context) {
		perm := resource + ".write"
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			perm = resource + ".read"
		}
		RequirePermissionsAny(perm, resource+".*", "*")(c)
	}
}
