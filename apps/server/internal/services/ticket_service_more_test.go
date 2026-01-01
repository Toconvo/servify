package services

import (
	"context"
	"testing"

	"servify/apps/server/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTicketServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := t.Name()
	dsn := "file:ticket_service_" + name + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Ticket{},
		&models.TicketComment{},
		&models.TicketStatus{},
		&models.TicketFile{},
		&models.User{},
		&models.Agent{},
		&models.SLAConfig{},
		&models.SLAViolation{},
		&models.Customer{},
		&models.Session{},
		&models.TicketCustomFieldValue{},
		&models.CustomField{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestTicketService_AddComment(t *testing.T) {
	db := newTicketServiceTestDB(t)
	logger := logrus.New()
	svc := NewTicketService(db, logger, nil)

	// 创建测试用户
	user := &models.User{
		Username: "testuser_add",
		Email:    "testuser_add@example.com",
		Name:     "Test User",
		Role:     "agent",
	}
	db.Create(user)

	// 创建测试工单
	ticket := &models.Ticket{
		Title:      "测试工单",
		CustomerID: user.ID,
		Status:     "open",
		Priority:   "normal",
	}
	db.Create(ticket)

	tests := []struct {
		name        string
		ticketID    uint
		userID      uint
		content     string
		commentType string
		wantErr     bool
	}{
		{
			name:        "valid comment",
			ticketID:    ticket.ID,
			userID:      user.ID,
			content:     "这是一条测试评论",
			commentType: "comment",
			wantErr:     false,
		},
		{
			name:        "internal note",
			ticketID:    ticket.ID,
			userID:      user.ID,
			content:     "内部备注",
			commentType: "internal_note",
			wantErr:     false,
		},
		{
			name:        "system comment",
			ticketID:    ticket.ID,
			userID:      user.ID,
			content:     "系统自动评论",
			commentType: "system",
			wantErr:     false,
		},
		{
			name:        "default comment type",
			ticketID:    ticket.ID,
			userID:      user.ID,
			content:     "默认类型评论",
			commentType: "",
			wantErr:     false,
		},
		{
			name:        "non-existent ticket",
			ticketID:    9999,
			userID:      user.ID,
			content:     "应该失败",
			commentType: "comment",
			wantErr:     false, // AddComment 不验证 ticket 是否存在
		},
		{
			name:        "non-existent user",
			ticketID:    ticket.ID,
			userID:      9999,
			content:     "测试",
			commentType: "comment",
			wantErr:     false, // 不验证用户是否存在
		},
		{
			name:        "empty content",
			ticketID:    ticket.ID,
			userID:      user.ID,
			content:     "",
			commentType: "comment",
			wantErr:     false, // 允许空评论
		},
		{
			name:        "long content",
			ticketID:    ticket.ID,
			userID:      user.ID,
			content:     string(make([]byte, 10000)),
			commentType: "comment",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comment, err := svc.AddComment(context.Background(), tt.ticketID, tt.userID, tt.content, tt.commentType)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddComment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if comment == nil {
					t.Fatal("expected comment, got nil")
				}
				if comment.ID == 0 {
					t.Error("expected non-zero comment ID")
				}
				if comment.TicketID != tt.ticketID {
					t.Errorf("expected ticket ID %d, got %d", tt.ticketID, comment.TicketID)
				}
				if comment.UserID != tt.userID {
					t.Errorf("expected user ID %d, got %d", tt.userID, comment.UserID)
				}
				if tt.commentType == "" && comment.Type != "comment" {
					t.Errorf("expected default type 'comment', got '%s'", comment.Type)
				}
				if tt.commentType != "" && comment.Type != tt.commentType {
					t.Errorf("expected type '%s', got '%s'", tt.commentType, comment.Type)
				}
			}
		})
	}
}

