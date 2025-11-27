package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"servify/apps/server/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// AutomationEvent represents an event that can trigger automations.
type AutomationEvent struct {
	Type     string
	TicketID uint
	Payload  interface{}
}

// AutomationService handles trigger evaluation and action execution.
type AutomationService struct {
	db     *gorm.DB
	logger *logrus.Logger
}

func NewAutomationService(db *gorm.DB, logger *logrus.Logger) *AutomationService {
	if logger == nil {
		logger = logrus.New()
	}
	return &AutomationService{db: db, logger: logger}
}

// TriggerCondition describes a single condition entry.
type TriggerCondition struct {
	Field string      `json:"field"`
	Op    string      `json:"op"`
	Value interface{} `json:"value"`
}

// TriggerAction describes an action to execute when trigger matches.
type TriggerAction struct {
	Type   string                 `json:"type"`
	Params map[string]interface{} `json:"params"`
}

// AutomationTriggerRequest 创建触发器的请求
type AutomationTriggerRequest struct {
	Name       string             `json:"name" binding:"required"`
	Event      string             `json:"event" binding:"required"`
	Conditions []TriggerCondition `json:"conditions"`
	Actions    []TriggerAction    `json:"actions"`
	Active     *bool              `json:"active"`
}

// HandleEvent evaluates triggers for the given event.
func (s *AutomationService) HandleEvent(ctx context.Context, evt AutomationEvent) {
	if s.db == nil {
		return
	}
	// 小型速率限制：避免同时多事件击穿
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var triggers []models.AutomationTrigger
	if err := s.db.WithContext(ctx).
		Where("event = ? AND active = true", evt.Type).
		Find(&triggers).Error; err != nil {
		s.logger.Warnf("automation: load triggers failed: %v", err)
		return
	}
	if len(triggers) == 0 {
		return
	}

	var ticket models.Ticket
	if evt.TicketID != 0 {
		s.db.First(&ticket, evt.TicketID)
	}

	for _, trig := range triggers {
		if s.matchTrigger(ctx, trig, evt, &ticket) {
			s.logger.Infof("automation: trigger %s matched event %s", trig.Name, evt.Type)
		}
	}
}

func (s *AutomationService) matchTrigger(ctx context.Context, trig models.AutomationTrigger, evt AutomationEvent, ticket *models.Ticket) bool {
	conds := []TriggerCondition{}
	if trig.Conditions != "" {
		if err := json.Unmarshal([]byte(trig.Conditions), &conds); err != nil {
			s.logger.Warnf("automation: invalid conditions for %s: %v", trig.Name, err)
			return false
		}
	}

	attrs := map[string]interface{}{}
	if ticket != nil {
		attrs["ticket.priority"] = ticket.Priority
		attrs["ticket.status"] = ticket.Status
		attrs["ticket.tags"] = ticket.Tags
	}
	if violation, ok := evt.Payload.(*models.SLAViolation); ok {
		attrs["violation.type"] = violation.ViolationType
	}

	for _, cond := range conds {
		if !evaluateCondition(cond, attrs) {
			return false
		}
	}

	actions := []TriggerAction{}
	if trig.Actions != "" {
		if err := json.Unmarshal([]byte(trig.Actions), &actions); err != nil {
			s.logger.Warnf("automation: invalid actions for %s: %v", trig.Name, err)
			return false
		}
	}

	for _, act := range actions {
		if err := s.executeAction(ctx, act, ticket); err != nil {
			s.logger.Warnf("automation: trigger %s action %s failed: %v", trig.Name, act.Type, err)
			s.recordRun(ctx, trig.ID, evt.TicketID, "failed", err.Error())
			return false
		}
	}
	s.recordRun(ctx, trig.ID, evt.TicketID, "success", "")
	return true
}

