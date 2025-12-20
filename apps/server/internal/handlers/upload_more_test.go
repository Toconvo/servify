package handlers

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"servify/apps/server/internal/config"
)

// build multipart request with one file field
func buildMultipart(url, field, filename string, content []byte) (*http.Request, error) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		return nil, err
	}
	_ = w.Close()
	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, nil
}

func newUploadRouterForTest(t *testing.T, cfg *config.Config) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// Use a temp dir to avoid polluting the repo when `go test` runs from this package dir.
	cfg.Upload.Enabled = true
	cfg.Upload.StoragePath = t.TempDir()
	_ = os.MkdirAll(cfg.Upload.StoragePath, 0o755)
	h := NewUploadHandler(cfg, nil)
	r := gin.New()
	r.POST("/api/v1/upload", h.UploadFile)
	return r
}

func TestUpload_NoFileProvided(t *testing.T) {
	cfg := config.GetDefaultConfig()
	r := newUploadRouterForTest(t, cfg)

	// 不传文件字段
	req, _ := http.NewRequest("POST", "/api/v1/upload", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when no file provided, got %d", w.Code)
	}
}

func TestUpload_MaxSizeBoundary_OK(t *testing.T) {
	cfg := config.GetDefaultConfig()
	cfg.Upload.AllowedTypes = []string{"*"}
	cfg.Upload.MaxFileSize = "1024" // exactly 1KB
	r := newUploadRouterForTest(t, cfg)

	payload := bytes.Repeat([]byte("x"), 1024)
	req, err := buildMultipart("/api/v1/upload", "file", "bin.dat", payload)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("boundary file should pass, got status %d body=%s", w.Code, w.Body.String())
	}
}

func TestUpload_EmptyFile_OK(t *testing.T) {
	cfg := config.GetDefaultConfig()
	cfg.Upload.AllowedTypes = []string{"*"}
	r := newUploadRouterForTest(t, cfg)

	req, err := buildMultipart("/api/v1/upload", "file", "empty.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("empty file should be accepted, got %d", w.Code)
	}
}

func TestUpload_MimePrefixMismatch_Reject(t *testing.T) {
	cfg := config.GetDefaultConfig()
	cfg.Upload.AllowedTypes = []string{"image/*"} // 仅允许 image/
	cfg.Upload.MaxFileSize = "10MB"
	r := newUploadRouterForTest(t, cfg)

	// 扩展名 .json，multipart 默认 octet-stream，不匹配 image/*
	req, err := buildMultipart("/api/v1/upload", "file", "data.json", []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for mime prefix mismatch, got %d", w.Code)
	}
}
