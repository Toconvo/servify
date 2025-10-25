package services

import (
	"context"
	"fmt"
	"time"

	"servify/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// TicketService 工单管理服务
type TicketService struct {
	db     *gorm.DB
	logger *logrus.Logger
}

// NewTicketService 创建工单服务
func NewTicketService(db *gorm.DB, logger *logrus.Logger) *TicketService {
	if logger == nil {
		logger = logrus.New()
	}

	return &TicketService{
		db:     db,
		logger: logger,
	}
}

// TicketCreateRequest 创建工单请求
type TicketCreateRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	CustomerID  uint   `json:"customer_id" binding:"required"`
	Category    string `json:"category"`
	Priority    string `json:"priority"`
	Source      string `json:"source"`
	Tags        string `json:"tags"`
	SessionID   string `json:"session_id"`
}

// TicketUpdateRequest 更新工单请求
type TicketUpdateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	AgentID     *uint   `json:"agent_id"`
	Category    *string `json:"category"`
	Priority    *string `json:"priority"`
	Status      *string `json:"status"`
	Tags        *string `json:"tags"`
	DueDate     *time.Time `json:"due_date"`
}

// TicketListRequest 工单列表请求
type TicketListRequest struct {
	Page       int      `form:"page,default=1"`
	PageSize   int      `form:"page_size,default=20"`
	Status     []string `form:"status"`
	Priority   []string `form:"priority"`
	Category   []string `form:"category"`
	AgentID    *uint    `form:"agent_id"`
	CustomerID *uint    `form:"customer_id"`
	Search     string   `form:"search"`
	SortBy     string   `form:"sort_by,default=created_at"`
	SortOrder  string   `form:"sort_order,default=desc"`
}

// CreateTicket 创建工单
func (s *TicketService) CreateTicket(ctx context.Context, req *TicketCreateRequest) (*models.Ticket, error) {
	// 验证客户是否存在
	var customer models.User
	if err := s.db.First(&customer, req.CustomerID).Error; err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	// 设置默认值
	if req.Category == "" {
		req.Category = "general"
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}
	if req.Source == "" {
		req.Source = "web"
	}

	// 创建工单
	ticket := &models.Ticket{
		Title:       req.Title,
		Description: req.Description,
		CustomerID:  req.CustomerID,
		Category:    req.Category,
		Priority:    req.Priority,
		Status:      "open",
		Source:      req.Source,
		Tags:        req.Tags,
	}

	// 如果提供了 SessionID，关联会话
	if req.SessionID != "" {
		ticket.SessionID = &req.SessionID
	}

	// 保存工单
	if err := s.db.Create(ticket).Error; err != nil {
		return nil, fmt.Errorf("failed to create ticket: %w", err)
	}

	// 记录状态变更历史
	s.recordStatusChange(ticket.ID, 0, "", "open", "工单创建")

	// 自动分配客服（如果有可用的客服）
	go s.autoAssignAgent(ticket.ID)

	s.logger.Infof("Created ticket %d for customer %d", ticket.ID, req.CustomerID)

	// 返回完整的工单信息
	return s.GetTicketByID(ctx, ticket.ID)
}

// GetTicketByID 根据ID获取工单
func (s *TicketService) GetTicketByID(ctx context.Context, ticketID uint) (*models.Ticket, error) {
	var ticket models.Ticket
	err := s.db.Preload("Customer").
		Preload("Agent").
		Preload("Session").
		Preload("Comments", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC").Preload("User")
		}).
		Preload("Attachments").
		Preload("StatusHistory", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC").Preload("User")
		}).
		First(&ticket, ticketID).Error

	if err != nil {
		return nil, fmt.Errorf("ticket not found: %w", err)
	}

	return &ticket, nil
}

