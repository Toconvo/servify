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

func newTestDBForTickets(t *testing.T) *gorm.DB {
	t.Helper()

	// Use shared in-memory DB; TicketService spawns goroutines (auto-assign) that may
	// use a different connection.
	db, err := gorm.Open(sqlite.Open("file:ticket_handler?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	// TicketService.GetTicketByID preloads these associations; keep schema in sync.
	if err := db.AutoMigrate(
		&models.User{},
		&models.Agent{},
		&models.Session{},
		&models.Ticket{},
		&models.TicketStatus{},
		&models.TicketComment{},
		&models.TicketFile{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	return db
}

func TestTicketHandler_Create_Get_List_Assign(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newTestDBForTickets(t)
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// Seed a customer and an agent user; ticket creation validates customer existence.
	if err := db.Create(&models.User{ID: 1, Username: "c1", Name: "c1", Email: "c1@example.com", Role: "customer"}).Error; err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	if err := db.Create(&models.User{ID: 2, Username: "a1", Name: "a1", Email: "a1@example.com", Role: "agent"}).Error; err != nil {
		t.Fatalf("seed agent user: %v", err)
	}
	if err := db.Create(&models.Agent{UserID: 2, Status: "online", MaxConcurrent: 5, CurrentLoad: 0}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	if err := db.Create(&models.User{ID: 3, Username: "a2", Name: "a2", Email: "a2@example.com", Role: "agent"}).Error; err != nil {
		t.Fatalf("seed agent user 2: %v", err)
	}
	if err := db.Create(&models.Agent{UserID: 3, Status: "online", MaxConcurrent: 5, CurrentLoad: 0}).Error; err != nil {
		t.Fatalf("seed agent 2: %v", err)
	}

	ticketSvc := services.NewTicketService(db, logger, nil)
	h := NewTicketHandler(ticketSvc, logger)

	r := gin.New()
	r.POST("/api/tickets", h.CreateTicket)
	r.POST("/api/tickets/bulk", h.BulkUpdateTickets)
	r.GET("/api/tickets", h.ListTickets)
	r.GET("/api/tickets/:id", h.GetTicket)
	r.POST("/api/tickets/:id/assign", h.AssignTicket)

	// Create ticket
	createBody := map[string]any{
		"title":       "hello",
		"description": "desc",
		"customer_id": 1,
		"priority":    "normal",
		"category":    "general",
	}
	b, _ := json.Marshal(createBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/tickets", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}
	var created models.Ticket
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created: %v body=%s", err, w.Body.String())
	}
	if created.ID == 0 {
		t.Fatalf("expected created ticket id")
	}

	// Get ticket
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/tickets/"+toStr(created.ID), nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", w2.Code, w2.Body.String())
	}

	// List tickets (no filters)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/api/tickets?page=1&page_size=10", nil)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", w3.Code, w3.Body.String())
	}

	// Assign ticket to agent (agent_id here is user_id per service logic).
	assignBody := map[string]any{"agent_id": 2}
	b2, _ := json.Marshal(assignBody)
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest(http.MethodPost, "/api/tickets/"+toStr(created.ID)+"/assign", bytes.NewReader(b2))
	req4.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("assign status=%d body=%s", w4.Code, w4.Body.String())
	}

	// Re-assign (transfer) to another agent; should decrement old load and increment new load.
	assignBody2 := map[string]any{"agent_id": 3}
	bTransfer, _ := json.Marshal(assignBody2)
	w4b := httptest.NewRecorder()
	req4b, _ := http.NewRequest(http.MethodPost, "/api/tickets/"+toStr(created.ID)+"/assign", bytes.NewReader(bTransfer))
	req4b.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w4b, req4b)
	if w4b.Code != http.StatusOK {
		t.Fatalf("transfer assign status=%d body=%s", w4b.Code, w4b.Body.String())
	}

	var agent2 models.Agent
	if err := db.Where("user_id = ?", 2).First(&agent2).Error; err != nil {
		t.Fatalf("load agent 2: %v", err)
	}
	var agent3 models.Agent
	if err := db.Where("user_id = ?", 3).First(&agent3).Error; err != nil {
		t.Fatalf("load agent 3: %v", err)
	}
	if agent2.CurrentLoad != 0 {
		t.Fatalf("expected agent 2 load decremented to 0, got %d", agent2.CurrentLoad)
	}
	if agent3.CurrentLoad != 1 {
		t.Fatalf("expected agent 3 load incremented to 1, got %d", agent3.CurrentLoad)
	}

	// Bulk update: add tags + set status
	bulkBody := map[string]any{
		"ticket_ids":  []uint{created.ID},
		"status":     "resolved",
		"add_tags":   []string{"vip", "urgent"},
		"remove_tags": []string{"non-existent"},
	}
	b3, _ := json.Marshal(bulkBody)
	w5 := httptest.NewRecorder()
	req5, _ := http.NewRequest(http.MethodPost, "/api/tickets/bulk", bytes.NewReader(b3))
	req5.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Fatalf("bulk status=%d body=%s", w5.Code, w5.Body.String())
	}

	var updated models.Ticket
	if err := db.First(&updated, created.ID).Error; err != nil {
		t.Fatalf("load ticket after bulk: %v", err)
	}
	if updated.Status != "resolved" {
		t.Fatalf("expected status resolved, got %q", updated.Status)
	}
	if updated.Tags == "" {
		t.Fatalf("expected tags to be set after bulk update")
	}

	// Bulk unassign
	bulkUnassign := map[string]any{
		"ticket_ids":      []uint{created.ID},
		"unassign_agent":  true,
		"remove_tags":     []string{"vip"},
		"add_tags":        []string{"after-unassign"},
	}
	b4, _ := json.Marshal(bulkUnassign)
	w6 := httptest.NewRecorder()
	req6, _ := http.NewRequest(http.MethodPost, "/api/tickets/bulk", bytes.NewReader(b4))
	req6.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w6, req6)
	if w6.Code != http.StatusOK {
		t.Fatalf("bulk unassign status=%d body=%s", w6.Code, w6.Body.String())
	}
	var after models.Ticket
	if err := db.First(&after, created.ID).Error; err != nil {
		t.Fatalf("load ticket after bulk unassign: %v", err)
	}
	if after.AgentID != nil {
		t.Fatalf("expected agent_id to be nil after unassign")
	}
}

func toStr(v uint) string {
	// uint->string without fmt to keep the test dependency surface small.
	if v == 0 {
		return "0"
	}
	var b [32]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