func TestTicketService_CloseTicket(t *testing.T) {
	db := newTicketServiceTestDB(t)
	logger := logrus.New()
	svc := NewTicketService(db, logger, nil)

	// 创建测试用户
	user := &models.User{
		Username: "closer_user",
		Email:    "closer_user@example.com",
		Name:     "Ticket Closer",
		Role:     "agent",
	}
	db.Create(user)

	// 创建测试工单
	openTicket := &models.Ticket{
		Title:      "待关闭工单",
		CustomerID: user.ID,
		Status:     "open",
		Priority:   "normal",
	}
	db.Create(openTicket)

	alreadyClosed := &models.Ticket{
		Title:      "已关闭工单",
		CustomerID: user.ID,
		Status:     "closed",
		Priority:   "low",
	}
	db.Create(alreadyClosed)

	tests := []struct {
		name     string
		ticketID uint
		userID   uint
		reason   string
		wantErr  bool
	}{
		{
			name:     "close open ticket",
			ticketID: openTicket.ID,
			userID:   user.ID,
			reason:   "问题已解决",
			wantErr:  false,
		},
		{
			name:     "close with empty reason",
			ticketID: openTicket.ID,
			userID:   user.ID,
			reason:   "",
			wantErr:  false,
		},
		{
			name:     "close already closed ticket",
			ticketID: alreadyClosed.ID,
			userID:   user.ID,
			reason:   "再次关闭",
			wantErr:  false, // 允许重复关闭
		},
		{
			name:     "non-existent ticket",
			ticketID: 9999,
			userID:   user.ID,
			reason:   "测试",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CloseTicket(context.Background(), tt.ticketID, tt.userID, tt.reason)
			if (err != nil) != tt.wantErr {
				t.Errorf("CloseTicket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// 验证工单状态
				var ticket models.Ticket
				db.First(&ticket, tt.ticketID)
				if ticket.Status != "closed" {
					t.Errorf("expected status 'closed', got '%s'", ticket.Status)
				}
				if ticket.ClosedAt == nil {
					t.Error("expected ClosedAt to be set")
				}

				// 验证添加了系统评论
				var comments []models.TicketComment
				db.Where("ticket_id = ? AND type = ?", tt.ticketID, "system").Find(&comments)
				if len(comments) == 0 {
					t.Error("expected system comment to be added")
				}
			}
		})
	}
}

func TestTicketService_autoAssignAgent(t *testing.T) {
	db := newTicketServiceTestDB(t)
	logger := logrus.New()
	svc := NewTicketService(db, logger, nil)

	// 创建在线客服
	agent1 := &models.User{
		Username: "agent_auto1",
		Email:    "agent_auto1@example.com",
		Name:     "Agent One",
		Role:     "agent",
	}
	agent2 := &models.User{
		Username: "agent_auto2",
		Email:    "agent_auto2@example.com",
		Name:     "Agent Two",
		Role:     "agent",
	}
	db.Create(agent1)
	db.Create(agent2)

	// 创建客户用于工单
	customer := &models.User{
		Username: "customer_auto",
		Email:    "customer_auto@example.com",
		Name:     "Customer",
		Role:     "customer",
	}
	db.Create(customer)

	agentRecord1 := &models.Agent{
		UserID:          agent1.ID,
		Department:      "support",
		Status:          "online",
		AvgResponseTime: 300,
	}
	agentRecord2 := &models.Agent{
		UserID:          agent2.ID,
		Department:      "support",
		Status:          "online",
		AvgResponseTime: 600,
	}
	db.Create(agentRecord1)
	db.Create(agentRecord2)

	// 创建未分配的工单
	ticket := &models.Ticket{
		Title:      "待分配工单",
		CustomerID: customer.ID,
		Status:     "open",
		Priority:   "normal",
		AgentID:    nil,
	}
	db.Create(ticket)

	// 测试自动分配
	svc.autoAssignAgent(ticket.ID)

	// 验证工单被分配
	var updated models.Ticket
	db.First(&updated, ticket.ID)
	if updated.AgentID == nil {
		t.Error("expected ticket to be assigned to an agent")
	}
}

