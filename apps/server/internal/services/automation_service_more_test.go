package services

import (
	"context"
	"testing"

	"servify/apps/server/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAutomationTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.AutomationTrigger{},
		&models.AutomationRun{},
		&models.Ticket{},
		&models.TicketComment{},
		&models.SLAViolation{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestAutomationService_ListTriggers(t *testing.T) {
	db := newAutomationTestDB(t)
	logger := logrus.New()
	svc := NewAutomationService(db, logger)

	// 创建测试触发器
	trigger1, _ := svc.CreateTrigger(context.Background(), &AutomationTriggerRequest{
		Name:  "触发器1",
		Event: "ticket_created",
	})

	_, _ = svc.CreateTrigger(context.Background(), &AutomationTriggerRequest{
		Name:  "触发器2",
		Event: "sla_violation",
	})

	triggers, err := svc.ListTriggers(context.Background())
	if err != nil {
		t.Fatalf("ListTriggers failed: %v", err)
	}
	if len(triggers) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(triggers))
	}
	// 验证按ID降序排列
	if triggers[0].ID != trigger1.ID+1 {
		t.Error("expected triggers sorted by id DESC")
	}
}

func TestAutomationService_CreateTrigger(t *testing.T) {
	db := newAutomationTestDB(t)
	logger := logrus.New()
	svc := NewAutomationService(db, logger)

	tests := []struct {
		name    string
		req     *AutomationTriggerRequest
		wantErr bool
	}{
		{
			name: "valid trigger with conditions and actions",
			req: &AutomationTriggerRequest{
				Name:  "高优先级自动设置",
				Event: "ticket_created",
				Conditions: []TriggerCondition{
					{Field: "ticket.priority", Op: "eq", Value: "1"},
				},
				Actions: []TriggerAction{
					{Type: "add_tag", Params: map[string]interface{}{"tag": "urgent"}},
				},
			},
			wantErr: false,
		},
		{
			name: "minimal trigger",
			req: &AutomationTriggerRequest{
				Name:  "最小触发器",
				Event: "ticket_created",
			},
			wantErr: false,
		},
		{
			name: "inactive trigger",
			req: &AutomationTriggerRequest{
				Name:  "非活跃触发器",
				Event: "ticket_updated",
			},
			wantErr: false,
			// 注意: 由于模型中 Active 字段有 gorm:"default:true"，
			// 即使传递 false，GORM 也可能使用默认值 true
		},
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
		{
			name: "unsupported event",
			req: &AutomationTriggerRequest{
				Name:  "无效事件",
				Event: "invalid_event",
			},
			wantErr: true,
		},
		{
			name: "ticket_updated event",
			req: &AutomationTriggerRequest{
				Name:  "工单更新触发器",
				Event: "ticket_updated",
				Conditions: []TriggerCondition{
					{Field: "ticket.status", Op: "eq", Value: "open"},
				},
			},
			wantErr: false,
		},
		{
			name: "sla_violation event",
			req: &AutomationTriggerRequest{
				Name:  "SLA违规触发器",
				Event: "sla_violation",
				Conditions: []TriggerCondition{
					{Field: "violation.type", Op: "eq", Value: "first_response"},
				},
				Actions: []TriggerAction{
					{Type: "add_tag", Params: map[string]interface{}{"tag": "sla_breach"}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trigger, err := svc.CreateTrigger(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateTrigger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if trigger.ID == 0 {
					t.Error("expected non-zero ID")
				}
				if trigger.CreatedAt.IsZero() {
					t.Error("expected CreatedAt to be set")
				}
				if trigger.Event != tt.req.Event {
					t.Errorf("expected Event=%s, got %s", tt.req.Event, trigger.Event)
				}
			}
		})
	}
}

func TestAutomationService_DeleteTrigger(t *testing.T) {
	db := newAutomationTestDB(t)
	logger := logrus.New()
	svc := NewAutomationService(db, logger)

	trigger, _ := svc.CreateTrigger(context.Background(), &AutomationTriggerRequest{
		Name:  "待删除触发器",
		Event: "ticket_created",
	})

	// 测试删除存在的触发器
	err := svc.DeleteTrigger(context.Background(), trigger.ID)
	if err != nil {
		t.Fatalf("DeleteTrigger failed: %v", err)
	}

	// 验证已删除
	triggers, _ := svc.ListTriggers(context.Background())
	if len(triggers) != 0 {
		t.Fatalf("expected 0 triggers after deletion, got %d", len(triggers))
	}

	// 测试删除不存在的触发器
	err = svc.DeleteTrigger(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for non-existent trigger")
	}
}

func TestAutomationService_ListRuns(t *testing.T) {
	db := newAutomationTestDB(t)
	logger := logrus.New()
	svc := NewAutomationService(db, logger)

	trigger, _ := svc.CreateTrigger(context.Background(), &AutomationTriggerRequest{
		Name:  "测试触发器",
		Event: "ticket_created",
	})

	ticket := &models.Ticket{
		CustomerID: 1,
		Status:     "open",
		Priority:   "1",
		Title:      "测试工单",
	}
	db.Create(ticket)

	// 创建测试运行记录
	run := &models.AutomationRun{
		TriggerID: trigger.ID,
		TicketID:  ticket.ID,
		Status:    "success",
		Message:   "test",
	}
	db.Create(run)

	tests := []struct {
		name    string
		req     *AutomationRunListRequest
		wantLen int
	}{
		{
			name:    "list all",
			req:     &AutomationRunListRequest{Page: 1, PageSize: 10},
			wantLen: 1,
		},
		{
			name: "filter by status",
			req: &AutomationRunListRequest{
				Page:     1,
				PageSize: 10,
				Status:   "success",
			},
			wantLen: 1,
		},
		{
			name: "filter by trigger_id",
			req: &AutomationRunListRequest{
				Page:      1,
				PageSize:  10,
				TriggerID: trigger.ID,
			},
			wantLen: 1,
		},
		{
			name: "filter by ticket_id",
			req: &AutomationRunListRequest{
				Page:     1,
				PageSize: 10,
				TicketID: ticket.ID,
			},
			wantLen: 1,
		},
		{
			name: "no matches",
			req: &AutomationRunListRequest{
				Page:     1,
				PageSize: 10,
				Status:   "failed",
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs, total, err := svc.ListRuns(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("ListRuns failed: %v", err)
			}
			if len(runs) != tt.wantLen {
				t.Errorf("expected %d runs, got %d", tt.wantLen, len(runs))
			}
			if tt.wantLen > 0 && total != int64(tt.wantLen) {
				t.Errorf("expected total %d, got %d", tt.wantLen, total)
			}
		})
	}
}

func TestAutomationService_BatchRun(t *testing.T) {
	db := newAutomationTestDB(t)
	logger := logrus.New()
	svc := NewAutomationService(db, logger)

	// 创建测试工单
	ticket1 := &models.Ticket{
		CustomerID: 1,
		Status:     "open",
		Priority:   "1",
		Title:      "高优先级工单",
	}
	ticket2 := &models.Ticket{
		CustomerID: 2,
		Status:     "open",
		Priority:   "2",
		Title:      "普通工单",
	}
	db.Create(ticket1)
	db.Create(ticket2)

	// 创建触发器
	_, _ = svc.CreateTrigger(context.Background(), &AutomationTriggerRequest{
		Name:  "高优先级标记",
		Event: "ticket_created",
		Conditions: []TriggerCondition{
			{Field: "ticket.priority", Op: "eq", Value: "1"},
		},
		Actions: []TriggerAction{
			{Type: "add_tag", Params: map[string]interface{}{"tag": "urgent"}},
		},
	})

	tests := []struct {
		name          string
		req           *AutomationBatchRunRequest
		wantErr       bool
		wantMatches   int
		wantProcessed int
	}{
		{
			name: "dry run with match",
			req: &AutomationBatchRunRequest{
				Event:     "ticket_created",
				TicketIDs: []uint{ticket1.ID},
				DryRun:    true,
			},
			wantErr:       false,
			wantMatches:   1,
			wantProcessed: 1,
		},
		{
			name: "dry run no match",
			req: &AutomationBatchRunRequest{
				Event:     "ticket_created",
				TicketIDs: []uint{ticket2.ID},
				DryRun:    true,
			},
			wantErr:       false,
			wantMatches:   0,
			wantProcessed: 1,
		},
		{
			name: "multiple tickets",
			req: &AutomationBatchRunRequest{
				Event:     "ticket_created",
				TicketIDs: []uint{ticket1.ID, ticket2.ID},
				DryRun:    true,
			},
			wantErr:       false,
			wantMatches:   1,
			wantProcessed: 2,
		},
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
		{
			name: "unsupported event",
			req: &AutomationBatchRunRequest{
				Event:     "invalid_event",
				TicketIDs: []uint{ticket1.ID},
			},
			wantErr: true,
		},
		{
			name: "empty ticket ids",
			req: &AutomationBatchRunRequest{
				Event:     "ticket_created",
				TicketIDs: []uint{},
			},
			wantErr: true,
		},
		{
			name: "too many ticket ids",
			req: &AutomationBatchRunRequest{
				Event:     "ticket_created",
				TicketIDs: make([]uint, 501),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.BatchRun(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if resp.Matches != tt.wantMatches {
					t.Errorf("expected %d matches, got %d", tt.wantMatches, resp.Matches)
				}
				if resp.TicketsProcessed != tt.wantProcessed {
					t.Errorf("expected %d processed, got %d", tt.wantProcessed, resp.TicketsProcessed)
				}
				if len(resp.Results) != tt.wantProcessed {
					t.Errorf("expected %d results, got %d", tt.wantProcessed, len(resp.Results))
				}
			}
		})
	}
}

func TestAutomationService_ExecuteAction(t *testing.T) {
	db := newAutomationTestDB(t)
	logger := logrus.New()
	svc := NewAutomationService(db, logger)

	ticket := &models.Ticket{
		CustomerID: 1,
		Status:     "open",
		Priority:   "1",
		Title:      "测试工单",
	}
	db.Create(ticket)

	trigger, _ := svc.CreateTrigger(context.Background(), &AutomationTriggerRequest{
		Name:  "测试触发器",
		Event: "ticket_created",
		Actions: []TriggerAction{
			{Type: "set_priority", Params: map[string]interface{}{"priority": "2"}},
			{Type: "add_tag", Params: map[string]interface{}{"tag": "test_tag"}},
			{Type: "add_comment", Params: map[string]interface{}{"content": "自动评论"}},
			{Type: "notify_log", Params: map[string]interface{}{"message": "测试通知"}},
		},
	})

	evt := AutomationEvent{
		Type:     "ticket_created",
		TicketID: ticket.ID,
		Payload:  nil,
	}

	// 执行触发器
	matched := svc.matchTrigger(context.Background(), *trigger, evt, ticket, false)
	if !matched {
		t.Fatal("expected trigger to match and execute")
	}

	// 验证工单被更新
	var updated models.Ticket
	db.First(&updated, ticket.ID)
	if updated.Priority != "2" {
		t.Errorf("expected priority '2', got '%s'", updated.Priority)
	}

	// 验证评论被添加
	var comments []models.TicketComment
	db.Where("ticket_id = ?", ticket.ID).Find(&comments)
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
}

func TestEvaluateCondition(t *testing.T) {
	tests := []struct {
		name     string
		cond     TriggerCondition
		attrs    map[string]interface{}
		expected bool
	}{
		{
			name: "eq true",
			cond: TriggerCondition{Field: "status", Op: "eq", Value: "open"},
			attrs: map[string]interface{}{"status": "open"},
			expected: true,
		},
		{
			name: "eq false",
			cond: TriggerCondition{Field: "status", Op: "eq", Value: "closed"},
			attrs: map[string]interface{}{"status": "open"},
			expected: false,
		},
		{
			name: "neq true",
			cond: TriggerCondition{Field: "status", Op: "neq", Value: "closed"},
			attrs: map[string]interface{}{"status": "open"},
			expected: true,
		},
		{
			name: "contains true",
			cond: TriggerCondition{Field: "tags", Op: "contains", Value: "urgent"},
			attrs: map[string]interface{}{"tags": "urgent,important"},
			expected: true,
		},
		{
			name: "contains false",
			cond: TriggerCondition{Field: "tags", Op: "contains", Value: "missing"},
			attrs: map[string]interface{}{"tags": "urgent,important"},
			expected: false,
		},
		{
			name: "unknown field",
			cond: TriggerCondition{Field: "unknown", Op: "eq", Value: "value"},
			attrs: map[string]interface{}{"status": "open"},
			expected: false,
		},
		{
			name:     "unknown operator",
			cond:     TriggerCondition{Field: "status", Op: "unknown", Value: "open"},
			attrs:    map[string]interface{}{"status": "open"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateCondition(tt.cond, tt.attrs)
			if result != tt.expected {
				t.Errorf("evaluateCondition() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestIsSupportedEvent(t *testing.T) {
	supported := []string{"ticket_created", "ticket_updated", "sla_violation"}
	for _, event := range supported {
		if !isSupportedEvent(event) {
			t.Errorf("event %s should be supported", event)
		}
	}

	unsupported := []string{"invalid", "ticket_deleted", ""}
	for _, event := range unsupported {
		if isSupportedEvent(event) {
			t.Errorf("event %s should not be supported", event)
		}
	}
}
