package services

import (
	"context"
	"testing"

	"servify/apps/server/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newMacroTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Macro{}, &models.Ticket{}, &models.TicketComment{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestMacroService_List(t *testing.T) {
	db := newMacroTestDB(t)
	svc := NewMacroService(db)

	// 创建测试数据
	_, _ = svc.Create(context.Background(), &MacroCreateRequest{
		Name:    "欢迎消息",
		Content: "您好，感谢联系我们的客服！",
	})
	macro2, _ := svc.Create(context.Background(), &MacroCreateRequest{
		Name:    "关闭消息",
		Content: "工单已关闭",
	})

	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 macros, got %d", len(list))
	}
	// 验证按更新时间排序
	if list[0].ID != macro2.ID {
		t.Error("expected macros sorted by updated_at DESC")
	}
}

func TestMacroService_Create(t *testing.T) {
	db := newMacroTestDB(t)
	svc := NewMacroService(db)

	tests := []struct {
		name    string
		req     *MacroCreateRequest
		wantErr bool
	}{
		{
			name: "valid macro",
			req: &MacroCreateRequest{
				Name:    "测试宏",
				Content: "这是一条测试消息",
			},
			wantErr: false,
		},
		{
			name: "with description",
			req: &MacroCreateRequest{
				Name:        "带描述的宏",
				Description: "这是一个带描述的宏",
				Content:     "内容",
			},
			wantErr: false,
		},
		{
			name: "with language",
			req: &MacroCreateRequest{
				Name:     "英文宏",
				Content: "Hello",
				Language: "en",
			},
			wantErr: false,
		},
		{
			name: "empty language defaults to zh",
			req: &MacroCreateRequest{
				Name:     "默认语言",
				Content: "内容",
				Language: "",
			},
			wantErr: false,
		},
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			macro, err := svc.Create(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if macro.ID == 0 {
					t.Error("expected non-zero ID")
				}
				if macro.CreatedAt.IsZero() {
					t.Error("expected CreatedAt to be set")
				}
				if macro.Active != true {
					t.Error("expected Active to be true by default")
				}
				if tt.req.Language == "" && macro.Language != "zh" {
					t.Errorf("expected default language 'zh', got '%s'", macro.Language)
				}
			}
		})
	}
}

func TestMacroService_Update(t *testing.T) {
	db := newMacroTestDB(t)
	svc := NewMacroService(db)

	macro, _ := svc.Create(context.Background(), &MacroCreateRequest{
		Name:    "原始宏",
		Content: "原始内容",
	})

	tests := []struct {
		name    string
		id      uint
		req     *MacroUpdateRequest
		wantErr bool
	}{
		{
			name: "update description",
			id:   macro.ID,
			req: &MacroUpdateRequest{
				Description: stringPtr("新描述"),
			},
			wantErr: false,
		},
		{
			name: "update content",
			id:   macro.ID,
			req: &MacroUpdateRequest{
				Content: stringPtr("新内容"),
			},
			wantErr: false,
		},
		{
			name: "update language",
			id:   macro.ID,
			req: &MacroUpdateRequest{
				Language: stringPtr("en"),
			},
			wantErr: false,
		},
		{
			name: "deactivate macro",
			id:   macro.ID,
			req: &MacroUpdateRequest{
				Active: boolPtr(false),
			},
			wantErr: false,
		},
		{
			name: "activate macro",
			id:   macro.ID,
			req: &MacroUpdateRequest{
				Active: boolPtr(true),
			},
			wantErr: false,
		},
		{
			name: "update multiple fields",
			id:   macro.ID,
			req: &MacroUpdateRequest{
				Description: stringPtr("更新的描述"),
				Content:     stringPtr("更新的内容"),
				Language:    stringPtr("zh"),
			},
			wantErr: false,
		},
		{
			name:    "nil request",
			id:      macro.ID,
			req:     nil,
			wantErr: true,
		},
		{
			name:    "non-existent macro",
			id:      9999,
			req:     &MacroUpdateRequest{Description: stringPtr("Test")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated, err := svc.Update(context.Background(), tt.id, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
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

func TestMacroService_Delete(t *testing.T) {
	db := newMacroTestDB(t)
	svc := NewMacroService(db)

	macro, _ := svc.Create(context.Background(), &MacroCreateRequest{
		Name:    "待删除宏",
		Content: "内容",
	})

	// 测试删除存在的宏
	err := svc.Delete(context.Background(), macro.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 验证已删除
	list, _ := svc.List(context.Background())
	if len(list) != 0 {
		t.Fatalf("expected 0 macros after deletion, got %d", len(list))
	}

	// 测试删除不存在的宏
	err = svc.Delete(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for non-existent macro")
	}
}

func TestMacroService_ApplyToTicket(t *testing.T) {
	db := newMacroTestDB(t)
	svc := NewMacroService(db)

	// 创建测试工单
	ticket := &models.Ticket{
		CustomerID: 1,
		Status:     "open",
		Priority:   "1",
		Title:      "测试工单",
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	// 创建活跃的宏
	activeMacro, _ := svc.Create(context.Background(), &MacroCreateRequest{
		Name:    "活跃宏",
		Content: "这是活跃宏的内容",
	})

	// 创建非活跃的宏
	inactiveMacro, _ := svc.Create(context.Background(), &MacroCreateRequest{
		Name:    "非活跃宏",
		Content: "这是非活跃宏的内容",
	})
	svc.Update(context.Background(), inactiveMacro.ID, &MacroUpdateRequest{
		Active: boolPtr(false),
	})

	tests := []struct {
		name     string
		macroID  uint
		ticketID uint
		actorID  uint
		wantErr  bool
	}{
		{
			name:     "apply active macro",
			macroID:  activeMacro.ID,
			ticketID: ticket.ID,
			actorID:  1,
			wantErr:  false,
		},
		{
			name:     "apply inactive macro",
			macroID:  inactiveMacro.ID,
			ticketID: ticket.ID,
			actorID:  1,
			wantErr:  true,
		},
		{
			name:     "non-existent macro",
			macroID:  9999,
			ticketID: ticket.ID,
			actorID:  1,
			wantErr:  true,
		},
		{
			name:     "non-existent ticket",
			macroID:  activeMacro.ID,
			ticketID: 9999,
			actorID:  1,
			wantErr:  false, // 会创建评论，即使工单不存在
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comment, err := svc.ApplyToTicket(context.Background(), tt.macroID, tt.ticketID, tt.actorID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyToTicket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.macroID == activeMacro.ID && tt.ticketID == ticket.ID {
				if comment.Content != "这是活跃宏的内容" {
					t.Errorf("unexpected content: %s", comment.Content)
				}
				if comment.Type != "system" {
					t.Errorf("expected type 'system', got '%s'", comment.Type)
				}
			}
		})
	}
}