// UpdateTicket 更新工单
func (s *TicketService) UpdateTicket(ctx context.Context, ticketID uint, req *TicketUpdateRequest, userID uint) (*models.Ticket, error) {
	// 获取原工单
	oldTicket, err := s.GetTicketByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	// 构建更新数据
	updates := make(map[string]interface{})

	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.AgentID != nil {
		updates["agent_id"] = *req.AgentID
	}
	if req.Category != nil {
		updates["category"] = *req.Category
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.Tags != nil {
		updates["tags"] = *req.Tags
	}
	if req.DueDate != nil {
		updates["due_date"] = *req.DueDate
	}

	// 处理状态变更
	if req.Status != nil && *req.Status != oldTicket.Status {
		updates["status"] = *req.Status

		// 设置特殊状态的时间戳
		switch *req.Status {
		case "resolved":
			now := time.Now()
			updates["resolved_at"] = &now
		case "closed":
			now := time.Now()
			updates["closed_at"] = &now
		}

		// 记录状态变更历史
		s.recordStatusChange(ticketID, userID, oldTicket.Status, *req.Status, "状态更新")
	}

	// 更新工单
	if err := s.db.Model(&models.Ticket{}).Where("id = ?", ticketID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update ticket: %w", err)
	}

	s.logger.Infof("Updated ticket %d by user %d", ticketID, userID)

	// 返回更新后的工单
	return s.GetTicketByID(ctx, ticketID)
}

// ListTickets 获取工单列表
func (s *TicketService) ListTickets(ctx context.Context, req *TicketListRequest) ([]models.Ticket, int64, error) {
	query := s.db.Model(&models.Ticket{}).
		Preload("Customer").
		Preload("Agent")

	// 应用过滤条件
	if len(req.Status) > 0 {
		query = query.Where("status IN ?", req.Status)
	}
	if len(req.Priority) > 0 {
		query = query.Where("priority IN ?", req.Priority)
	}
	if len(req.Category) > 0 {
		query = query.Where("category IN ?", req.Category)
	}
	if req.AgentID != nil {
		query = query.Where("agent_id = ?", *req.AgentID)
	}
	if req.CustomerID != nil {
		query = query.Where("customer_id = ?", *req.CustomerID)
	}

	// 搜索条件
	if req.Search != "" {
		searchTerm := "%" + req.Search + "%"
		query = query.Where("title ILIKE ? OR description ILIKE ? OR tags ILIKE ?",
			searchTerm, searchTerm, searchTerm)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count tickets: %w", err)
	}

	// 排序
	orderBy := fmt.Sprintf("%s %s", req.SortBy, req.SortOrder)
	query = query.Order(orderBy)

	// 分页
	offset := (req.Page - 1) * req.PageSize
	query = query.Offset(offset).Limit(req.PageSize)

	// 获取数据
	var tickets []models.Ticket
	if err := query.Find(&tickets).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list tickets: %w", err)
	}

	return tickets, total, nil
}

// AssignTicket 分配工单给客服
func (s *TicketService) AssignTicket(ctx context.Context, ticketID uint, agentID uint, assignerID uint) error {
	// 验证客服是否存在且可用
	var agent models.Agent
	if err := s.db.Where("user_id = ? AND status IN ?", agentID, []string{"online", "busy"}).First(&agent).Error; err != nil {
		return fmt.Errorf("agent not available: %w", err)
	}

	// 检查客服是否超载
	if agent.CurrentLoad >= agent.MaxConcurrent {
		return fmt.Errorf("agent is at maximum capacity")
	}

	// 更新工单
	updates := map[string]interface{}{
		"agent_id": agentID,
		"status":   "assigned",
	}

	if err := s.db.Model(&models.Ticket{}).Where("id = ?", ticketID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to assign ticket: %w", err)
	}

	// 更新客服负载
	s.db.Model(&models.Agent{}).Where("user_id = ?", agentID).UpdateColumn("current_load", gorm.Expr("current_load + 1"))

	// 记录状态变更
	s.recordStatusChange(ticketID, assignerID, "open", "assigned", fmt.Sprintf("分配给客服 %d", agentID))

	s.logger.Infof("Assigned ticket %d to agent %d", ticketID, agentID)

	return nil
}

// AddComment 添加工单评论
func (s *TicketService) AddComment(ctx context.Context, ticketID uint, userID uint, content string, commentType string) (*models.TicketComment, error) {
	if commentType == "" {
		commentType = "comment"
	}

	comment := &models.TicketComment{
		TicketID:  ticketID,
		UserID:    userID,
		Content:   content,
		Type:      commentType,
	}

	if err := s.db.Create(comment).Error; err != nil {
		return nil, fmt.Errorf("failed to add comment: %w", err)
	}

	// 预加载用户信息
	s.db.Preload("User").First(comment, comment.ID)

	s.logger.Infof("Added comment to ticket %d by user %d", ticketID, userID)

	return comment, nil
}

