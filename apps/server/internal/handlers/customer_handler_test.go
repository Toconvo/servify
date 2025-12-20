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

func newTestDBForCustomers(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:customer_handler?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	// CustomerService.GetCustomerByID preloads Sessions and Tickets.
	if err := db.AutoMigrate(
		&models.User{},
		&models.Customer{},
		&models.Session{},
		&models.Ticket{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	return db
}

func TestCustomerHandler_Create_Get_Update_List(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newTestDBForCustomers(t)
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	svc := services.NewCustomerService(db, logger)
	h := NewCustomerHandler(svc, logger)

	r := gin.New()
	r.POST("/api/customers", h.CreateCustomer)
	r.GET("/api/customers", h.ListCustomers)
	r.GET("/api/customers/:id", h.GetCustomer)
	r.PUT("/api/customers/:id", h.UpdateCustomer)

	// Create
	createBody := map[string]any{
		"username": "c2",
		"email":    "c2@example.com",
		"name":     "c2 name",
		"company":  "acme",
	}
	b, _ := json.Marshal(createBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/customers", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}
	var created models.User
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created: %v body=%s", err, w.Body.String())
	}
	if created.ID == 0 {
		t.Fatalf("expected created customer id")
	}

	// Get
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/customers/"+toStr(created.ID), nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", w2.Code, w2.Body.String())
	}

	// Update
	newName := "c2 name updated"
	updateBody := map[string]any{"name": newName}
	bu, _ := json.Marshal(updateBody)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodPut, "/api/customers/"+toStr(created.ID), bytes.NewReader(bu))
	req3.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", w3.Code, w3.Body.String())
	}

	// List (no search/ILIKE filters to keep sqlite compatibility)
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest(http.MethodGet, "/api/customers?page=1&page_size=10", nil)
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", w4.Code, w4.Body.String())
	}
}
