package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"servify/apps/server/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// SessionTransferService 会话转接服务
type SessionTransferService struct {
	db           *gorm.DB
	logger       *logrus.Logger
	aiService    AIServiceInterface
	agentService *AgentService
	wsHub        *WebSocketHub
}

// NewSessionTransferService 创建会话转接服务
func NewSessionTransferService(
	db *gorm.DB,
	logger *logrus.Logger,
	aiService AIServiceInterface,
	agentService *AgentService,
	wsHub *WebSocketHub,
) *SessionTransferService {
	if logger == nil {
		logger = logrus.New()
	}

	return &SessionTransferService{
		db:           db,
		logger:       logger,
		aiService:    aiService,
		agentService: agentService,
		wsHub:        wsHub,
	}
}

// TransferRequest 转接请求
type TransferRequest struct {
	SessionID    string   `json:"session_id" binding:"required"`
	Reason       string   `json:"reason"`
	TargetSkills []string `json:"target_skills"`
	Priority     string   `json:"priority"`
	Notes        string   `json:"notes"`
}

// TransferToHuman 转接到人工客服
func (s *SessionTransferService) TransferToHuman(ctx context.Context, req *TransferRequest) (*TransferResult, error) {
	// 获取会话信息
	var session models.Session
	if err := s.db.Preload("User").Preload("Messages").First(&session, "id = ?", req.SessionID).Error; err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// 检查会话状态
	if session.Status == "transferred" || session.AgentID != nil {
		return nil, fmt.Errorf("session already transferred or assigned")
	}

	// 查找可用的客服
	agent, err := s.agentService.FindAvailableAgent(ctx, req.TargetSkills, req.Priority)
	if err != nil {
		// 没有可用客服，加入等待队列
		return s.addToWaitingQueue(ctx, &session, req)
	}

	// 执行转接
	return s.executeTransfer(ctx, &session, agent.UserID, req.Reason, req.Notes)
}

// TransferToAgent 转接到指定客服
func (s *SessionTransferService) TransferToAgent(ctx context.Context, sessionID string, targetAgentID uint, reason string) (*TransferResult, error) {
	// 获取会话信息
	var session models.Session
	if err := s.db.Preload("User").First(&session, "id = ?", sessionID).Error; err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// 检查目标客服是否可用
	agentInfo, ok := s.agentService.onlineAgents.Load(targetAgentID)
	if !ok {
		return nil, fmt.Errorf("target agent is not online")
	}

	info := agentInfo.(*AgentInfo)
	if info.CurrentLoad >= info.MaxConcurrent {
		return nil, fmt.Errorf("target agent is at maximum capacity")
	}

	// 执行转接
	return s.executeTransfer(ctx, &session, targetAgentID, reason, "")
}

// executeTransfer 执行转接
func (s *SessionTransferService) executeTransfer(ctx context.Context, session *models.Session, targetAgentID uint, reason, notes string) (*TransferResult, error) {
	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 如果会话已有客服，先释放原客服
	if session.AgentID != nil {
		if err := s.agentService.ReleaseSessionFromAgent(ctx, session.ID, *session.AgentID); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to release from current agent: %w", err)
		}
	}

	// 分配给新客服
	if err := s.agentService.AssignSessionToAgent(ctx, session.ID, targetAgentID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to assign to target agent: %w", err)
	}

	// 更新会话状态
	updates := map[string]interface{}{
		"agent_id": targetAgentID,
		"status":   "transferred",
	}

	if err := tx.Model(&models.Session{}).Where("id = ?", session.ID).Updates(updates).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	// 创建系统消息
	transferMessage := &models.Message{
		SessionID: session.ID,
		UserID:    targetAgentID,
		Content:   s.buildTransferMessage(reason, notes),
		Type:      "system",
		Sender:    "system",
	}

	if err := tx.Create(transferMessage).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create transfer message: %w", err)
	}

	// 生成会话摘要
	summary, err := s.generateSessionSummary(session)
	if err != nil {
		s.logger.Warnf("Failed to generate session summary: %v", err)
		summary = "无法生成会话摘要"
	}

	// 创建转接记录
	transferRecord := &TransferRecord{
		SessionID:       session.ID,
		FromAgentID:     session.AgentID,
		ToAgentID:       &targetAgentID,
		Reason:          reason,
		Notes:           notes,
		SessionSummary:  summary,
		TransferredAt:   time.Now(),
	}

	if err := tx.Create(transferRecord).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create transfer record: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 发送实时通知
	s.notifyTransfer(session.ID, targetAgentID, transferMessage.Content)

	s.logger.Infof("Successfully transferred session %s to agent %d", session.ID, targetAgentID)

	return &TransferResult{
		Success:       true,
		SessionID:     session.ID,
		NewAgentID:    targetAgentID,
		TransferredAt: transferRecord.TransferredAt,
		Summary:       summary,
	}, nil
}

