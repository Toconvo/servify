package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"servify/apps/server/internal/services"
)

// MockEnhancedAIService 用于测试增强AI服务的处理器
type MockEnhancedAIService struct {
	*services.AIService
	weKnoraEnabled bool
	metrics        *services.AIMetrics
}

func (m *MockEnhancedAIService) ProcessQueryEnhanced(ctx context.Context, query, sessionID string) (*services.EnhancedAIResponse, error) {
	return &services.EnhancedAIResponse{
		AIResponse: &services.AIResponse{
			Content:    "Mock enhanced response",
			Confidence: 0.9,
		},
		Strategy: "mock",
	}, nil
}

func (m *MockEnhancedAIService) UploadDocumentToWeKnora(ctx context.Context, title, content string, tags []string) error {
	return nil
}

func (m *MockEnhancedAIService) SyncKnowledgeBase(ctx context.Context) error {
	return nil
}

func (m *MockEnhancedAIService) GetMetrics() *services.AIMetrics {
	if m.metrics == nil {
		return &services.AIMetrics{
			QueryCount:         10,
			WeKnoraUsageCount:  5,
			FallbackUsageCount: 2,
			AverageLatency:     100 * time.Millisecond,
		}
	}
	return m.metrics
}

func (m *MockEnhancedAIService) SetWeKnoraEnabled(enabled bool) {
	m.weKnoraEnabled = enabled
}

func (m *MockEnhancedAIService) SetFallbackEnabled(enabled bool) {
	// Mock implementation
}

func (m *MockEnhancedAIService) ResetCircuitBreaker() {
	// Mock implementation
}

func TestAIHandler_GetMetrics_EnhancedService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockEnhancedAIService{
		AIService:      services.NewAIService("", ""),
		weKnoraEnabled: true,
	}
	mockService.InitializeKnowledgeBase()
	handler := NewAIHandler(mockService)

	router := gin.New()
	router.GET("/api/v1/ai/metrics", handler.GetMetrics)

	req := httptest.NewRequest("GET", "/api/v1/ai/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["success"] != true {
		t.Error("expected success=true")
	}
	if response["data"] == nil {
		t.Error("expected data to be present")
	}
}

