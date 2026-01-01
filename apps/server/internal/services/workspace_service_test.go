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

func newWorkspaceServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := t.Name()
	dsn := "file:workspace_" + name + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Session{},
		&models.Agent{},
		&models.User{},
		&models.Ticket{},
		&models.Customer{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestWorkspaceService_GetOverview_EmptyDB(t *testing.T) {
	db := newWorkspaceServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	agentSvc := NewAgentService(db, logger)
	svc := NewWorkspaceService(db, agentSvc)

	overview, err := svc.GetOverview(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if overview == nil {
		t.Fatal("expected overview, got nil")
	}
	if overview.TotalActiveSessions != 0 {
		t.Errorf("expected 0 active sessions, got %d", overview.TotalActiveSessions)
	}
	if overview.OnlineAgents != 0 {
		t.Errorf("expected 0 online agents, got %d", overview.OnlineAgents)
	}
}

func TestWorkspaceService_GetOverview_WithSessions(t *testing.T) {
	db := newWorkspaceServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	agentSvc := NewAgentService(db, logger)
	svc := NewWorkspaceService(db, agentSvc)

	// Create test users
	agentUser := &models.User{
		Username: "agent1",
		Email:    "agent1@test.com",
		Name:     "Agent One",
		Role:     "agent",
	}
	customerUser := &models.User{
		Username: "customer1",
		Email:    "customer1@test.com",
		Name:     "Customer One",
		Role:     "customer",
	}
	db.Create(agentUser)
	db.Create(customerUser)

	// Create agent
	agent := &models.Agent{
		UserID:  agentUser.ID,
		Status:  "online",
	}
	db.Create(agent)

	// Create active session
	session := &models.Session{
		Platform:  "web",
		Status:    "active",
		AgentID:   &agent.ID,
		StartedAt: time.Now(),
	}
	db.Create(session)

	// Make agent online first
	agentSvc.AgentGoOnline(context.Background(), agent.UserID)

	overview, err := svc.GetOverview(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if overview.TotalActiveSessions != 1 {
		t.Errorf("expected 1 active session, got %d", overview.TotalActiveSessions)
	}
	if overview.OnlineAgents != 1 {
		t.Errorf("expected 1 online agent, got %d", overview.OnlineAgents)
	}
}

func TestWorkspaceService_GetOverview_DefaultLimit(t *testing.T) {
	db := newWorkspaceServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	agentSvc := NewAgentService(db, logger)
	svc := NewWorkspaceService(db, agentSvc)

	// Test with limit <= 0 (should default to 10)
	overview, err := svc.GetOverview(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if overview == nil {
		t.Fatal("expected overview, got nil")
	}
}

func TestWorkspaceService_GetOverview_WaitingSessions(t *testing.T) {
	db := newWorkspaceServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	agentSvc := NewAgentService(db, logger)
	svc := NewWorkspaceService(db, agentSvc)

	// Create active session without agent (waiting)
	session := &models.Session{
		Platform:  "web",
		Status:    "active",
		AgentID:   nil,
		StartedAt: time.Now(),
	}
	db.Create(session)

	overview, err := svc.GetOverview(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if overview.WaitingQueue != 1 {
		t.Errorf("expected 1 waiting session, got %d", overview.WaitingQueue)
	}
}

func TestWorkspaceService_GetOverview_MultipleChannels(t *testing.T) {
	db := newWorkspaceServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	agentSvc := NewAgentService(db, logger)
	svc := NewWorkspaceService(db, agentSvc)

	// Create sessions for different platforms with unique IDs
	for _, platform := range []string{"web", "api", "mobile"} {
		session := &models.Session{
			ID:        "test-session-" + platform,
			Platform:  platform,
			Status:    "active",
			AgentID:   nil,
			StartedAt: time.Now(),
		}
		db.Create(session)
	}

	overview, err := svc.GetOverview(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if len(overview.Channels) != 3 {
		t.Errorf("expected 3 channels, got %d", len(overview.Channels))
	}
}

func TestWorkspaceService_GetOverview_BusyAgents(t *testing.T) {
	db := newWorkspaceServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	agentSvc := NewAgentService(db, logger)
	svc := NewWorkspaceService(db, agentSvc)

	// Create users and agents
	agent1User := &models.User{Username: "agent1", Email: "agent1@test.com", Name: "Agent 1", Role: "agent"}
	agent2User := &models.User{Username: "agent2", Email: "agent2@test.com", Name: "Agent 2", Role: "agent"}
	db.Create(agent1User)
	db.Create(agent2User)

	// Create agents - one busy, one online
	agent1 := &models.Agent{UserID: agent1User.ID, Status: "online", CurrentLoad: 5, MaxConcurrent: 5}
	agent2 := &models.Agent{UserID: agent2User.ID, Status: "online", CurrentLoad: 2, MaxConcurrent: 5}
	db.Create(agent1)
	db.Create(agent2)

	// Make them online
	agentSvc.AgentGoOnline(context.Background(), agent1.UserID)
	agentSvc.AgentGoOnline(context.Background(), agent2.UserID)

	overview, err := svc.GetOverview(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if overview.BusyAgents != 1 {
		t.Errorf("expected 1 busy agent, got %d", overview.BusyAgents)
	}
}

func TestWorkspaceService_GetOverview_NilAgentService(t *testing.T) {
	db := newWorkspaceServiceTestDB(t)
	svc := NewWorkspaceService(db, nil)

	// Create active session
	session := &models.Session{
		Platform:  "web",
		Status:    "active",
		AgentID:   nil,
		StartedAt: time.Now(),
	}
	db.Create(session)

	overview, err := svc.GetOverview(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if overview.OnlineAgents != 0 {
		t.Errorf("expected 0 online agents (nil service), got %d", overview.OnlineAgents)
	}
}

func TestNewWorkspaceService(t *testing.T) {
	db := newWorkspaceServiceTestDB(t)
	logger := logrus.New()
	agentSvc := NewAgentService(db, logger)

	svc := NewWorkspaceService(db, agentSvc)

	if svc == nil {
		t.Fatal("expected service, got nil")
	}
	if svc.db != db {
		t.Error("expected db to be set")
	}
	if svc.agentService != agentSvc {
		t.Error("expected agentService to be set")
	}
}

func TestChannelSummary_Structure(t *testing.T) {
	summary := ChannelSummary{
		Platform:         "web",
		ActiveSessions:   10,
		WaitingSessions:  2,
		AvgResponseTime:  150.5,
	}

	if summary.Platform != "web" {
		t.Errorf("expected platform 'web', got '%s'", summary.Platform)
	}
	if summary.ActiveSessions != 10 {
		t.Errorf("expected 10 active sessions, got %d", summary.ActiveSessions)
	}
}

func TestWorkspaceOverview_Structure(t *testing.T) {
	overview := &WorkspaceOverview{
		TotalActiveSessions: 5,
		WaitingQueue:       2,
		OnlineAgents:       3,
		BusyAgents:         1,
		Channels: []ChannelSummary{
			{Platform: "web", ActiveSessions: 3},
		},
		RecentSessions: []WorkspaceSession{
			{ID: "sess1", Platform: "web"},
		},
	}

	if overview.TotalActiveSessions != 5 {
		t.Errorf("expected 5 total active sessions, got %d", overview.TotalActiveSessions)
	}
	if len(overview.Channels) != 1 {
		t.Errorf("expected 1 channel, got %d", len(overview.Channels))
	}
}
