package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"servify/apps/server/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// TicketService 工单管理服务
type TicketService struct {
	db           *gorm.DB
	logger       *logrus.Logger
	slaService   *SLAService
	automation   *AutomationService
	satisfaction *SatisfactionService
}

// NewTicketService 创建工单服务
func NewTicketService(db *gorm.DB, logger *logrus.Logger, slaService *SLAService) *TicketService {
	if logger == nil {
		logger = logrus.New()
	}

	return &TicketService{
		db:         db,
		logger:     logger,
		slaService: slaService,
	}
}

// SetAutomationService 注入自动化服务
func (s *TicketService) SetAutomationService(automation *AutomationService) {
	s.automation = automation
}

// SetSatisfactionService 注入满意度服务
func (s *TicketService) SetSatisfactionService(satisfaction *SatisfactionService) {
	s.satisfaction = satisfaction
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
	Title       *string    `json:"title"`
	Description *string    `json:"description"`
	AgentID     *uint      `json:"agent_id"`
	Category    *string    `json:"category"`
	Priority    *string    `json:"priority"`
	Status      *string    `json:"status"`
	Tags        *string    `json:"tags"`
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

// TicketBulkUpdateRequest 批量更新工单请求（状态/标签/指派）
type TicketBulkUpdateRequest struct {
	TicketIDs     []uint   `json:"ticket_ids" binding:"required,min=1"`
	Status        *string  `json:"status"`
	SetTags       *string  `json:"set_tags"` // 覆盖式设置（逗号分隔）
	AddTags       []string `json:"add_tags"`
	RemoveTags    []string `json:"remove_tags"`
	AgentID       *uint    `json:"agent_id"` // 指派/转移到某个客服（agent.user_id）
	UnassignAgent bool     `json:"unassign_agent"`
}

type TicketBulkUpdateFailure struct {
	TicketID uint   `json:"ticket_id"`
	Error    string `json:"error"`
}

type TicketBulkUpdateResult struct {
	Updated []uint                   `json:"updated"`
	Failed  []TicketBulkUpdateFailure `json:"failed"`
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

	createdTicket, err := s.GetTicketByID(ctx, ticket.ID)
	if err != nil {
		return nil, err
	}

	// 初次检查 SLA（确保计时任务开始跟踪）
	s.evaluateTicketSLA(ctx, createdTicket, false, false)

	return createdTicket, nil
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

	statusChanged := false
	// 处理状态变更
	if req.Status != nil && *req.Status != oldTicket.Status {
		updates["status"] = *req.Status
		statusChanged = true

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
	agentChanged := false
	if req.AgentID != nil {
		if (oldTicket.AgentID == nil && *req.AgentID != 0) || (oldTicket.AgentID != nil && *oldTicket.AgentID != *req.AgentID) {
			agentChanged = true
		}
	}

	// 更新工单
	if err := s.db.Model(&models.Ticket{}).Where("id = ?", ticketID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update ticket: %w", err)
	}

	s.logger.Infof("Updated ticket %d by user %d", ticketID, userID)

	updatedTicket, err := s.GetTicketByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	// 根据状态/指派变更触发 SLA 处理
	s.evaluateTicketSLA(ctx, updatedTicket, statusChanged, agentChanged)

	return updatedTicket, nil
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
	// Load current ticket state (for transfer/unassign semantics)
	var ticket models.Ticket
	if err := s.db.Select("id", "status", "agent_id").First(&ticket, ticketID).Error; err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	// No-op if already assigned to the same agent
	if ticket.AgentID != nil && *ticket.AgentID == agentID {
		return nil
	}

	// 验证客服是否存在且可用
	var agent models.Agent
	if err := s.db.Where("user_id = ? AND status IN ?", agentID, []string{"online", "busy"}).First(&agent).Error; err != nil {
		return fmt.Errorf("agent not available: %w", err)
	}

	// 检查客服是否超载
	if agent.CurrentLoad >= agent.MaxConcurrent {
		return fmt.Errorf("agent is at maximum capacity")
	}

	fromStatus := ticket.Status
	toStatus := ticket.Status
	if fromStatus == "open" || fromStatus == "" {
		toStatus = "assigned"
	}

	// Transfer implies decrementing previous agent load
	var prevAgentID *uint = ticket.AgentID
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if prevAgentID != nil {
			// Best-effort decrement; never below 0
			if err := tx.Exec(`UPDATE agents SET current_load = CASE WHEN current_load > 0 THEN current_load - 1 ELSE 0 END WHERE user_id = ?`, *prevAgentID).Error; err != nil {
				return fmt.Errorf("failed to decrement previous agent load: %w", err)
			}
		}

		updates := map[string]interface{}{
			"agent_id": agentID,
		}
		if toStatus != fromStatus {
			updates["status"] = toStatus
		}
		if err := tx.Model(&models.Ticket{}).Where("id = ?", ticketID).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to assign ticket: %w", err)
		}

		if err := tx.Model(&models.Agent{}).Where("user_id = ?", agentID).UpdateColumn("current_load", gorm.Expr("current_load + 1")).Error; err != nil {
			return fmt.Errorf("failed to increment agent load: %w", err)
		}

		// 记录状态变更（若仅转移且状态不变，用同状态记录原因）
		reason := fmt.Sprintf("指派给客服 %d", agentID)
		if prevAgentID != nil {
			reason = fmt.Sprintf("工单转移至客服 %d", agentID)
		}
		s.recordStatusChangeWithDB(tx, ticketID, assignerID, fromStatus, toStatus, reason)
		return nil
	})
	if err != nil {
		return err
	}

	s.logger.Infof("Assigned ticket %d to agent %d", ticketID, agentID)

	// 分配/转移后做 SLA 处理
	updatedTicket, err := s.GetTicketByID(ctx, ticketID)
	if err == nil {
		if prevAgentID == nil {
			s.resolveTicketSLAViolations(ctx, updatedTicket.ID, []string{"first_response"})
		}
		s.evaluateTicketSLA(ctx, updatedTicket, fromStatus != toStatus, true)
	} else {
		s.logger.Warnf("Failed to fetch ticket %d after assignment for SLA evaluation: %v", ticketID, err)
	}

	return nil
}