func TestAIHandler_GetMetrics_StandardService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	baseService := services.NewAIService("", "")
	baseService.InitializeKnowledgeBase()
	handler := NewAIHandler(baseService)

	router := gin.New()
	router.GET("/api/v1/ai/metrics", handler.GetMetrics)

	req := httptest.NewRequest("GET", "/api/v1/ai/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAIHandler_UploadDocument_EnhancedService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockEnhancedAIService{
		AIService:      services.NewAIService("", ""),
		weKnoraEnabled: true,
	}
	mockService.InitializeKnowledgeBase()
	handler := NewAIHandler(mockService)

	router := gin.New()
	router.POST("/api/v1/ai/upload", handler.UploadDocument)

	payload := map[string]interface{}{
		"title":   "Test Document",
		"content": "This is test content",
		"tags":    []string{"test", "document"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/ai/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestAIHandler_UploadDocument_InvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockEnhancedAIService{
		AIService:      services.NewAIService("", ""),
		weKnoraEnabled: true,
	}
	mockService.InitializeKnowledgeBase()
	handler := NewAIHandler(mockService)

	router := gin.New()
	router.POST("/api/v1/ai/upload", handler.UploadDocument)

	// Missing title
	payload := map[string]interface{}{
		"content": "This is test content",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/ai/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAIHandler_UploadDocument_StandardService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	baseService := services.NewAIService("", "")
	baseService.InitializeKnowledgeBase()
	handler := NewAIHandler(baseService)

	router := gin.New()
	router.POST("/api/v1/ai/upload", handler.UploadDocument)

	payload := map[string]interface{}{
		"title":   "Test Document",
		"content": "This is test content",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/ai/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAIHandler_SyncKnowledgeBase_EnhancedService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockEnhancedAIService{
		AIService:      services.NewAIService("", ""),
		weKnoraEnabled: true,
	}
	mockService.InitializeKnowledgeBase()
	handler := NewAIHandler(mockService)

	router := gin.New()
	router.POST("/api/v1/ai/sync", handler.SyncKnowledgeBase)

	req := httptest.NewRequest("POST", "/api/v1/ai/sync", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestAIHandler_SyncKnowledgeBase_StandardService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	baseService := services.NewAIService("", "")
	baseService.InitializeKnowledgeBase()
	handler := NewAIHandler(baseService)

	router := gin.New()
	router.POST("/api/v1/ai/sync", handler.SyncKnowledgeBase)

	req := httptest.NewRequest("POST", "/api/v1/ai/sync", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAIHandler_EnableWeKnora_EnhancedService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockEnhancedAIService{
		AIService:      services.NewAIService("", ""),
		weKnoraEnabled: false,
	}
	mockService.InitializeKnowledgeBase()
	handler := NewAIHandler(mockService)

	router := gin.New()
	router.POST("/api/v1/ai/weknora/enable", handler.EnableWeKnora)

	req := httptest.NewRequest("POST", "/api/v1/ai/weknora/enable", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !mockService.weKnoraEnabled {
		t.Error("expected WeKnora to be enabled")
	}
}

func TestAIHandler_EnableWeKnora_StandardService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	baseService := services.NewAIService("", "")
	baseService.InitializeKnowledgeBase()
	handler := NewAIHandler(baseService)

	router := gin.New()
	router.POST("/api/v1/ai/weknora/enable", handler.EnableWeKnora)

	req := httptest.NewRequest("POST", "/api/v1/ai/weknora/enable", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAIHandler_DisableWeKnora_EnhancedService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockEnhancedAIService{
		AIService:      services.NewAIService("", ""),
		weKnoraEnabled: true,
	}
	mockService.InitializeKnowledgeBase()
	handler := NewAIHandler(mockService)

	router := gin.New()
	router.POST("/api/v1/ai/weknora/disable", handler.DisableWeKnora)

	req := httptest.NewRequest("POST", "/api/v1/ai/weknora/disable", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if mockService.weKnoraEnabled {
		t.Error("expected WeKnora to be disabled")
	}
}

func TestAIHandler_DisableWeKnora_StandardService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	baseService := services.NewAIService("", "")
	baseService.InitializeKnowledgeBase()
	handler := NewAIHandler(baseService)

	router := gin.New()
	router.POST("/api/v1/ai/weknora/disable", handler.DisableWeKnora)

	req := httptest.NewRequest("POST", "/api/v1/ai/weknora/disable", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAIHandler_ResetCircuitBreaker_EnhancedService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockEnhancedAIService{
		AIService:      services.NewAIService("", ""),
		weKnoraEnabled: true,
	}
	mockService.InitializeKnowledgeBase()
	handler := NewAIHandler(mockService)

	router := gin.New()
	router.POST("/api/v1/ai/circuit-breaker/reset", handler.ResetCircuitBreaker)

	req := httptest.NewRequest("POST", "/api/v1/ai/circuit-breaker/reset", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestAIHandler_ResetCircuitBreaker_StandardService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	baseService := services.NewAIService("", "")
	baseService.InitializeKnowledgeBase()
	handler := NewAIHandler(baseService)

	router := gin.New()
	router.POST("/api/v1/ai/circuit-breaker/reset", handler.ResetCircuitBreaker)

	req := httptest.NewRequest("POST", "/api/v1/ai/circuit-breaker/reset", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAIHandler_ProcessQuery_EnhancedService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockEnhancedAIService{
		AIService:      services.NewAIService("", ""),
		weKnoraEnabled: true,
	}
	mockService.InitializeKnowledgeBase()
	handler := NewAIHandler(mockService)

	router := gin.New()
	router.POST("/api/v1/ai/query", handler.ProcessQuery)

	payload := map[string]string{
		"query":      "测试查询",
		"session_id": "test-session",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/ai/query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestAIHandler_ProcessQuery_MissingQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockEnhancedAIService{
		AIService:      services.NewAIService("", ""),
		weKnoraEnabled: true,
	}
	mockService.InitializeKnowledgeBase()
	handler := NewAIHandler(mockService)

	router := gin.New()
	router.POST("/api/v1/ai/query", handler.ProcessQuery)

	payload := map[string]string{
		"session_id": "test-session",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/ai/query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAIHandler_ProcessQuery_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &MockEnhancedAIService{
		AIService:      services.NewAIService("", ""),
		weKnoraEnabled: true,
	}
	mockService.InitializeKnowledgeBase()
	handler := NewAIHandler(mockService)

	router := gin.New()
	router.POST("/api/v1/ai/query", handler.ProcessQuery)

	req := httptest.NewRequest("POST", "/api/v1/ai/query", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
