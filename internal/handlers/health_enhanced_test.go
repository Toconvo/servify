package handlers

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "servify/internal/config"
    "servify/internal/services"
)

func TestEnhancedHealth_Ready_Health(t *testing.T) {
    gin.SetMode(gin.TestMode)
    cfg := config.Load()
    ai := services.NewAIService("", "")
    ai.InitializeKnowledgeBase()
    h := NewEnhancedHealthHandler(cfg, ai)

    r := gin.New()
    r.GET("/health", h.Health)
    r.GET("/ready", h.Ready)

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/health", nil)
    r.ServeHTTP(w, req)
    if w.Code != http.StatusOK { t.Fatalf("health status: %d", w.Code) }

    w2 := httptest.NewRecorder()
    req2, _ := http.NewRequest("GET", "/ready", nil)
    r.ServeHTTP(w2, req2)
    if w2.Code != http.StatusOK { t.Fatalf("ready status: %d", w2.Code) }
}