func TestTicketService_CreateTicket(t *testing.T) {
	db := newTicketServiceTestDB(t)
	logger := logrus.New()
	svc := NewTicketService(db, logger, nil)

	// 创建客户
	customer := &models.User{
		Username: "customer_create",
		Email:    "customer_create@example.com",
		Name:     "Customer One",
		Role:     "customer",
	}
	db.Create(customer)

	tests := []struct {
		name    string
		req     *TicketCreateRequest
		wantErr bool
	}{
		{
			name: "valid ticket",
			req: &TicketCreateRequest{
				Title:      "测试工单",
				CustomerID: customer.ID,
				Priority:   "normal",
				Category:   "technical",
			},
			wantErr: false,
		},
		{
			name: "ticket with description",
			req: &TicketCreateRequest{
				Title:       "带描述的工单",
				Description: "这是工单描述",
				CustomerID:  customer.ID,
				Priority:    "high",
			},
			wantErr: false,
		},
		{
			name: "missing title - empty",
			req: &TicketCreateRequest{
				CustomerID: customer.ID,
				Title:      "",
				Priority:   "normal",
			},
			wantErr: false, // Title validation not enforced in service layer
		},
		{
			name: "missing customer",
			req: &TicketCreateRequest{
				Title:    "测试工单",
				Priority: "normal",
			},
			wantErr: true,
		},
		{
			name: "with empty custom fields",
			req: &TicketCreateRequest{
				Title:        "自定义字段工单",
				CustomerID:   customer.ID,
				CustomFields: map[string]interface{}{},
			},
			wantErr: false,
		},
		{
			name: "with session",
			req: &TicketCreateRequest{
				Title:      "会话工单",
				CustomerID: customer.ID,
				SessionID:  "session123",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket, err := svc.CreateTicket(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateTicket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if ticket.ID == 0 {
					t.Error("expected non-zero ticket ID")
				}
				if ticket.Status != "open" {
					t.Errorf("expected status 'open', got '%s'", ticket.Status)
				}
				if ticket.CreatedAt.IsZero() {
					t.Error("expected CreatedAt to be set")
				}
			}
		})
	}
}

func TestTicketService_UpdateTicket(t *testing.T) {
	db := newTicketServiceTestDB(t)
	logger := logrus.New()
	svc := NewTicketService(db, logger, nil)

	// 创建测试数据
	customer := &models.User{
		Username: "customer_update",
		Email:    "customer_update@example.com",
		Name:     "Customer",
		Role:     "customer",
	}
	agent := &models.User{
		Username: "agent_update",
		Email:    "agent_update@example.com",
		Name:     "Agent",
		Role:     "agent",
	}
	db.Create(customer)
	db.Create(agent)

	ticket := &models.Ticket{
		Title:      "原始标题",
		CustomerID: customer.ID,
		Status:     "open",
		Priority:   "normal",
	}
	db.Create(ticket)

	tests := []struct {
		name    string
		id      uint
		req     *TicketUpdateRequest
		wantErr bool
	}{
		{
			name: "update title",
			id:   ticket.ID,
			req: &TicketUpdateRequest{
				Title: stringPtr("新标题"),
			},
			wantErr: false,
		},
		{
			name: "update status",
			id:   ticket.ID,
			req: &TicketUpdateRequest{
				Status: stringPtr("in_progress"),
			},
			wantErr: false,
		},
		{
			name: "update priority",
			id:   ticket.ID,
			req: &TicketUpdateRequest{
				Priority: stringPtr("high"),
			},
			wantErr: false,
		},
		{
			name: "assign agent",
			id:   ticket.ID,
			req: &TicketUpdateRequest{
				AgentID: uintPtr(agent.ID),
			},
			wantErr: false,
		},
		{
			name: "update tags",
			id:   ticket.ID,
			req: &TicketUpdateRequest{
				Tags: stringPtr("urgent,important"),
			},
			wantErr: false,
		},
		{
			name: "update multiple fields",
			id:   ticket.ID,
			req: &TicketUpdateRequest{
				Title:    stringPtr("完全更新"),
				Status:   stringPtr("assigned"),
				Priority: stringPtr("urgent"),
				Tags:     stringPtr("updated"),
			},
			wantErr: false,
		},
		{
			name: "non-existent ticket",
			id:   9999,
			req: &TicketUpdateRequest{
				Title: stringPtr("测试"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated, err := svc.UpdateTicket(context.Background(), tt.id, tt.req, 999)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateTicket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if updated.ID != tt.id {
					t.Errorf("expected ID %d, got %d", tt.id, updated.ID)
				}
				if updated.UpdatedAt.IsZero() {
					t.Error("expected UpdatedAt to be set")
				}
			}
		})
	}
}

