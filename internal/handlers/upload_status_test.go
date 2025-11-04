package handlers

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
)

func TestUploadStatus_OK(t *testing.T) {
    gin.SetMode(gin.TestMode)
    h := NewUploadHandler(nil, nil)
    r := gin.New()
    r.GET("/api/v1/upload/status/:id", h.GetUploadStatus)
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/api/v1/upload/status/abc123", nil)
    r.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
    }
}