// addToWaitingQueue 添加到等待队列
func (s *SessionTransferService) addToWaitingQueue(ctx context.Context, session *models.Session, req *TransferRequest) (*TransferResult, error) {
	// 更新会话状态为等待中
	if err := s.db.Model(&models.Session{}).
		Where("id = ?", session.ID).
		Updates(map[string]interface{}{
			"status": "waiting_for_agent",
		}).Error; err != nil {
		return nil, fmt.Errorf("failed to update session status: %w", err)
	}

	// 创建等待记录
	waitingRecord := &WaitingRecord{
		SessionID:    session.ID,
		Reason:       req.Reason,
		TargetSkills: strings.Join(req.TargetSkills, ","),
		Priority:     req.Priority,
		Notes:        req.Notes,
		QueuedAt:     time.Now(),
	}

	if err := s.db.Create(waitingRecord).Error; err != nil {
		return nil, fmt.Errorf("failed to create waiting record: %w", err)
	}

	// 发送等待消息给用户
	waitingMessage := &models.Message{
		SessionID: session.ID,
		UserID:    session.UserID,
		Content:   "您的会话已加入人工客服等待队列，我们会尽快为您安排客服。请耐心等待。",
		Type:      "system",
		Sender:    "system",
	}

	s.db.Create(waitingMessage)

	// 发送实时通知
	s.notifyWaiting(session.ID, waitingMessage.Content)

	s.logger.Infof("Added session %s to waiting queue", session.ID)

	return &TransferResult{
		Success:     true,
		SessionID:   session.ID,
		IsWaiting:   true,
		QueuedAt:    &waitingRecord.QueuedAt,
		Summary:     "会话已加入等待队列",
	}, nil
}

// ProcessWaitingQueue 处理等待队列
func (s *SessionTransferService) ProcessWaitingQueue(ctx context.Context) error {
	// 获取等待中的会话
	var waitingRecords []WaitingRecord
	if err := s.db.Where("status = ?", "waiting").
		Order("priority DESC, queued_at ASC").
		Limit(10).
		Find(&waitingRecords).Error; err != nil {
		return fmt.Errorf("failed to get waiting records: %w", err)
	}

	for _, record := range waitingRecords {
		// 查找可用客服
		skills := []string{}
		if record.TargetSkills != "" {
			skills = strings.Split(record.TargetSkills, ",")
		}

		agent, err := s.agentService.FindAvailableAgent(ctx, skills, record.Priority)
		if err != nil {
			continue // 没有可用客服，继续下一个
		}

		// 获取会话信息
		var session models.Session
		if err := s.db.First(&session, "id = ?", record.SessionID).Error; err != nil {
			continue
		}

		// 执行转接
		result, err := s.executeTransfer(ctx, &session, agent.UserID, record.Reason, record.Notes)
		if err != nil {
			s.logger.Errorf("Failed to transfer waiting session %s: %v", record.SessionID, err)
			continue
		}

		// 更新等待记录状态
		s.db.Model(&WaitingRecord{}).
			Where("id = ?", record.ID).
			Updates(map[string]interface{}{
				"status":       "transferred",
				"assigned_at":  time.Now(),
				"assigned_to":  agent.UserID,
			})

		s.logger.Infof("Successfully transferred waiting session %s to agent %d",
			result.SessionID, result.NewAgentID)
	}

	return nil
}

