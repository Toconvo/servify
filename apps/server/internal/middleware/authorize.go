package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRolesAny returns a middleware that checks if the request context
// contains at least one of the required roles in "roles" (set by AuthMiddleware).
func RequireRolesAny(required ...string) gin.HandlerFunc {
	reqSet := make(map[string]struct{}, len(required))
	for _, r := range required {
		reqSet[r] = struct{}{}
	}
	return func(c *gin.Context) {
		var roles []string
		if v, ok := c.Get("roles"); ok {
			switch t := v.(type) {
			case []string:
				roles = t
			case []interface{}:
				for _, it := range t {
					if s, ok := it.(string); ok {
						roles = append(roles, s)
					}
				}
			case interface{}:
				// tolerate single role as string
				if s, ok := t.(string); ok && s != "" {
					roles = []string{s}
				}
			}
		}
		for _, r := range roles {
			if _, ok := reqSet[r]; ok {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "Forbidden",
			"message": "insufficient role",
		})
	}
}