// UnassignTicket 取消工单指派（将 agent_id 置空）
func (s *TicketService) UnassignTicket(ctx context.Context, ticketID uint, operatorID uint, reason string) error {
	var ticket models.Ticket
	if err := s.db.Select("id", "status", "agent_id").First(&ticket, ticketID).Error; err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}
	if ticket.AgentID == nil {
		return nil
	}
	fromStatus := ticket.Status
	toStatus := ticket.Status
	if fromStatus == "assigned" || fromStatus == "in_progress" || fromStatus == "" {
		toStatus = "open"
	}
	if reason == "" {
		reason = "取消指派"
	}

	prevAgent := *ticket.AgentID
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`UPDATE agents SET current_load = CASE WHEN current_load > 0 THEN current_load - 1 ELSE 0 END WHERE user_id = ?`, prevAgent).Error; err != nil {
			return fmt.Errorf("failed to decrement agent load: %w", err)
		}
		updates := map[string]interface{}{
			"agent_id": nil,
		}
		if toStatus != fromStatus {
			updates["status"] = toStatus
		}
		if err := tx.Model(&models.Ticket{}).Where("id = ?", ticketID).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to unassign ticket: %w", err)
		}
		s.recordStatusChangeWithDB(tx, ticketID, operatorID, fromStatus, toStatus, reason)
		return nil
	})
	if err != nil {
		return err
	}

	updatedTicket, err := s.GetTicketByID(ctx, ticketID)
	if err == nil {
		s.evaluateTicketSLA(ctx, updatedTicket, fromStatus != toStatus, true)
	} else {
		s.logger.Warnf("Failed to fetch ticket %d after unassignment for SLA evaluation: %v", ticketID, err)
	}
	return nil
}

// BulkUpdateTickets 批量更新工单（支持：状态、标签、指派/取消指派）
func (s *TicketService) BulkUpdateTickets(ctx context.Context, req *TicketBulkUpdateRequest, userID uint) (*TicketBulkUpdateResult, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	if len(req.TicketIDs) == 0 {
		return nil, fmt.Errorf("ticket_ids is required")
	}
	if req.UnassignAgent && req.AgentID != nil {
		return nil, fmt.Errorf("cannot set both unassign_agent and agent_id")
	}

	ids := make([]uint, 0, len(req.TicketIDs))
	seen := make(map[uint]struct{}, len(req.TicketIDs))
	for _, id := range req.TicketIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no valid ticket ids")
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	out := &TicketBulkUpdateResult{}
	for _, ticketID := range ids {
		// 1) agent assignment changes
		if req.UnassignAgent {
			if err := s.UnassignTicket(ctx, ticketID, userID, "批量取消指派"); err != nil {
				out.Failed = append(out.Failed, TicketBulkUpdateFailure{TicketID: ticketID, Error: err.Error()})
				continue
			}
		} else if req.AgentID != nil {
			if err := s.AssignTicket(ctx, ticketID, *req.AgentID, userID); err != nil {
				out.Failed = append(out.Failed, TicketBulkUpdateFailure{TicketID: ticketID, Error: err.Error()})
				continue
			}
		}

		// 2) tags/status changes (via UpdateTicket to keep side-effects consistent)
		needUpdate := req.Status != nil || req.SetTags != nil || len(req.AddTags) > 0 || len(req.RemoveTags) > 0
		if !needUpdate {
			out.Updated = append(out.Updated, ticketID)
			continue
		}

		updateReq := &TicketUpdateRequest{
			Status: req.Status,
		}

		if req.SetTags != nil {
			tags := normalizeTags(splitTags(*req.SetTags))
			joined := strings.Join(tags, ",")
			updateReq.Tags = &joined
		} else if len(req.AddTags) > 0 || len(req.RemoveTags) > 0 {
			var cur models.Ticket
			if err := s.db.Select("id", "tags").First(&cur, ticketID).Error; err != nil {
				out.Failed = append(out.Failed, TicketBulkUpdateFailure{TicketID: ticketID, Error: fmt.Sprintf("ticket not found: %v", err)})
				continue
			}
			newTags := applyTagDelta(cur.Tags, req.AddTags, req.RemoveTags)
			joined := strings.Join(newTags, ",")
			updateReq.Tags = &joined
		}

		if _, err := s.UpdateTicket(ctx, ticketID, updateReq, userID); err != nil {
			out.Failed = append(out.Failed, TicketBulkUpdateFailure{TicketID: ticketID, Error: err.Error()})
			continue
		}

		out.Updated = append(out.Updated, ticketID)
	}

	return out, nil
}

