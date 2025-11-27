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

func newSLATestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Ticket{}, &models.SLAConfig{}, &models.SLAViolation{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestSLAService_CheckViolation_FirstResponse(t *testing.T) {
	db := newSLATestDB(t)
	svc := NewSLAService(db, logrus.New())

	now := time.Now()
	cfg := &models.SLAConfig{
		Name:              "High Priority",
		Priority:          "high",
		FirstResponseTime: 5,
		ResolutionTime:    60,
		EscalationTime:    30,
		Active:            true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatalf("failed to insert config: %v", err)
	}

	ticket := &models.Ticket{
		ID:        1,
		Title:     "Test",
		Priority:  "high",
		Status:    "open",
		CreatedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now.Add(-10 * time.Minute),
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("failed to insert ticket: %v", err)
	}

	violation, err := svc.CheckSLAViolation(context.Background(), ticket)
	if err != nil {
		t.Fatalf("CheckSLAViolation failed: %v", err)
	}
	if violation == nil || violation.ViolationType != "first_response" {
		t.Fatalf("expected first_response violation, got %+v", violation)
	}

	// running again should reuse existing violation
	violation2, err := svc.CheckSLAViolation(context.Background(), ticket)
	if err != nil {
		t.Fatalf("CheckSLAViolation second call failed: %v", err)
	}
	if violation2 == nil || violation2.ID != violation.ID {
		t.Fatalf("expected duplicate detection; got %#v", violation2)
	}
}

func TestSLAService_ResolveViolationsByTicket(t *testing.T) {
	db := newSLATestDB(t)
	svc := NewSLAService(db, logrus.New())

	now := time.Now()
	cfg := &models.SLAConfig{
		Name:              "Normal Priority",
		Priority:          "normal",
		FirstResponseTime: 10,
		ResolutionTime:    120,
		EscalationTime:    60,
		Active:            true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatalf("failed to insert config: %v", err)
	}

	ticket := &models.Ticket{
		ID:        2,
		Title:     "Need help",
		Priority:  "normal",
		Status:    "open",
		CreatedAt: now.Add(-3 * time.Hour),
		UpdatedAt: now.Add(-3 * time.Hour),
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("failed to insert ticket: %v", err)
	}

	// create violation manually via service
	if _, err := svc.CheckSLAViolation(context.Background(), ticket); err != nil {
		t.Fatalf("failed to create violation: %v", err)
	}

	if err := svc.ResolveViolationsByTicket(context.Background(), ticket.ID, []string{"first_response"}); err != nil {
		t.Fatalf("ResolveViolationsByTicket failed: %v", err)
	}

	var count int64
	if err := db.Model(&models.SLAViolation{}).Where("ticket_id = ? AND resolved = true", ticket.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count resolved violations: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected resolved violations, got zero")
	}
}
