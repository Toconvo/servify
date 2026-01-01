package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"servify/apps/server/internal/config"

	"github.com/gin-gonic/gin"
)

func TestAuthMiddleware_Extended(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		JWT: config.JWTConfig{Secret: "test-secret"},
		Security: config.SecurityConfig{
			RBAC: config.RBACConfig{
				Enabled: false,
			},
		},
	}

	r := gin.New()
	r.Use(AuthMiddleware(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	tests := []struct {
		name           string
		authHeader     string
		wantStatusCode int
	}{
		{
			name:           "missing authorization header",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "invalid bearer format",
			authHeader:     "Basic token-value",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "only bearer prefix",
			authHeader:     "Bearer ",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "malformed jwt",
			authHeader:     "Bearer not.a.valid.jwt",
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatusCode)
			}
		})
	}
}

func TestRequireResourcePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name            string
		permission      string
		userPermissions []string
		wantStatusCode  int
	}{
		{
			name:            "has read permission for GET",
			permission:      "tickets",
			userPermissions: []string{"tickets.read"},
			wantStatusCode:  http.StatusOK,
		},
		{
			name:            "missing permission",
			permission:      "tickets",
			userPermissions: []string{},
			wantStatusCode:  http.StatusForbidden,
		},
		{
			name:            "wildcard permission",
			permission:      "tickets",
			userPermissions: []string{"*"},
			wantStatusCode:  http.StatusOK,
		},
		{
			name:            "resource wildcard permission",
			permission:      "tickets",
			userPermissions: []string{"tickets.*"},
			wantStatusCode:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("permissions", tt.userPermissions)
				c.Next()
			})
			r.Use(RequireResourcePermission(tt.permission))
			r.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("status code = %d, want %d", w.Code, tt.wantStatusCode)
			}
		})
	}
}

func TestCORSMiddleware(t *testing.T) {
	t.Skip("CORS middleware not implemented yet")
}

func TestRBACMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		permissions    []string
		permission     string
		wantStatusCode int
	}{
		{
			name:           "admin with wildcard",
			permissions:    []string{"*"},
			permission:     "tickets",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "agent with write permission",
			permissions:    []string{"tickets.write", "tickets.read"},
			permission:     "tickets",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "viewer without write permission but has read",
			permissions:    []string{"tickets.read"},
			permission:     "tickets",
			wantStatusCode: http.StatusOK, // GET checks .read, viewer has it
		},
		{
			name:           "viewer with no permissions",
			permissions:    []string{},
			permission:     "tickets",
			wantStatusCode: http.StatusForbidden, // No permissions at all
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("permissions", tt.permissions)
				c.Next()
			})
			r.Use(RequireResourcePermission(tt.permission))
			r.GET("/tickets", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/tickets", nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatusCode)
			}
		})
	}
}