// CloseTicket 关闭工单
func (s *TicketService) CloseTicket(ctx context.Context, ticketID uint, userID uint, reason string) error {
	// 获取工单信息
	ticket, err := s.GetTicketByID(ctx, ticketID)
	if err != nil {
		return err
	}

	// 更新工单状态
	now := time.Now()
	updates := map[string]interface{}{
		"status":    "closed",
		"closed_at": &now,
	}

	if err := s.db.Model(&models.Ticket{}).Where("id = ?", ticketID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to close ticket: %w", err)
	}

	// 如果有分配的客服，减少其负载
	if ticket.AgentID != nil {
		s.db.Model(&models.Agent{}).
			Where("user_id = ? AND current_load > 0", *ticket.AgentID).
			UpdateColumn("current_load", gorm.Expr("current_load - 1"))
	}

	// 记录状态变更
	s.recordStatusChange(ticketID, userID, ticket.Status, "closed", reason)

	// 添加系统评论
	s.AddComment(ctx, ticketID, userID, fmt.Sprintf("工单已关闭。原因：%s", reason), "system")

	s.logger.Infof("Closed ticket %d by user %d", ticketID, userID)

	return nil
}

// autoAssignAgent 自动分配客服
func (s *TicketService) autoAssignAgent(ticketID uint) {
	// 查找可用的客服（在线且负载最低）
	var agent models.Agent
	err := s.db.Where("status = ? AND current_load < max_concurrent", "online").
		Order("current_load ASC, avg_response_time ASC").
		First(&agent).Error

	if err != nil {
		s.logger.Debugf("No available agent for auto-assignment of ticket %d", ticketID)
		return
	}

	// 尝试分配
	if err := s.AssignTicket(context.Background(), ticketID, agent.UserID, 0); err != nil {
		s.logger.Errorf("Failed to auto-assign ticket %d to agent %d: %v", ticketID, agent.UserID, err)
	} else {
		s.logger.Infof("Auto-assigned ticket %d to agent %d", ticketID, agent.UserID)
	}
}

// recordStatusChange 记录状态变更历史
func (s *TicketService) recordStatusChange(ticketID uint, userID uint, fromStatus, toStatus, reason string) {
	statusChange := &models.TicketStatus{
		TicketID:   ticketID,
		UserID:     userID,
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		Reason:     reason,
	}

	if err := s.db.Create(statusChange).Error; err != nil {
		s.logger.Errorf("Failed to record status change for ticket %d: %v", ticketID, err)
	}
}

// GetTicketStats 获取工单统计
func (s *TicketService) GetTicketStats(ctx context.Context, agentID *uint) (*TicketStats, error) {
	stats := &TicketStats{}

	query := s.db.Model(&models.Ticket{})
	if agentID != nil {
		query = query.Where("agent_id = ?", *agentID)
	}

	// 总工单数
	query.Count(&stats.Total)

	// 按状态统计
	s.db.Model(&models.Ticket{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&stats.ByStatus)

	// 按优先级统计
	s.db.Model(&models.Ticket{}).
		Select("priority, COUNT(*) as count").
		Group("priority").
		Scan(&stats.ByPriority)

	// 今日新建工单
	today := time.Now().Truncate(24 * time.Hour)
	s.db.Model(&models.Ticket{}).
		Where("created_at >= ?", today).
		Count(&stats.TodayCreated)

	// 待处理工单
	s.db.Model(&models.Ticket{}).
		Where("status IN ?", []string{"open", "assigned"}).
		Count(&stats.Pending)

	// 已解决工单
	s.db.Model(&models.Ticket{}).
		Where("status = ?", "resolved").
		Count(&stats.Resolved)

	return stats, nil
}

// TicketStats 工单统计信息
type TicketStats struct {
	Total        int64                    `json:"total"`
	TodayCreated int64                    `json:"today_created"`
	Pending      int64                    `json:"pending"`
	Resolved     int64                    `json:"resolved"`
	ByStatus     []StatusCount            `json:"by_status"`
	ByPriority   []PriorityCount          `json:"by_priority"`
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type PriorityCount struct {
	Priority string `json:"priority"`
	Count    int64  `json:"count"`
}