package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"servify/apps/server/internal/config"
	"servify/apps/server/internal/services"
)

func TestEnhancedHealthHandler_Health_And_Ready(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := config.GetDefaultConfig()
	// Keep tests hermetic: skip external deps checks.
	cfg.Monitoring.HealthChecks.Database = false
	cfg.Monitoring.HealthChecks.Redis = false
	cfg.Monitoring.HealthChecks.WeKnora = false
	cfg.WeKnora.Enabled = false

	ai := services.NewAIService("", "")
	ai.InitializeKnowledgeBase()

	h := NewEnhancedHealthHandler(cfg, ai)

	r := gin.New()
	r.GET("/health", h.Health)
	r.GET("/ready", h.Ready)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("health status=%d body=%s", w.Code, w.Body.String())
	}

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/ready", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("ready status=%d body=%s", w2.Code, w2.Body.String())
	}
}
