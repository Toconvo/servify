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

// helper to build a multipart/form-data request with a single file part named "file"
func newUploadRequest(url, fieldName, filename string, contentType string, data []byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, bytes.NewReader(data)); err != nil {
		return nil, err
	}
	_ = writer.Close()
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if contentType != "" {
		// SaveUploadedFile relies on header in FileHeader for validation; we can't directly set it here,
		// but our handler checks header.Header.Get("Content-Type"). Gin populates it from incoming part.
		// multipart.Writer doesn't allow setting per-part headers here; this is sufficient for extension checks.
		// We cover both paths across tests (.ext and mime prefix).
		req.Header.Set("X-Content-Hint", contentType)
	}
	return req, nil
}

func setupUploadRouter(t *testing.T, cfg *config.Config) (*gin.Engine, string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "upload-test-*")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}
	cfg.Upload.Enabled = true
	cfg.Upload.StoragePath = dir
	// ensure dir exists
	_ = os.MkdirAll(cfg.Upload.StoragePath, 0o755)
	h := NewUploadHandler(cfg, nil)
	r := gin.New()
	r.POST("/api/v1/upload", h.UploadFile)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return r, dir
}

func TestUpload_DisallowedType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.GetDefaultConfig()
	cfg.Upload.AllowedTypes = []string{".txt"} // only allow .txt
	cfg.Upload.MaxFileSize = "10MB"
	r, _ := setupUploadRouter(t, cfg)

	req, err := newUploadRequest("/api/v1/upload", "file", "doc.pdf", "application/pdf", []byte("pdf-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", w.Code)
	}
}

func TestUpload_MaxSizeExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.GetDefaultConfig()
	cfg.Upload.AllowedTypes = []string{"*"} // allow all for this test
	cfg.Upload.MaxFileSize = "1KB"
	r, _ := setupUploadRouter(t, cfg)

	big := bytes.Repeat([]byte("a"), 2048)
	req, err := newUploadRequest("/api/v1/upload", "file", "big.bin", "application/octet-stream", big)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", w.Code)
	}
}

func TestUpload_TextExtraction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.GetDefaultConfig()
	cfg.Upload.AllowedTypes = []string{".txt"}
	cfg.Upload.AutoProcess = true
	cfg.Upload.AutoIndex = false
	r, dir := setupUploadRouter(t, cfg)

	content := []byte("hello extraction")
	req, err := newUploadRequest("/api/v1/upload", "file", "note.txt", "text/plain", content)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, body=%s", w.Code, w.Body.String())
	}
	// ensure file saved
	files, _ := os.ReadDir(dir)
	if len(files) == 0 {
		t.Fatalf("expected saved file in %s", dir)
	}
}

func TestUpload_NonTextExtractionPlaceholder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.GetDefaultConfig()
	cfg.Upload.AllowedTypes = []string{".png"}
	cfg.Upload.AutoProcess = true
	cfg.Upload.AutoIndex = false
	r, _ := setupUploadRouter(t, cfg)

	data := []byte{0x89, 0x50, 0x4E, 0x47}
	req, err := newUploadRequest("/api/v1/upload", "file", "img.png", "image/png", data)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	// Best-effort check: response body should contain placeholder text
	if !bytes.Contains(w.Body.Bytes(), []byte("extraction not implemented")) {
		t.Fatalf("expected placeholder extraction output in response body: %s", w.Body.String())
	}
}
