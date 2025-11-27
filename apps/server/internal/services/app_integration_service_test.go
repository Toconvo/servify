package services

import (
	"context"
	"testing"

	"servify/apps/server/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newIntegrationTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AppIntegration{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestAppIntegrationService_CreateAndList(t *testing.T) {
	db := newIntegrationTestDB(t)
	svc := NewAppIntegrationService(db, logrus.New())

	item, err := svc.Create(context.Background(), &AppIntegrationCreateRequest{
		Name:      "Stripe Billing",
		Slug:      "stripe-billing",
		Vendor:    "Stripe",
		IFrameURL: "https://example.com/stripe",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if item.Slug != "stripe-billing" {
		t.Fatalf("unexpected slug: %s", item.Slug)
	}

	list, total, err := svc.List(context.Background(), &AppIntegrationListRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("unexpected list result: total=%d len=%d", total, len(list))
	}
}
