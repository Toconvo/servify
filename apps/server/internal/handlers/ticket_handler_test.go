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

	ticketSvc := services.NewTicketService(db, logger, nil)
	h := NewTicketHandler(ticketSvc, logger)

	r := gin.New()
	r.POST("/api/tickets", h.CreateTicket)
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
