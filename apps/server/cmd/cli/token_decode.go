package cli

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"servify/apps/server/internal/config"

	"github.com/spf13/cobra"
)

var (
	decToken   string
	decVerify  bool
	decSecret  string
	decShowSig bool
)

// decodeTokenCmd prints JWT header/payload; optionally verifies HS256 signature/time claims.
var decodeTokenCmd = &cobra.Command{
	Use:   "token-decode",
	Short: "Decode a JWT and optionally verify HS256 signature",
	Long:  "Decode a compact JWT (header.payload.signature). With --verify, check HS256 signature using jwt.secret from config (or --secret).",
	RunE: func(cmd *cobra.Command, args []string) error {
		token := decToken
		if token == "" && len(args) > 0 {
			token = args[0]
		}
		if token == "" {
			return errors.New("missing token (pass via --token or arg)")
		}
		header, payload, err := decodeJWT(token)
		if err != nil {
			return err
		}
		pretty := func(v any) string {
			b, _ := json.MarshalIndent(v, "", "  ")
			return string(b)
		}
		fmt.Println("Header:")
		fmt.Println(pretty(header))
		fmt.Println("Payload:")
		fmt.Println(pretty(payload))

		if decVerify {
			secret := decSecret
			if secret == "" {
				cfg := config.Load()
				secret = cfg.JWT.Secret
			}
			if secret == "" {
				return errors.New("no secret provided and jwt.secret empty in config")
			}
			validSig, validTime, sigHex, err := verifyHS256(token, secret, time.Now())
			if err != nil {
				fmt.Printf("Verify error: %v\n", err)
			}
			fmt.Printf("Signature valid: %v\n", validSig)
			fmt.Printf("Time claims valid (nbf/iat/exp): %v\n", validTime)
			if decShowSig {
				fmt.Printf("Computed signature (base64url): %s\n", sigHex)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(decodeTokenCmd)
	decodeTokenCmd.Flags().StringVar(&decToken, "token", "", "JWT to decode (compact form). If omitted, use first arg")
	decodeTokenCmd.Flags().BoolVar(&decVerify, "verify", false, "verify HS256 signature and time claims")
	decodeTokenCmd.Flags().StringVar(&decSecret, "secret", "", "secret for HS256 verify (default: jwt.secret in config)")
	decodeTokenCmd.Flags().BoolVar(&decShowSig, "show-computed-sig", false, "print computed signature (base64url)")
}

func decodeJWT(token string) (map[string]any, map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, errors.New("invalid token format")
	}
	dec := base64.RawURLEncoding
	hb, err := dec.DecodeString(parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("header decode: %w", err)
	}
	pb, err := dec.DecodeString(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("payload decode: %w", err)
	}
	var header map[string]any
	var payload map[string]any
	if err := json.Unmarshal(hb, &header); err != nil {
		return nil, nil, fmt.Errorf("header json: %w", err)
	}
	if err := json.Unmarshal(pb, &payload); err != nil {
		return nil, nil, fmt.Errorf("payload json: %w", err)
	}
	return header, payload, nil
}

func verifyHS256(token, secret string, now time.Time) (sigValid bool, timeValid bool, computedSigB64 string, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false, false, "", errors.New("invalid token format")
	}
	signing := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signing))
	comp := mac.Sum(nil)
	computedSigB64 = base64.RawURLEncoding.EncodeToString(comp)
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false, false, "", fmt.Errorf("signature decode: %w", err)
	}
	sigValid = hmac.Equal(sig, comp)

	// time claims
	_, payload, err := decodeJWT(token)
	if err != nil {
		return sigValid, false, computedSigB64, err
	}
	timeValid = checkTimeClaims(payload, now)
	return sigValid, timeValid, computedSigB64, nil
}

func checkTimeClaims(payload map[string]any, now time.Time) bool {
	nowSec := now.Unix()
	check := func(k string, cmp func(int64) bool) bool {
		if v, ok := payload[k]; ok {
			switch t := v.(type) {
			case float64:
				return cmp(int64(t))
			case json.Number:
				sec, _ := t.Int64()
				return cmp(sec)
			case string:
				// tolerate numeric string
				var n int64
				if _, err := fmt.Sscan(t, &n); err == nil {
					return cmp(n)
				}
			}
			return false
		}
		return true
	}
	return check("nbf", func(sec int64) bool { return nowSec >= sec }) &&
		check("iat", func(sec int64) bool { return nowSec >= sec }) &&
		check("exp", func(sec int64) bool { return nowSec < sec })
}

// Allow piping token via stdin: `echo $JWT | servify-cli token-decode`
func readStdinIfEmpty(s string) string {
	if s != "" {
		return s
	}
	fi, _ := os.Stdin.Stat()
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		b, _ := os.ReadFile("/dev/stdin")
		return strings.TrimSpace(string(b))
	}
	return s
}
