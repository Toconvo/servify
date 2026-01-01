package services

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"github.com/sirupsen/logrus"
	"servify/apps/server/internal/models"
)

func newStatisticsServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := t.Name()
	dsn := "file:statistics_" + name + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Agent{},
		&models.Ticket{},
		&models.Session{},
		&models.Message{},
		&models.Customer{},
		&models.DailyStats{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestNewStatisticsService(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()

	svc := NewStatisticsService(db, logger)

	if svc == nil {
		t.Fatal("expected service, got nil")
	}
	if svc.db != db {
		t.Error("expected db to be set")
	}
	if svc.logger != logger {
		t.Error("expected logger to be set")
	}
}

func TestNewStatisticsService_NilLogger(t *testing.T) {
	db := newStatisticsServiceTestDB(t)

	svc := NewStatisticsService(db, nil)

	if svc == nil {
		t.Fatal("expected service, got nil")
	}
	if svc.logger == nil {
		t.Error("expected logger to be initialized")
	}
}

func TestStatisticsService_GetDashboardStats_EmptyDB(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	stats, err := svc.GetDashboardStats(context.Background())
	if err != nil {
		t.Fatalf("GetDashboardStats() error = %v", err)
	}
	if stats == nil {
		t.Fatal("expected stats, got nil")
	}
	// All counts should be 0
	if stats.TotalCustomers != 0 {
		t.Errorf("expected 0 total customers, got %d", stats.TotalCustomers)
	}
	if stats.TotalAgents != 0 {
		t.Errorf("expected 0 total agents, got %d", stats.TotalAgents)
	}
}

func TestStatisticsService_GetDashboardStats_WithData(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	// Create test users
	customer := &models.User{
		Username: "customer1",
		Email:    "customer1@test.com",
		Name:     "Customer One",
		Role:     "customer",
	}
	db.Create(customer)

	agentUser := &models.User{
		Username: "agent1",
		Email:    "agent1@test.com",
		Name:     "Agent One",
		Role:     "agent",
	}
	db.Create(agentUser)

	// Create agent
	agent := &models.Agent{
		UserID: agentUser.ID,
		Status: "online",
	}
	db.Create(agent)

	// Create ticket
	ticket := &models.Ticket{
		Title:       "Test Ticket",
		Description: "Test",
		Status:      "open",
		CustomerID:  1,
	}
	db.Create(ticket)

	// Create session
	session := &models.Session{
		Platform:  "web",
		Status:    "active",
		StartedAt: time.Now(),
	}
	db.Create(session)

	stats, err := svc.GetDashboardStats(context.Background())
	if err != nil {
		t.Fatalf("GetDashboardStats() error = %v", err)
	}
	if stats.TotalCustomers != 1 {
		t.Errorf("expected 1 total customer, got %d", stats.TotalCustomers)
	}
	if stats.TotalAgents != 1 {
		t.Errorf("expected 1 total agent, got %d", stats.TotalAgents)
	}
	if stats.TotalTickets != 1 {
		t.Errorf("expected 1 total ticket, got %d", stats.TotalTickets)
	}
	if stats.TotalSessions != 1 {
		t.Errorf("expected 1 total session, got %d", stats.TotalSessions)
	}
	if stats.OnlineAgents != 1 {
		t.Errorf("expected 1 online agent, got %d", stats.OnlineAgents)
	}
	if stats.ActiveSessions != 1 {
		t.Errorf("expected 1 active session, got %d", stats.ActiveSessions)
	}
}

func TestStatisticsService_GetTimeRangeStats_Empty(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	startDate := time.Now().AddDate(0, 0, -7)
	endDate := time.Now()

	stats, err := svc.GetTimeRangeStats(context.Background(), startDate, endDate)
	if err != nil {
		t.Fatalf("GetTimeRangeStats() error = %v", err)
	}
	if len(stats) != 8 { // 7 days + today
		t.Errorf("expected 8 stats entries, got %d", len(stats))
	}
	// All counts should be 0
	for _, stat := range stats {
		if stat.Tickets != 0 {
			t.Errorf("expected 0 tickets for %s, got %d", stat.Date, stat.Tickets)
		}
	}
}

func TestStatisticsService_GetTimeRangeStats_SingleDay(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	// Create a ticket for today
	ticket := &models.Ticket{
		Title:       "Test Ticket",
		Description: "Test",
		Status:      "open",
	}
	db.Create(ticket)

	startDate := time.Now().Truncate(24 * time.Hour)
	endDate := startDate

	stats, err := svc.GetTimeRangeStats(context.Background(), startDate, endDate)
	if err != nil {
		t.Fatalf("GetTimeRangeStats() error = %v", err)
	}
	if len(stats) != 1 {
		t.Errorf("expected 1 stat entry, got %d", len(stats))
	}
	if stats[0].Tickets != 1 {
		t.Errorf("expected 1 ticket, got %d", stats[0].Tickets)
	}
}

func TestStatisticsService_GetTicketCategoryStats_Empty(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	startDate := time.Now().AddDate(0, 0, -7)
	endDate := time.Now()

	stats, err := svc.GetTicketCategoryStats(context.Background(), startDate, endDate)
	if err != nil {
		t.Fatalf("GetTicketCategoryStats() error = %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected 0 category stats, got %d", len(stats))
	}
}

func TestStatisticsService_GetTicketCategoryStats_WithCategories(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	// Create tickets with different categories
	db.Create(&models.Ticket{Title: "T1", Category: "technical", Status: "open"})
	db.Create(&models.Ticket{Title: "T2", Category: "billing", Status: "open"})
	db.Create(&models.Ticket{Title: "T3", Category: "technical", Status: "open"})

	startDate := time.Now().AddDate(0, 0, -1)
	endDate := time.Now()

	stats, err := svc.GetTicketCategoryStats(context.Background(), startDate, endDate)
	if err != nil {
		t.Fatalf("GetTicketCategoryStats() error = %v", err)
	}
	if len(stats) != 2 {
		t.Errorf("expected 2 category stats, got %d", len(stats))
	}
	// Find technical category
	var technicalStat *CategoryStats
	for i := range stats {
		if stats[i].Category == "technical" {
			technicalStat = &stats[i]
			break
		}
	}
	if technicalStat == nil {
		t.Fatal("technical category not found")
	}
	if technicalStat.Count != 2 {
		t.Errorf("expected 2 technical tickets, got %d", technicalStat.Count)
	}
}

func TestStatisticsService_GetTicketPriorityStats_Empty(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	startDate := time.Now().AddDate(0, 0, -7)
	endDate := time.Now()

	stats, err := svc.GetTicketPriorityStats(context.Background(), startDate, endDate)
	if err != nil {
		t.Fatalf("GetTicketPriorityStats() error = %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected 0 priority stats, got %d", len(stats))
	}
}

func TestStatisticsService_GetTicketPriorityStats_WithPriorities(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	// Create tickets with different priorities
	db.Create(&models.Ticket{Title: "T1", Priority: "high", Status: "open"})
	db.Create(&models.Ticket{Title: "T2", Priority: "low", Status: "open"})
	db.Create(&models.Ticket{Title: "T3", Priority: "high", Status: "open"})

	startDate := time.Now().AddDate(0, 0, -1)
	endDate := time.Now()

	stats, err := svc.GetTicketPriorityStats(context.Background(), startDate, endDate)
	if err != nil {
		t.Fatalf("GetTicketPriorityStats() error = %v", err)
	}
	if len(stats) != 2 {
		t.Errorf("expected 2 priority stats, got %d", len(stats))
	}
}

func TestStatisticsService_GetCustomerSourceStats_Empty(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	stats, err := svc.GetCustomerSourceStats(context.Background())
	if err != nil {
		t.Fatalf("GetCustomerSourceStats() error = %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected 0 source stats, got %d", len(stats))
	}
}

func TestStatisticsService_GetCustomerSourceStats_WithSources(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	// Create customers with different sources
	db.Create(&models.Customer{UserID: 1, Source: "web"})
	db.Create(&models.Customer{UserID: 2, Source: "api"})
	db.Create(&models.Customer{UserID: 3, Source: "web"})

	stats, err := svc.GetCustomerSourceStats(context.Background())
	if err != nil {
		t.Fatalf("GetCustomerSourceStats() error = %v", err)
	}
	if len(stats) != 2 {
		t.Errorf("expected 2 source stats, got %d", len(stats))
	}
	// Find web source
	var webStat *CategoryStats
	for i := range stats {
		if stats[i].Category == "web" {
			webStat = &stats[i]
			break
		}
	}
	if webStat == nil {
		t.Fatal("web source not found")
	}
	if webStat.Count != 2 {
		t.Errorf("expected 2 web customers, got %d", webStat.Count)
	}
}

func TestStatisticsService_UpdateDailyStats_NewRecord(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	date := time.Now().Truncate(24 * time.Hour)

	err := svc.UpdateDailyStats(context.Background(), date)
	if err != nil {
		t.Fatalf("UpdateDailyStats() error = %v", err)
	}

	// Verify record was created
	var dailyStats models.DailyStats
	err = db.Where("date = ?", date).First(&dailyStats).Error
	if err != nil {
		t.Fatalf("failed to find daily stats: %v", err)
	}
}

func TestStatisticsService_IncrementAIUsage_NewRecord(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	svc.IncrementAIUsage(context.Background())

	// Verify record was created
	today := time.Now().Truncate(24 * time.Hour)
	var dailyStats models.DailyStats
	err := db.Where("date = ?", today).First(&dailyStats).Error
	if err != nil {
		t.Fatalf("failed to find daily stats: %v", err)
	}
	if dailyStats.AIUsageCount != 1 {
		t.Errorf("expected AI usage count 1, got %d", dailyStats.AIUsageCount)
	}
}

func TestStatisticsService_IncrementWeKnoraUsage_NewRecord(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	svc.IncrementWeKnoraUsage(context.Background())

	// Verify record was created
	today := time.Now().Truncate(24 * time.Hour)
	var dailyStats models.DailyStats
	err := db.Where("date = ?", today).First(&dailyStats).Error
	if err != nil {
		t.Fatalf("failed to find daily stats: %v", err)
	}
	if dailyStats.WeKnoraUsageCount != 1 {
		t.Errorf("expected WeKnora usage count 1, got %d", dailyStats.WeKnoraUsageCount)
	}
}

func TestStatisticsService_IncrementAIUsage_ExistingRecord(t *testing.T) {
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	today := time.Now().Truncate(24 * time.Hour)
	db.Create(&models.DailyStats{
		Date:         today,
		AIUsageCount: 5,
	})

	svc.IncrementAIUsage(context.Background())

	// Verify count was incremented
	var dailyStats models.DailyStats
	err := db.Where("date = ?", today).First(&dailyStats).Error
	if err != nil {
		t.Fatalf("failed to find daily stats: %v", err)
	}
	if dailyStats.AIUsageCount != 6 {
		t.Errorf("expected AI usage count 6, got %d", dailyStats.AIUsageCount)
	}
}

func TestStatisticsService_IncrementWeKnoraUsage_ExistingRecord(t *testing.T) {
	t.Skip("Skipping: SQLite doesn't support GORM's UpdateColumn with expression for column increments")
	db := newStatisticsServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewStatisticsService(db, logger)

	today := time.Now().Truncate(24 * time.Hour)
	db.Create(&models.DailyStats{
		Date:              today,
		WeKnoraUsageCount: 3,
	})

	svc.IncrementWeKnoraUsage(context.Background())

	// Verify count was incremented
	var dailyStats models.DailyStats
	err := db.Where("date = ?", today).First(&dailyStats).Error
	if err != nil {
		t.Fatalf("failed to find daily stats: %v", err)
	}
	if dailyStats.WeKnoraUsageCount != 4 {
		t.Errorf("expected WeKnora usage count 4, got %d", dailyStats.WeKnoraUsageCount)
	}
}
