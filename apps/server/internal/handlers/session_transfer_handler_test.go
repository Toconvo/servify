package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"servify/apps/server/internal/models"
	"servify/apps/server/internal/services"
)

type stubAIForTransferHandler struct{}

func (s stubAIForTransferHandler) ProcessQuery(ctx context.Context, query string, sessionID string) (*services.AIResponse, error) {
	return &services.AIResponse{Content: "ok", Confidence: 1, Source: "ai"}, nil
}
func (s stubAIForTransferHandler) ShouldTransferToHuman(query string, sessionHistory []models.Message) bool {
	return false
}
func (s stubAIForTransferHandler) GetSessionSummary(messages []models.Message) (string, error) {
	return "summary", nil
}
func (s stubAIForTransferHandler) InitializeKnowledgeBase() {}
func (s stubAIForTransferHandler) GetStatus(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{"type": "stub"}
}

func newTestDBForSessionTransferHandler(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	dsn := "file:session_transfer_handler_" + name + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&models.User{},
		&models.Agent{},
		&models.Session{},
		&models.Message{},
		&models.TransferRecord{},
		&models.WaitingRecord{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

func TestSessionTransferHandler_ListWaiting_And_Cancel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newTestDBForSessionTransferHandler(t)
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	if err := db.Create(&models.User{ID: 1, Username: "u1", Name: "u1", Email: "u1@example.com", Role: "customer"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.Session{ID: "s1", UserID: 1, Status: "active", Platform: "web", StartedAt: now, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed session: %v", err)
	}
	if err := db.Create(&models.WaitingRecord{SessionID: "s1", Reason: "r", Priority: "normal", Status: "waiting", QueuedAt: now, CreatedAt: now}).Error; err != nil {
		t.Fatalf("seed waiting: %v", err)
	}

	agentSvc := services.NewAgentService(db, logger)
	transferSvc := services.NewSessionTransferService(db, logger, stubAIForTransferHandler{}, agentSvc, nil)
	h := NewSessionTransferHandler(transferSvc, logger)

	r := gin.New()
	api := r.Group("/api")
	RegisterSessionTransferRoutes(api, h)

	// list waiting
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/session-transfer/waiting?status=waiting&limit=10", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list waiting status=%d body=%s", w.Code, w.Body.String())
	}
	var listResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list: %v body=%s", err, w.Body.String())
	}
	if int(listResp["count"].(float64)) != 1 {
		t.Fatalf("expected count=1 got %v", listResp["count"])
	}

	// cancel waiting
	w2 := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"session_id": "s1", "reason": "no_need"})
	req2, _ := http.NewRequest(http.MethodPost, "/api/session-transfer/cancel", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", w2.Code, w2.Body.String())
	}

	var wr models.WaitingRecord
	if err := db.Where("session_id = ?", "s1").First(&wr).Error; err != nil {
		t.Fatalf("load waiting record: %v", err)
	}
	if wr.Status != "cancelled" {
		t.Fatalf("expected cancelled got %s", wr.Status)
	}
}
