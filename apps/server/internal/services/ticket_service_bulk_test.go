package services

import (
	"context"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"servify/apps/server/internal/models"
)

func newTestDBForTicketService(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:ticket_service_" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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
		&models.Session{},
		&models.Ticket{},
		&models.CustomField{},
		&models.TicketCustomFieldValue{},
		&models.TicketStatus{},
		&models.TicketComment{},
		&models.TicketFile{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

func TestTicketService_Assign_Transfer_Unassign(t *testing.T) {
	db := newTestDBForTicketService(t)

	if err := db.Create(&models.User{ID: 1, Username: "c1", Name: "c1", Email: "c1@example.com", Role: "customer"}).Error; err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	if err := db.Create(&models.User{ID: 2, Username: "a1", Name: "a1", Email: "a1@example.com", Role: "agent"}).Error; err != nil {
		t.Fatalf("seed agent user: %v", err)
	}
	if err := db.Create(&models.User{ID: 3, Username: "a2", Name: "a2", Email: "a2@example.com", Role: "agent"}).Error; err != nil {
		t.Fatalf("seed agent user 2: %v", err)
	}
	if err := db.Create(&models.Agent{UserID: 2, Status: "online", MaxConcurrent: 5, CurrentLoad: 0}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	if err := db.Create(&models.Agent{UserID: 3, Status: "online", MaxConcurrent: 5, CurrentLoad: 0}).Error; err != nil {
		t.Fatalf("seed agent 2: %v", err)
	}

	if err := db.Create(&models.Ticket{ID: 1, Title: "t1", CustomerID: 1, Status: "open", Priority: "normal"}).Error; err != nil {
		t.Fatalf("seed ticket: %v", err)
	}

	svc := NewTicketService(db, logrus.New(), nil)
	ctx := context.Background()

	if err := svc.AssignTicket(ctx, 1, 2, 99); err != nil {
		t.Fatalf("assign: %v", err)
	}
	var t1 models.Ticket
	if err := db.First(&t1, 1).Error; err != nil {
		t.Fatalf("load ticket: %v", err)
	}
	if t1.AgentID == nil || *t1.AgentID != 2 || t1.Status != "assigned" {
		t.Fatalf("expected assigned to agent 2, got agent_id=%v status=%q", t1.AgentID, t1.Status)
	}

	if err := svc.AssignTicket(ctx, 1, 3, 99); err != nil {
		t.Fatalf("transfer assign: %v", err)
	}
	var a2, a3 models.Agent
	_ = db.Where("user_id = ?", 2).First(&a2).Error
	_ = db.Where("user_id = ?", 3).First(&a3).Error
	if a2.CurrentLoad != 0 || a3.CurrentLoad != 1 {
		t.Fatalf("expected loads a2=0 a3=1, got a2=%d a3=%d", a2.CurrentLoad, a3.CurrentLoad)
	}

	if err := svc.UnassignTicket(ctx, 1, 99, ""); err != nil {
		t.Fatalf("unassign: %v", err)
	}
	var after models.Ticket
	if err := db.First(&after, 1).Error; err != nil {
		t.Fatalf("load ticket after unassign: %v", err)
	}
	if after.AgentID != nil || after.Status != "open" {
		t.Fatalf("expected unassigned and open, got agent_id=%v status=%q", after.AgentID, after.Status)
	}
	_ = db.Where("user_id = ?", 3).First(&a3).Error
	if a3.CurrentLoad != 0 {
		t.Fatalf("expected agent 3 load back to 0, got %d", a3.CurrentLoad)
	}
}

func TestTicketService_BulkUpdateTickets_Status_Tags_Assign(t *testing.T) {
	db := newTestDBForTicketService(t)

	if err := db.Create(&models.User{ID: 1, Username: "c1", Name: "c1", Email: "c1@example.com", Role: "customer"}).Error; err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	if err := db.Create(&models.User{ID: 2, Username: "a1", Name: "a1", Email: "a1@example.com", Role: "agent"}).Error; err != nil {
		t.Fatalf("seed agent user: %v", err)
	}
	if err := db.Create(&models.Agent{UserID: 2, Status: "online", MaxConcurrent: 5, CurrentLoad: 0}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	if err := db.Create(&models.Ticket{ID: 2, Title: "t2", CustomerID: 1, Status: "open", Priority: "normal", Tags: "a,b"}).Error; err != nil {
		t.Fatalf("seed ticket: %v", err)
	}

	svc := NewTicketService(db, logrus.New(), nil)
	ctx := context.Background()

	_, err := svc.BulkUpdateTickets(ctx, &TicketBulkUpdateRequest{
		TicketIDs:  []uint{2},
		Status:     stringPtr("resolved"),
		AddTags:    []string{"c"},
		RemoveTags: []string{"a"},
		AgentID:    uintPtr(2),
	}, 99)
	if err != nil {
		t.Fatalf("bulk update: %v", err)
	}

	var updated models.Ticket
	if err := db.First(&updated, 2).Error; err != nil {
		t.Fatalf("load ticket: %v", err)
	}
	if updated.Status != "resolved" {
		t.Fatalf("expected resolved, got %q", updated.Status)
	}
	if updated.AgentID == nil || *updated.AgentID != 2 {
		t.Fatalf("expected agent_id=2, got %v", updated.AgentID)
	}
	if strings.Contains(updated.Tags, "a") {
		t.Fatalf("expected tag a removed, got %q", updated.Tags)
	}
	if !strings.Contains(updated.Tags, "b") || !strings.Contains(updated.Tags, "c") {
		t.Fatalf("expected tags include b and c, got %q", updated.Tags)
	}
	if updated.ResolvedAt == nil {
		t.Fatalf("expected resolved_at to be set")
	}

	// invalid request: agent_id + unassign
	_, err = svc.BulkUpdateTickets(ctx, &TicketBulkUpdateRequest{
		TicketIDs:     []uint{2},
		AgentID:       uintPtr(2),
		UnassignAgent: true,
	}, 99)
	if err == nil {
		t.Fatalf("expected error for conflicting agent mutation")
	}
}
