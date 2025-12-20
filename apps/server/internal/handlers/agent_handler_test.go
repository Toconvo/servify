package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"servify/apps/server/internal/models"
	"servify/apps/server/internal/services"
)

func newTestDBForAgents(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:agent_handler?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(
		&models.User{},
		&models.Agent{},
		&models.Ticket{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	return db
}

func TestAgentHandler_Create_Online_Status_Offline(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newTestDBForAgents(t)
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// Seed a base user to be promoted to agent.
	if err := db.Create(&models.User{
		ID:       10,
		Username: "agent10",
		Email:    "agent10@example.com",
		Name:     "agent10",
		Role:     "customer",
		Status:   "active",
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	svc := services.NewAgentService(db, logger)
	h := NewAgentHandler(svc, logger)

	r := gin.New()
	r.POST("/api/agents", h.CreateAgent)
	r.POST("/api/agents/:id/online", h.AgentGoOnline)
	r.PUT("/api/agents/:id/status", h.UpdateAgentStatus)
	r.GET("/api/agents/online", h.GetOnlineAgents)
	r.POST("/api/agents/:id/offline", h.AgentGoOffline)

	// Create agent for user 10
	createBody := map[string]any{"user_id": 10, "department": "support", "skills": "billing,tech", "max_concurrent": 3}
	b, _ := json.Marshal(createBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/agents", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}

	// Go online
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodPost, "/api/agents/10/online", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("online status=%d body=%s", w2.Code, w2.Body.String())
	}

	// Update status
	updateBody := map[string]any{"status": "busy"}
	bu, _ := json.Marshal(updateBody)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodPut, "/api/agents/10/status", bytes.NewReader(bu))
	req3.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("status update status=%d body=%s", w3.Code, w3.Body.String())
	}

	// Online list
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest(http.MethodGet, "/api/agents/online", nil)
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("online list status=%d body=%s", w4.Code, w4.Body.String())
	}

	// Go offline
	w5 := httptest.NewRecorder()
	req5, _ := http.NewRequest(http.MethodPost, "/api/agents/10/offline", nil)
	r.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Fatalf("offline status=%d body=%s", w5.Code, w5.Body.String())
	}
}
