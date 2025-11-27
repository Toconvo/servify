package services

import (
	"context"
	"testing"
	"time"

	"servify/apps/server/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSatisfactionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Ticket{}, &models.CustomerSatisfaction{}, &models.SatisfactionSurvey{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestSatisfactionService_ScheduleAndRespondSurvey(t *testing.T) {
	db := newSatisfactionTestDB(t)
	logger := logrus.New()
	svc := NewSatisfactionService(db, logger)

	now := time.Now()
	user := &models.User{ID: 1, Username: "customer1", Email: "c1@example.com"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	ticket := &models.Ticket{
		ID:         1,
		Title:      "测试工单",
		CustomerID: user.ID,
		Status:     "closed",
		Source:     "web",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	survey, err := svc.ScheduleSurvey(context.Background(), ticket)
	if err != nil {
		t.Fatalf("ScheduleSurvey failed: %v", err)
	}
	if survey == nil || survey.SurveyToken == "" {
		t.Fatalf("expected survey with token, got %+v", survey)
	}
	if survey.Status != "sent" {
		t.Fatalf("expected status sent, got %s", survey.Status)
	}

	satisfaction, err := svc.RespondSurvey(context.Background(), survey.SurveyToken, 5, "great job")
	if err != nil {
		t.Fatalf("RespondSurvey failed: %v", err)
	}
	if satisfaction == nil || satisfaction.Rating != 5 {
		t.Fatalf("unexpected satisfaction result: %+v", satisfaction)
	}

	var savedSurvey models.SatisfactionSurvey
	if err := db.First(&savedSurvey, survey.ID).Error; err != nil {
		t.Fatalf("load saved survey: %v", err)
	}
	if savedSurvey.Status != "completed" {
		t.Fatalf("expected completed status, got %s", savedSurvey.Status)
	}
	if savedSurvey.SatisfactionID == nil {
		t.Fatalf("expected satisfaction_id recorded")
	}
}

func TestSatisfactionService_GetSurveyPreview(t *testing.T) {
	db := newSatisfactionTestDB(t)
	logger := logrus.New()
	svc := NewSatisfactionService(db, logger)

	now := time.Now()
	user := &models.User{ID: 2, Username: "customer2", Email: "c2@example.com"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	ticket := &models.Ticket{
		ID:         2,
		Title:      "预览工单",
		CustomerID: user.ID,
		Status:     "closed",
		Source:     "email",
		ResolvedAt: &now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	survey, err := svc.ScheduleSurvey(context.Background(), ticket)
	if err != nil {
		t.Fatalf("ScheduleSurvey failed: %v", err)
	}

	preview, err := svc.GetSurveyPreviewByToken(context.Background(), survey.SurveyToken)
	if err != nil {
		t.Fatalf("GetSurveyPreviewByToken failed: %v", err)
	}
	if preview == nil || preview.TicketTitle != ticket.Title {
		t.Fatalf("unexpected preview result: %+v", preview)
	}
	if preview.Status != "sent" {
		t.Fatalf("expected sent status, got %s", preview.Status)
	}
	if preview.ResolvedAt == nil {
		t.Fatalf("expected resolved time in preview")
	}
}
