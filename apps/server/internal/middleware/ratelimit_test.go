package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"servify/apps/server/internal/config"
)

func TestRateLimitMiddleware_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Security: config.SecurityConfig{
			RateLimiting: config.RateLimitingConfig{
				Enabled: false,
			},
		},
	}

	middleware := RateLimitMiddleware(cfg)
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// 应该允许所有请求
	for i := 0; i < 100; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i, w.Code)
		}
	}
}

func TestRateLimitMiddleware_BasicLimiting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Security: config.SecurityConfig{
			RateLimiting: config.RateLimitingConfig{
				Enabled:           true,
				RequestsPerMinute: 10,
				Burst:             5,
			},
		},
	}

	middleware := RateLimitMiddleware(cfg)
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// 第一个请求应该成功
	if w.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w.Code)
	}

	// 发送超过 burst 的请求
	allowed := 0
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			allowed++
		}
	}

	// 应该允许大约 burst 个请求
	if allowed < 3 || allowed > 6 {
		t.Errorf("expected 3-6 allowed requests, got %d", allowed)
	}
}

func TestRateLimitMiddleware_ZeroRate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Security: config.SecurityConfig{
			RateLimiting: config.RateLimitingConfig{
				Enabled:           true,
				RequestsPerMinute: 0,
			},
		},
	}

	middleware := RateLimitMiddleware(cfg)
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestTokenBucket_Allow(t *testing.T) {
	b := newBucket(60, 10) // 60 req/min, burst 10

	// 应该允许 burst 个请求
	for i := 0; i < 10; i++ {
		if !b.allow() {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// 下一个请求应该被拒绝
	if b.allow() {
		t.Error("request beyond burst should be denied")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	b := newBucket(600, 10) // 600 req/min = 10 req/sec

	// 消耗所有 tokens
	for i := 0; i < 10; i++ {
		b.allow()
	}

	// 应该被拒绝
	if b.allow() {
		t.Error("should be denied after exhausting tokens")
	}

	// 等待令牌补充
	time.Sleep(150 * time.Millisecond)

	// 现在应该允许一个请求
	if !b.allow() {
		t.Error("should allow after refill")
	}
}

func TestTokenBucket_ZeroParams(t *testing.T) {
	b := newBucket(0, 0) // 应该使用默认值

	// 应该有默认的 burst
	allowed := 0
	for i := 0; i < 100; i++ {
		if b.allow() {
			allowed++
		}
	}

	if allowed == 0 {
		t.Error("expected at least some requests to be allowed")
	}
}

func TestRateLimitMiddlewareFromConfig_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Security: config.SecurityConfig{
			RateLimiting: config.RateLimitingConfig{
				Enabled: false,
			},
		},
	}

	middleware := RateLimitMiddlewareFromConfig(cfg)
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// 应该允许所有请求
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i, w.Code)
		}
	}
}

func TestRateLimitMiddlewareFromConfig_WhitelistIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Security: config.SecurityConfig{
			RateLimiting: config.RateLimitingConfig{
				Enabled:           true,
				RequestsPerMinute: 10,
				Burst:             2,
				WhitelistIPs:      []string{"127.0.0.1"},
			},
		},
	}

	middleware := RateLimitMiddlewareFromConfig(cfg)
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// 白名单 IP 应该允许所有请求
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200 (whitelisted), got %d", i, w.Code)
		}
	}
}

func TestRateLimitMiddlewareFromConfig_PathBased(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Security: config.SecurityConfig{
			RateLimiting: config.RateLimitingConfig{
				Enabled:           true,
				RequestsPerMinute: 100,
				Burst:             50,
				Paths: []config.PathRateLimitConfig{
					{
						Enabled:           true,
						Prefix:            "/api/",
						RequestsPerMinute: 5,
						Burst:             2,
					},
				},
			},
		},
	}

	middleware := RateLimitMiddlewareFromConfig(cfg)
	router := gin.New()
	router.Use(middleware)
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/other", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// /api/ 路径应该受到严格限制
	allowed := 0
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/test", nil)
		router.ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			allowed++
		}
	}
	if allowed > 3 {
		t.Errorf("/api/ allowed %d requests, expected at most 3", allowed)
	}

	// /other 路径应该使用全局限制（更宽松）
	allowed = 0
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/other", nil)
		router.ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			allowed++
		}
	}
	if allowed < 5 {
		t.Errorf("/other allowed %d requests, expected at least 5", allowed)
	}
}