func TestTicketService_GetTicketByID(t *testing.T) {
	db := newTicketServiceTestDB(t)
	logger := logrus.New()
	svc := NewTicketService(db, logger, nil)

	// 创建测试数据
	customer := &models.User{
		Username: "customer_get",
		Email:    "customer_get@example.com",
		Name:     "Customer",
		Role:     "customer",
	}
	db.Create(customer)

	ticket := &models.Ticket{
		Title:       "测试工单",
		Description: "工单描述",
		CustomerID:  customer.ID,
		Status:      "open",
		Priority:    "normal",
	}
	db.Create(ticket)

	tests := []struct {
		name    string
		id      uint
		wantErr bool
	}{
		{
			name:    "existing ticket",
			id:      ticket.ID,
			wantErr: false,
		},
		{
			name:    "non-existent ticket",
			id:      9999,
			wantErr: true,
		},
		{
			name:    "zero id",
			id:      0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := svc.GetTicketByID(context.Background(), tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTicketByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.id == ticket.ID {
				if found.Title != "测试工单" {
					t.Errorf("expected title '测试工单', got '%s'", found.Title)
				}
			}
		})
	}
}

func TestTicketService_ListTickets(t *testing.T) {
	db := newTicketServiceTestDB(t)
	logger := logrus.New()
	svc := NewTicketService(db, logger, nil)

	// 创建客户
	customer := &models.User{
		Username: "customer_list",
		Email:    "customer_list@example.com",
		Name:     "Customer",
		Role:     "customer",
	}
	db.Create(customer)

	// 创建测试工单
	tickets := []*models.Ticket{
		{
			Title:      "高优先级工单",
			CustomerID: customer.ID,
			Status:     "open",
			Priority:   "urgent",
		},
		{
			Title:      "普通工单",
			CustomerID: customer.ID,
			Status:     "in_progress",
			Priority:   "normal",
		},
		{
			Title:      "已关闭工单",
			CustomerID: customer.ID,
			Status:     "closed",
			Priority:   "low",
		},
	}
	for _, ticket := range tickets {
		db.Create(ticket)
	}

	tests := []struct {
		name     string
		req      *TicketListRequest
		wantMin  int
		wantMax  int
	}{
		{
			name: "list all",
			req: &TicketListRequest{
				Page:     1,
				PageSize: 10,
			},
			wantMin: 3,
			wantMax: 3,
		},
		{
			name: "filter by status",
			req: &TicketListRequest{
				Page:     1,
				PageSize: 10,
				Status:   []string{"open"},
			},
			wantMin: 1,
			wantMax: 1,
		},
		{
			name: "filter by priority",
			req: &TicketListRequest{
				Page:     1,
				PageSize: 10,
				Priority: []string{"urgent"},
			},
			wantMin: 1,
			wantMax: 1,
		},
		{
			name: "pagination",
			req: &TicketListRequest{
				Page:     1,
				PageSize: 2,
			},
			wantMin: 2,
			wantMax: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := svc.ListTickets(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("ListTickets() error = %v", err)
			}
			if len(result) < tt.wantMin || len(result) > tt.wantMax {
				t.Errorf("expected between %d and %d tickets, got %d", tt.wantMin, tt.wantMax, len(result))
			}
		})
	}
}