// AddComment 添加工单评论
func (s *TicketService) AddComment(ctx context.Context, ticketID uint, userID uint, content string, commentType string) (*models.TicketComment, error) {
	if commentType == "" {
		commentType = "comment"
	}

	comment := &models.TicketComment{
		TicketID: ticketID,
		UserID:   userID,
		Content:  content,
		Type:     commentType,
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

	// 解决类型 SLA 违约
	s.resolveTicketSLAViolations(ctx, ticketID, []string{"resolution"})

	s.logger.Infof("Closed ticket %d by user %d", ticketID, userID)

	// 触发 CSAT 调查
	if s.satisfaction != nil {
		if _, err := s.satisfaction.ScheduleSurvey(ctx, ticket); err != nil {
			s.logger.Warnf("Failed to schedule CSAT survey for ticket %d: %v", ticketID, err)
		}
	}

	return nil
}

func splitTags(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

func applyTagDelta(current string, add []string, remove []string) []string {
	cur := normalizeTags(splitTags(current))
	m := make(map[string]string, len(cur))
	for _, t := range cur {
		m[strings.ToLower(t)] = t
	}
	for _, t := range normalizeTags(add) {
		m[strings.ToLower(t)] = t
	}
	for _, t := range normalizeTags(remove) {
		delete(m, strings.ToLower(t))
	}
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
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
	s.recordStatusChangeWithDB(s.db, ticketID, userID, fromStatus, toStatus, reason)
}

func (s *TicketService) recordStatusChangeWithDB(db *gorm.DB, ticketID uint, userID uint, fromStatus, toStatus, reason string) {
	statusChange := &models.TicketStatus{
		TicketID:   ticketID,
		UserID:     userID,
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		Reason:     reason,
	}

	if db == nil {
		db = s.db
	}

	if err := db.Create(statusChange).Error; err != nil {
		s.logger.Errorf("Failed to record status change for ticket %d: %v", ticketID, err)
	}
}

// evaluateTicketSLA 根据最新工单状态检查/处理SLA
func (s *TicketService) evaluateTicketSLA(ctx context.Context, ticket *models.Ticket, statusChanged, agentChanged bool) {
	if s.slaService == nil || ticket == nil {
		return
	}

	// 状态流转到 resolved/closed 时，标记解决超时
	if statusChanged && (ticket.Status == "resolved" || ticket.Status == "closed") {
		if err := s.slaService.ResolveViolationsByTicket(ctx, ticket.ID, []string{"resolution"}); err != nil {
			s.logger.Warnf("Failed to resolve SLA resolution violations for ticket %d: %v", ticket.ID, err)
		}
	}

	// 分配客服后，标记首次响应违约
	if agentChanged && ticket.AgentID != nil {
		if err := s.slaService.ResolveViolationsByTicket(ctx, ticket.ID, []string{"first_response"}); err != nil {
			s.logger.Warnf("Failed to resolve SLA first response violations for ticket %d: %v", ticket.ID, err)
		}
	}

	// 主动检查当前工单是否触发新的违约
	if _, err := s.slaService.CheckSLAViolation(ctx, ticket); err != nil {
		s.logger.Warnf("Failed to evaluate SLA violation for ticket %d: %v", ticket.ID, err)
	}
}

// resolveTicketSLAViolations 包装方法
func (s *TicketService) resolveTicketSLAViolations(ctx context.Context, ticketID uint, types []string) {
	if s.slaService == nil {
		return
	}
	if err := s.slaService.ResolveViolationsByTicket(ctx, ticketID, types); err != nil {
		s.logger.Warnf("Failed to resolve SLA violations for ticket %d: %v", ticketID, err)
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
	Total        int64           `json:"total"`
	TodayCreated int64           `json:"today_created"`
	Pending      int64           `json:"pending"`
	Resolved     int64           `json:"resolved"`
	ByStatus     []StatusCount   `json:"by_status"`
	ByPriority   []PriorityCount `json:"by_priority"`
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type PriorityCount struct {
	Priority string `json:"priority"`
	Count    int64  `json:"count"`
}
