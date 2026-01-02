package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"servify/apps/server/internal/models"
	"servify/apps/server/internal/services"
)

func newTestDBForStatistics(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:statistics_handler?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	// StatisticsService touches these tables.
	if err := db.AutoMigrate(
		&models.User{},
		&models.Agent{},
		&models.Ticket{},
		&models.Session{},
		&models.Message{},
		&models.DailyStats{},
		&models.Customer{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	return db
}

func TestStatisticsHandler_Dashboard_And_TimeRange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newTestDBForStatistics(t)
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	svc := services.NewStatisticsService(db, logger)
	h := NewStatisticsHandler(svc, logger)

	r := gin.New()
	r.GET("/api/statistics/dashboard", h.GetDashboardStats)
	r.GET("/api/statistics/time-range", h.GetTimeRangeStats)

	// Dashboard should succeed even with empty DB.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/statistics/dashboard", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("dashboard status=%d body=%s", w.Code, w.Body.String())
	}

	// Missing params should fail fast.
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/statistics/time-range", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("time-range missing params status=%d body=%s", w2.Code, w2.Body.String())
	}

	// Valid params should return 200.
	today := time.Now().Format("2006-01-02")
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/api/statistics/time-range?start_date="+today+"&end_date="+today, nil)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("time-range ok status=%d body=%s", w3.Code, w3.Body.String())
	}
}

func TestStatisticsHandler_GetAgentPerformanceStats_SQLiteError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newTestDBForStatistics(t)
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	svc := services.NewStatisticsService(db, logger)
	h := NewStatisticsHandler(svc, logger)

	r := gin.New()
	r.GET("/api/statistics/agent-performance", h.GetAgentPerformanceStats)

	// SQLite doesn't support PostgreSQL's EXTRACT function, so this will return 500
	today := time.Now().Format("2006-01-02")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/statistics/agent-performance?start_date="+today+"&end_date="+today, nil)
	r.ServeHTTP(w, req)

	// Should return 500 due to SQLite not supporting the SQL query
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 (SQLite doesn't support PostgreSQL EXTRACT), got %d, body: %s", w.Code, w.Body.String())
	}
}


func TestStatisticsHandler_GetTicketCategoryStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newTestDBForStatistics(t)
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	svc := services.NewStatisticsService(db, logger)
	h := NewStatisticsHandler(svc, logger)

	r := gin.New()
	r.GET("/api/statistics/ticket-category", h.GetTicketCategoryStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/statistics/ticket-category", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestStatisticsHandler_GetTicketPriorityStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newTestDBForStatistics(t)
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	svc := services.NewStatisticsService(db, logger)
	h := NewStatisticsHandler(svc, logger)

	r := gin.New()
	r.GET("/api/statistics/ticket-priority", h.GetTicketPriorityStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/statistics/ticket-priority", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestStatisticsHandler_GetCustomerSourceStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newTestDBForStatistics(t)
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	svc := services.NewStatisticsService(db, logger)
	h := NewStatisticsHandler(svc, logger)

	r := gin.New()
	r.GET("/api/statistics/customer-source", h.GetCustomerSourceStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/statistics/customer-source", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}
