package handlers

import (
    "bytes"
    "encoding/json"
    "context"
    "strings"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "servify/internal/services"
)

func TestMetricsHandler_ExposesCounters(t *testing.T) {
    gin.SetMode(gin.TestMode)
    // set up enhanced AI and trigger one query to bump metrics
    base := services.NewAIService("", "")
    base.InitializeKnowledgeBase()
    enh := services.NewEnhancedAIService(base, nil, "kb", nil)
    _, _ = enh.ProcessQueryEnhanced(context.Background(), "ping", "s1")

    hub := services.NewWebSocketHub(); go hub.Run()
    wh := services.NewWebRTCService("stun:stun.l.google.com:19302", hub)

    r := gin.New()
    r.GET("/metrics", NewMetricsHandler(hub, wh, enh, nil).GetMetrics)

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/metrics", nil)
    r.ServeHTTP(w, req)
    if w.Code != http.StatusOK { t.Fatalf("status=%d", w.Code) }
    body := w.Body.String()
    if !strings.Contains(body, "servify_ai_requests_total 1") {
        t.Fatalf("expected ai requests counter in metrics, body=\n%s", body)
    }
}

func TestAIHandler_GetMetrics_NotEnhanced(t *testing.T) {
    gin.SetMode(gin.TestMode)
    base := services.NewAIService("", "")
    base.InitializeKnowledgeBase()
    h := NewAIHandler(base)
    r := gin.New()
    r.GET("/api/v1/ai/metrics", h.GetMetrics)
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/api/v1/ai/metrics", nil)
    r.ServeHTTP(w, req)
    if w.Code != http.StatusNotFound {
        t.Fatalf("expected 404 for standard AI metrics, got %d", w.Code)
    }
}

func TestAIHandler_UploadDocument_WeKnoraDisabled(t *testing.T) {
    gin.SetMode(gin.TestMode)
    base := services.NewAIService("", "")
    base.InitializeKnowledgeBase()
    enh := services.NewEnhancedAIService(base, nil, "kb", nil)
    h := NewAIHandler(enh)
    r := gin.New()
    r.POST("/api/v1/ai/knowledge/upload", h.UploadDocument)

    payload := map[string]interface{}{
        "title": "t",
        "content": "c",
        "tags": []string{"a"},
    }
    buf, _ := json.Marshal(payload)
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("POST", "/api/v1/ai/knowledge/upload", bytes.NewReader(buf))
    req.Header.Set("Content-Type", "application/json")
    r.ServeHTTP(w, req)
    if w.Code != http.StatusInternalServerError {
        t.Fatalf("expected 500 when weknora disabled, got %d", w.Code)
    }
}
