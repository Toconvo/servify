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

func newTestDB(t *testing.T, tables ...interface{}) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if len(tables) == 0 {
		tables = []interface{}{&models.ShiftSchedule{}}
	}
	if err := db.AutoMigrate(tables...); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestShiftService_Create_and_GetStats(t *testing.T) {
	db := newTestDB(t, &models.ShiftSchedule{})
	svc := NewShiftService(db, logrus.New())

	now := time.Now()
	req := &ShiftCreateRequest{
		AgentID:   1,
		ShiftType: "morning",
		StartTime: now.Add(time.Hour),
		EndTime:   now.Add(4 * time.Hour),
		Status:    "scheduled",
	}
	_, err := svc.CreateShift(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateShift failed: %v", err)
	}

	stats, err := svc.GetShiftStats(context.Background())
	if err != nil {
		t.Fatalf("GetShiftStats failed: %v", err)
	}
	if stats.Total != 1 {
		t.Fatalf("expected total 1, got %d", stats.Total)
	}
	if stats.ByType["morning"] != 1 {
		t.Fatalf("expected morning count 1, got %d", stats.ByType["morning"])
	}
	if stats.ByStatus["scheduled"] != 1 {
		t.Fatalf("expected scheduled count 1, got %d", stats.ByStatus["scheduled"])
	}
}

func TestShiftService_Create_Validation(t *testing.T) {
	db := newTestDB(t, &models.ShiftSchedule{})
	svc := NewShiftService(db, logrus.New())

	now := time.Now()
	req := &ShiftCreateRequest{
		AgentID:   1,
		ShiftType: "morning",
		StartTime: now,
		EndTime:   now.Add(-time.Hour), // invalid: end before start
		Status:    "scheduled",
	}
	if _, err := svc.CreateShift(context.Background(), req); err == nil {
		t.Fatalf("expected validation error for end before start")
	}
}

func TestShiftService_Update_and_Delete(t *testing.T) {
	db := newTestDB(t, &models.ShiftSchedule{})
	svc := NewShiftService(db, logrus.New())

	now := time.Now()
	shift, err := svc.CreateShift(context.Background(), &ShiftCreateRequest{
		AgentID:   2,
		ShiftType: "night",
		StartTime: now,
		EndTime:   now.Add(3 * time.Hour),
		Status:    "scheduled",
	})
	if err != nil {
		t.Fatalf("CreateShift failed: %v", err)
	}

	newStatus := "active"
	updated, err := svc.UpdateShift(context.Background(), shift.ID, &ShiftUpdateRequest{
		Status: &newStatus,
	})
	if err != nil {
		t.Fatalf("UpdateShift failed: %v", err)
	}
	if updated.Status != newStatus {
		t.Fatalf("expected status %s, got %s", newStatus, updated.Status)
	}

	if err := svc.DeleteShift(context.Background(), shift.ID); err != nil {
		t.Fatalf("DeleteShift failed: %v", err)
	}
	if err := svc.DeleteShift(context.Background(), shift.ID); err == nil {
		t.Fatalf("expected error deleting non-existent shift")
	}
}