func evaluateCondition(cond TriggerCondition, attrs map[string]interface{}) bool {
	val, ok := attrs[cond.Field]
	if !ok {
		return false
	}
	actual := fmt.Sprintf("%v", val)
	expected := fmt.Sprintf("%v", cond.Value)

	switch cond.Op {
	case "eq":
		return actual == expected
	case "neq":
		return actual != expected
	case "contains":
		return strings.Contains(actual, expected)
	default:
		return false
	}
}

func (s *AutomationService) executeAction(ctx context.Context, act TriggerAction, ticket *models.Ticket) error {
	switch act.Type {
	case "set_priority":
		if ticket == nil {
			return fmt.Errorf("ticket not loaded")
		}
		val, _ := act.Params["priority"].(string)
		if val == "" {
			return fmt.Errorf("priority param required")
		}
		return s.db.WithContext(ctx).Model(&models.Ticket{}).
			Where("id = ?", ticket.ID).
			Update("priority", val).Error
	case "add_tag":
		if ticket == nil {
			return fmt.Errorf("ticket not loaded")
		}
		val, _ := act.Params["tag"].(string)
		if val == "" {
			return fmt.Errorf("tag param required")
		}
		tags := ticket.Tags
		if tags == "" {
			tags = val
		} else if !strings.Contains(tags, val) {
			tags = tags + "," + val
		}
		return s.db.WithContext(ctx).Model(&models.Ticket{}).
			Where("id = ?", ticket.ID).
			Update("tags", tags).Error
	case "add_comment":
		if ticket == nil {
			return fmt.Errorf("ticket not loaded")
		}
		content, _ := act.Params["content"].(string)
		if content == "" {
			return fmt.Errorf("content required")
		}
		comment := &models.TicketComment{
			TicketID:  ticket.ID,
			UserID:    0,
			Content:   content,
			Type:      "system",
			CreatedAt: time.Now(),
		}
		return s.db.WithContext(ctx).Create(comment).Error
	case "notify_log":
		msg, _ := act.Params["message"].(string)
		if msg == "" {
			msg = "automation trigger"
		}
		s.logger.Infof("automation notify: %s", msg)
		return nil
	default:
		return fmt.Errorf("unsupported action type: %s", act.Type)
	}
}

func (s *AutomationService) recordRun(ctx context.Context, triggerID uint, ticketID uint, status, message string) {
	run := &models.AutomationRun{
		TriggerID: triggerID,
		TicketID:  ticketID,
		Status:    status,
		Message:   message,
		CreatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(run).Error; err != nil {
		s.logger.Warnf("automation: record run failed: %v", err)
	}
}

// ListTriggers 返回所有触发器
func (s *AutomationService) ListTriggers(ctx context.Context) ([]models.AutomationTrigger, error) {
	var triggers []models.AutomationTrigger
	if err := s.db.WithContext(ctx).Order("id DESC").Find(&triggers).Error; err != nil {
		return nil, err
	}
	return triggers, nil
}

// CreateTrigger 新建触发器
func (s *AutomationService) CreateTrigger(ctx context.Context, req *AutomationTriggerRequest) (*models.AutomationTrigger, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}

	if !isSupportedEvent(req.Event) {
		return nil, fmt.Errorf("unsupported event: %s", req.Event)
	}

	condJSON, err := json.Marshal(req.Conditions)
	if err != nil {
		return nil, fmt.Errorf("invalid conditions: %w", err)
	}

	actJSON, err := json.Marshal(req.Actions)
	if err != nil {
		return nil, fmt.Errorf("invalid actions: %w", err)
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	trigger := &models.AutomationTrigger{
		Name:       req.Name,
		Event:      req.Event,
		Conditions: string(condJSON),
		Actions:    string(actJSON),
		Active:     active,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(trigger).Error; err != nil {
		return nil, err
	}
	return trigger, nil
}

// DeleteTrigger 删除触发器
func (s *AutomationService) DeleteTrigger(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.AutomationTrigger{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("trigger not found")
	}
	return nil
}

func isSupportedEvent(event string) bool {
	switch event {
	case "ticket_created", "ticket_updated", "sla_violation":
		return true
	default:
		return false
	}
}
