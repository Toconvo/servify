package handlers

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "servify/internal/services"
)

func TestWebSocketStatsEndpoint(t *testing.T) {
    gin.SetMode(gin.TestMode)
    hub := services.NewWebSocketHub()
    go hub.Run()
    r := gin.New()

    ws := NewWebSocketHandler(hub)
    r.GET("/api/v1/ws/stats", ws.GetStats)

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/api/v1/ws/stats", nil)
    r.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("unexpected status: %d", w.Code)
    }
    var body map[string]interface{}
    if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
        t.Fatalf("invalid json: %v", err)
    }
    if body["success"] != true {
        t.Fatalf("expected success=true, got %v", body["success"])
    }
}

