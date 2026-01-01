package services

import (
	"context"
	"testing"

	"servify/apps/server/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newCustomFieldTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.CustomField{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestCustomFieldService_List(t *testing.T) {
	db := newCustomFieldTestDB(t)
	svc := NewCustomFieldService(db)

	// 创建测试数据
	_, _ = svc.Create(context.Background(), &CustomFieldCreateRequest{
		Key:  "priority",
		Name: "优先级",
		Type: "select",
	})
	_, _ = svc.Create(context.Background(), &CustomFieldCreateRequest{
		Key:  "severity",
		Name: "严重程度",
		Type: "select",
	})

	// 测试列出所有字段
	all, err := svc.List(context.Background(), "ticket", false)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(all))
	}

	// 测试仅列出激活字段
	// 注意: 由于模型中的 Active 字段有 gorm:"default:true"，
	// 所有创建的字段默认都是 active 的
	active, err := svc.List(context.Background(), "ticket", true)
	if err != nil {
		t.Fatalf("List active only failed: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("expected 2 active fields, got %d", len(active))
	}
}

func TestCustomFieldService_Get(t *testing.T) {
	db := newCustomFieldTestDB(t)
	svc := NewCustomFieldService(db)

	field, err := svc.Create(context.Background(), &CustomFieldCreateRequest{
		Key:  "category",
		Name: "分类",
		Type: "select",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 测试获取存在的字段
	found, err := svc.Get(context.Background(), field.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found.Key != "category" {
		t.Fatalf("unexpected key: %s", found.Key)
	}

	// 测试获取不存在的字段
	_, err = svc.Get(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for non-existent field")
	}
}

func TestCustomFieldService_Create(t *testing.T) {
	db := newCustomFieldTestDB(t)
	svc := NewCustomFieldService(db)

	tests := []struct {
		name    string
		req     *CustomFieldCreateRequest
		wantErr bool
	}{
		{
			name: "valid field",
			req: &CustomFieldCreateRequest{
				Key:  "priority",
				Name: "优先级",
				Type: "select",
			},
			wantErr: false,
		},
		{
			name: "field with options",
			req: &CustomFieldCreateRequest{
				Key:     "severity",
				Name:    "严重程度",
				Type:    "select",
				Options: `["low","medium","high"]`,
			},
			wantErr: false,
		},
		{
			name: "nil request",
			req:  nil,
			wantErr: true,
		},
		{
			name: "empty name",
			req: &CustomFieldCreateRequest{
				Key:  "test",
				Name: "   ",
				Type: "string",
			},
			wantErr: true,
		},
		{
			name: "invalid key format",
			req: &CustomFieldCreateRequest{
				Key:  "123invalid",
				Name: "Test",
				Type: "string",
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			req: &CustomFieldCreateRequest{
				Key:  "test",
				Name: "Test",
				Type: "invalid_type",
			},
			wantErr: true,
		},
		{
			name: "unsupported resource",
			req: &CustomFieldCreateRequest{
				Resource: "unsupported",
				Key:      "test",
				Name:     "Test",
				Type:     "string",
			},
			wantErr: true,
		},
		{
			name: "active false",
			req: &CustomFieldCreateRequest{
				Key:    "test_field",
				Name:   "Test Field",
				Type:   "string",
				Active: boolPtr(false),
			},
			wantErr: false,
		},
		{
			name: "required field",
			req: &CustomFieldCreateRequest{
				Key:      "required_field",
				Name:     "Required Field",
				Type:     "string",
				Required: true,
			},
			wantErr: false,
		},
		{
			name: "with validation",
			req: &CustomFieldCreateRequest{
				Key:        "validated_field",
				Name:       "Validated Field",
				Type:       "string",
				Validation: `{"min":5,"max":100}`,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := svc.Create(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if field.ID == 0 {
					t.Error("expected non-zero ID")
				}
				if field.CreatedAt.IsZero() {
					t.Error("expected CreatedAt to be set")
				}
			}
		})
	}
}

func TestCustomFieldService_Update(t *testing.T) {
	db := newCustomFieldTestDB(t)
	svc := NewCustomFieldService(db)

	field, _ := svc.Create(context.Background(), &CustomFieldCreateRequest{
		Key:  "test_field",
		Name: "Test Field",
		Type: "string",
	})

	tests := []struct {
		name    string
		id      uint
		req     *CustomFieldUpdateRequest
		wantErr bool
	}{
		{
			name: "update name",
			id:   field.ID,
			req: &CustomFieldUpdateRequest{
				Name: stringPtr("Updated Name"),
			},
			wantErr: false,
		},
		{
			name: "update type",
			id:   field.ID,
			req: &CustomFieldUpdateRequest{
				Type: stringPtr("number"),
			},
			wantErr: false,
		},
		{
			name: "update active",
			id:   field.ID,
			req: &CustomFieldUpdateRequest{
				Active: boolPtr(false),
			},
			wantErr: false,
		},
		{
			name: "update required",
			id:   field.ID,
			req: &CustomFieldUpdateRequest{
				Required: boolPtr(true),
			},
			wantErr: false,
		},
		{
			name: "update options",
			id:   field.ID,
			req: &CustomFieldUpdateRequest{
				Options: `["a","b","c"]`,
			},
			wantErr: false,
		},
		{
			name: "update validation",
			id:   field.ID,
			req: &CustomFieldUpdateRequest{
				Validation: `{"regex":"^[a-z]+$"}`,
			},
			wantErr: false,
		},
		{
			name: "nil request",
			id:   field.ID,
			req:  nil,
			wantErr: true,
		},
		{
			name:    "non-existent field",
			id:      9999,
			req:     &CustomFieldUpdateRequest{Name: stringPtr("Test")},
			wantErr: true,
		},
		{
			name: "invalid type",
			id:   field.ID,
			req: &CustomFieldUpdateRequest{
				Type: stringPtr("invalid"),
			},
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

func TestCustomFieldService_Delete(t *testing.T) {
	db := newCustomFieldTestDB(t)
	svc := NewCustomFieldService(db)

	field, _ := svc.Create(context.Background(), &CustomFieldCreateRequest{
		Key:  "to_delete",
		Name: "To Delete",
		Type: "string",
	})

	// 测试删除存在的字段
	err := svc.Delete(context.Background(), field.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 验证已删除
	_, err = svc.Get(context.Background(), field.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}

	// 测试删除不存在的字段
	err = svc.Delete(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for non-existent field")
	}
}

func TestCustomFieldService_AllowedTypes(t *testing.T) {
	db := newCustomFieldTestDB(t)
	svc := NewCustomFieldService(db)

	allowedTypes := []string{"string", "number", "boolean", "date", "select", "multiselect"}

	for _, typ := range allowedTypes {
		t.Run(typ, func(t *testing.T) {
			_, err := svc.Create(context.Background(), &CustomFieldCreateRequest{
				Key:  "field_" + typ,
				Name: "Field " + typ,
				Type: typ,
			})
			if err != nil {
				t.Errorf("type %s should be allowed: %v", typ, err)
			}
		})
	}
}