// generateSessionSummary 生成会话摘要
func (s *SessionTransferService) generateSessionSummary(session *models.Session) (string, error) {
	// 获取会话消息
	var messages []models.Message
	if err := s.db.Where("session_id = ?", session.ID).
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		return "", err
	}

	// 如果消息太少，返回简单摘要
	if len(messages) < 3 {
		return fmt.Sprintf("用户%s的简短会话，共%d条消息", session.User.Username, len(messages)), nil
	}

	// 使用 AI 服务生成摘要
	return s.aiService.GetSessionSummary(messages)
}

// buildTransferMessage 构建转接消息
func (s *SessionTransferService) buildTransferMessage(reason, notes string) string {
	message := "您的会话已转接至人工客服"

	if reason != "" {
		message += fmt.Sprintf("。转接原因：%s", reason)
	}

	if notes != "" {
		message += fmt.Sprintf("。备注：%s", notes)
	}

	message += "。客服将很快为您提供帮助。"

	return message
}

// notifyTransfer 发送转接通知
func (s *SessionTransferService) notifyTransfer(sessionID string, agentID uint, message string) {
	// 发送给用户
	if s.wsHub != nil {
		s.wsHub.SendToSession(sessionID, WebSocketMessage{
			Type: "transfer_notification",
			Data: map[string]interface{}{
				"message":   message,
				"agent_id":  agentID,
				"timestamp": time.Now(),
			},
		})
	}
}

// notifyWaiting 发送等待通知
func (s *SessionTransferService) notifyWaiting(sessionID string, message string) {
	if s.wsHub != nil {
		s.wsHub.SendToSession(sessionID, WebSocketMessage{
			Type: "waiting_notification",
			Data: map[string]interface{}{
				"message":   message,
				"timestamp": time.Now(),
			},
		})
	}
}

// GetTransferHistory 获取转接历史
func (s *SessionTransferService) GetTransferHistory(ctx context.Context, sessionID string) ([]TransferRecord, error) {
	var records []TransferRecord
	err := s.db.Where("session_id = ?", sessionID).
		Order("transferred_at DESC").
		Find(&records).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get transfer history: %w", err)
	}

	return records, nil
}

// AutoTransferCheck 自动转接检查
func (s *SessionTransferService) AutoTransferCheck(ctx context.Context, sessionID string, messages []models.Message) bool {
	// 检查是否需要自动转接
	lastMessages := messages
	if len(messages) > 5 {
		lastMessages = messages[len(messages)-5:]
	}

	// 构建查询字符串
	var queryBuilder strings.Builder
	for _, msg := range lastMessages {
		if msg.Sender == "user" {
			queryBuilder.WriteString(msg.Content)
			queryBuilder.WriteString(" ")
		}
	}

	query := queryBuilder.String()

	// 使用 AI 服务判断是否需要转人工
	return s.aiService.ShouldTransferToHuman(query, messages)
}

// 数据模型
type TransferRecord struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	SessionID      string     `gorm:"index" json:"session_id"`
	FromAgentID    *uint      `json:"from_agent_id"`
	ToAgentID      *uint      `json:"to_agent_id"`
	Reason         string     `json:"reason"`
	Notes          string     `json:"notes"`
	SessionSummary string     `gorm:"type:text" json:"session_summary"`
	TransferredAt  time.Time  `json:"transferred_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type WaitingRecord struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	SessionID    string     `gorm:"index" json:"session_id"`
	Reason       string     `json:"reason"`
	TargetSkills string     `json:"target_skills"`
	Priority     string     `json:"priority"`
	Notes        string     `json:"notes"`
	Status       string     `gorm:"default:'waiting'" json:"status"` // waiting, transferred, cancelled
	QueuedAt     time.Time  `json:"queued_at"`
	AssignedAt   *time.Time `json:"assigned_at"`
	AssignedTo   *uint      `json:"assigned_to"`
	CreatedAt    time.Time  `json:"created_at"`
}

type TransferResult struct {
	Success       bool       `json:"success"`
	SessionID     string     `json:"session_id"`
	NewAgentID    uint       `json:"new_agent_id,omitempty"`
	IsWaiting     bool       `json:"is_waiting,omitempty"`
	QueuedAt      *time.Time `json:"queued_at,omitempty"`
	TransferredAt time.Time  `json:"transferred_at,omitempty"`
	Summary       string     `json:"summary"`
}
