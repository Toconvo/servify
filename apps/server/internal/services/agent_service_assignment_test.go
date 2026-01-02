package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"servify/apps/server/internal/models"
)

func TestAgentService_AssignSessionToAgent_Success(t *testing.T) {
	db := newAgentServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewAgentService(db, logger)

	// Create user and agent
	user := &models.User{
		Username: "agent1",
		Email:    "agent1@test.com",
		Name:     "Agent One",
		Role:     "agent",
	}
	db.Create(user)

	agent := &models.Agent{
		UserID:       user.ID,
		Status:       "online",
		CurrentLoad:  0,
		MaxConcurrent: 5,
	}
	db.Create(agent)

	// Make agent online
	svc.AgentGoOnline(context.Background(), agent.UserID)

	// Create session
	session := &models.Session{
		Platform:  "web",
		Status:    "active",
		StartedAt: time.Now(),
	}
	db.Create(session)

	// Assign session to agent
	err := svc.AssignSessionToAgent(context.Background(), session.ID, agent.ID)
	if err != nil {
		t.Fatalf("AssignSessionToAgent() error = %v", err)
	}

	// Verify agent load increased
	agentInfo, ok := svc.onlineAgents.Load(agent.ID)
	if !ok {
		t.Fatal("agent not found in online agents")
	}
	info := agentInfo.(*AgentInfo)
	if info.CurrentLoad != 1 {
		t.Errorf("expected CurrentLoad 1, got %d", info.CurrentLoad)
	}
}

func TestAgentService_AssignSessionToAgent_AgentNotOnline(t *testing.T) {
	db := newAgentServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewAgentService(db, logger)

	// Create user and agent but don't make them online
	user := &models.User{
		Username: "agent1",
		Email:    "agent1@test.com",
		Name:     "Agent One",
		Role:     "agent",
	}
	db.Create(user)

	agent := &models.Agent{
		UserID: user.ID,
		Status: "online",
	}
	db.Create(agent)

	session := &models.Session{
		Platform:  "web",
		Status:    "active",
		StartedAt: time.Now(),
	}
	db.Create(session)

	err := svc.AssignSessionToAgent(context.Background(), session.ID, agent.ID)
	if err == nil {
		t.Error("expected error for offline agent, got nil")
	}
}

func TestAgentService_AssignSessionToAgent_AgentAtCapacity(t *testing.T) {
	db := newAgentServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewAgentService(db, logger)

	// Create user and agent
	user := &models.User{
		Username: "agent1",
		Email:    "agent1@test.com",
		Name:     "Agent One",
		Role:     "agent",
	}
	db.Create(user)

	agent := &models.Agent{
		UserID:       user.ID,
		Status:       "online",
		CurrentLoad:  5,
		MaxConcurrent: 5,
	}
	db.Create(agent)

	svc.AgentGoOnline(context.Background(), agent.UserID)

	session := &models.Session{
		Platform:  "web",
		Status:    "active",
		StartedAt: time.Now(),
	}
	db.Create(session)

	err := svc.AssignSessionToAgent(context.Background(), session.ID, agent.ID)
	if err == nil {
		t.Error("expected error for agent at capacity, got nil")
	}
}

func TestAgentService_ReleaseSessionFromAgent_Success(t *testing.T) {
	db := newAgentServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewAgentService(db, logger)

	// Create user and agent
	user := &models.User{
		Username: "agent1",
		Email:    "agent1@test.com",
		Name:     "Agent One",
		Role:     "agent",
	}
	db.Create(user)

	agent := &models.Agent{
		UserID:       user.ID,
		Status:       "online",
		CurrentLoad:  1,
		MaxConcurrent: 5,
	}
	db.Create(agent)

	svc.AgentGoOnline(context.Background(), agent.UserID)

	// Create session
	session := &models.Session{
		Platform:  "web",
		Status:    "active",
		AgentID:   &agent.ID,
		StartedAt: time.Now(),
	}
	db.Create(session)

	// Release session
	err := svc.ReleaseSessionFromAgent(context.Background(), session.ID, agent.ID)
	if err != nil {
		t.Fatalf("ReleaseSessionFromAgent() error = %v", err)
	}

	// Verify agent load decreased
	agentInfo, ok := svc.onlineAgents.Load(agent.ID)
	if !ok {
		t.Fatal("agent not found in online agents")
	}
	info := agentInfo.(*AgentInfo)
	if info.CurrentLoad != 0 {
		t.Errorf("expected CurrentLoad 0, got %d", info.CurrentLoad)
	}
}

func TestAgentService_ReleaseSessionFromAgent_AgentNotOnline(t *testing.T) {
	db := newAgentServiceTestDB(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	svc := NewAgentService(db, logger)

	user := &models.User{
		Username: "agent1",
		Email:    "agent1@test.com",
		Name:     "Agent One",
		Role:     "agent",
	}
	db.Create(user)

	agent := &models.Agent{
		UserID: user.ID,
		Status: "online",
	}
	db.Create(agent)

	session := &models.Session{
		Platform:  "web",
		Status:    "active",
		AgentID:   &agent.ID,
		StartedAt: time.Now(),
	}
	db.Create(session)

	// ReleaseSessionFromAgent updates database even if agent is not in onlineAgents map
	err := svc.ReleaseSessionFromAgent(context.Background(), session.ID, agent.ID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify session was ended
	var updatedSession models.Session
	db.First(&updatedSession, session.ID)
	if updatedSession.Status != "ended" {
		t.Errorf("expected session status 'ended', got '%s'", updatedSession.Status)
	}
}
