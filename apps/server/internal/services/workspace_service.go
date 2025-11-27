package services

import (
	"context"
	"fmt"
	"time"

	"servify/apps/server/internal/models"

	"gorm.io/gorm"
)

// WorkspaceService 汇总全渠道代理工作台数据
type WorkspaceService struct {
	db           *gorm.DB
	agentService *AgentService
}

func NewWorkspaceService(db *gorm.DB, agentService *AgentService) *WorkspaceService {
	return &WorkspaceService{
		db:           db,
		agentService: agentService,
	}
}

// ChannelSummary 渠道汇总
type ChannelSummary struct {
	Platform        string  `json:"platform"`
	ActiveSessions  int64   `json:"active_sessions"`
	WaitingSessions int64   `json:"waiting_sessions"`
	AvgResponseTime float64 `json:"avg_response_time"`
}

// WorkspaceOverview 全渠道视图
type WorkspaceOverview struct {
	TotalActiveSessions int64              `json:"total_active_sessions"`
	WaitingQueue        int64              `json:"waiting_queue"`
	OnlineAgents        int64              `json:"online_agents"`
	BusyAgents          int64              `json:"busy_agents"`
	Channels            []ChannelSummary   `json:"channels"`
	RecentSessions      []WorkspaceSession `json:"recent_sessions"`
}

// WorkspaceSession 最近会话摘要
type WorkspaceSession struct {
	ID           string    `json:"id"`
	Platform     string    `json:"platform"`
	Status       string    `json:"status"`
	AgentID      *uint     `json:"agent_id"`
	AgentName    string    `json:"agent_name"`
	CustomerID   *uint     `json:"customer_id"`
	CustomerName string    `json:"customer_name"`
	StartedAt    time.Time `json:"started_at"`
}

// GetOverview 汇总全渠道工作台所需的数据
func (s *WorkspaceService) GetOverview(ctx context.Context, limit int) (*WorkspaceOverview, error) {
	if limit <= 0 {
		limit = 10
	}

	overview := &WorkspaceOverview{}

	if err := s.db.Model(&models.Session{}).
		Where("status = ?", "active").
		Count(&overview.TotalActiveSessions).Error; err != nil {
		return nil, fmt.Errorf("count active sessions: %w", err)
	}

	if err := s.db.Model(&models.Session{}).
		Where("status = ? AND agent_id IS NULL", "active").
		Count(&overview.WaitingQueue).Error; err != nil {
		return nil, fmt.Errorf("count waiting sessions: %w", err)
	}

	var channelRows []struct {
		Platform string
		Active   int64
		Waiting  int64
	}
	if err := s.db.Model(&models.Session{}).
		Select("COALESCE(platform, 'unknown') AS platform, SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END) AS active, SUM(CASE WHEN status = 'active' AND agent_id IS NULL THEN 1 ELSE 0 END) AS waiting").
		Group("platform").
		Scan(&channelRows).Error; err != nil {
		return nil, fmt.Errorf("aggregate channels: %w", err)
	}

	for _, row := range channelRows {
		overview.Channels = append(overview.Channels, ChannelSummary{
			Platform:        row.Platform,
			ActiveSessions:  row.Active,
			WaitingSessions: row.Waiting,
			AvgResponseTime: s.getAvgResponseTime(),
		})
	}

	if s.agentService != nil {
		online := s.agentService.GetOnlineAgents(ctx)
		overview.OnlineAgents = int64(len(online))
		busy := 0
		for _, a := range online {
			if a.Status == "busy" || a.CurrentLoad >= a.MaxConcurrent {
				busy++
			}
		}
		overview.BusyAgents = int64(busy)
	}

	var sessions []WorkspaceSession
	if err := s.db.Table("sessions").
		Select(`sessions.id, COALESCE(sessions.platform, 'unknown') AS platform, sessions.status, sessions.agent_id, sessions.started_at,
				customers.user_id AS customer_id, cu.name AS customer_name, au.name AS agent_name`).
		Joins("LEFT JOIN tickets t ON t.id = sessions.ticket_id").
		Joins("LEFT JOIN customers ON customers.id = t.customer_id").
		Joins("LEFT JOIN users cu ON cu.id = customers.user_id").
		Joins("LEFT JOIN users au ON au.id = sessions.agent_id").
		Order("sessions.created_at DESC").
		Limit(limit).
		Scan(&sessions).Error; err != nil {
		return nil, fmt.Errorf("load recent sessions: %w", err)
	}
	overview.RecentSessions = sessions

	return overview, nil
}

func (s *WorkspaceService) getAvgResponseTime() float64 {
	var avg float64
	_ = s.db.Model(&models.Agent{}).Select("AVG(avg_response_time)").Row().Scan(&avg)
	return avg
}
