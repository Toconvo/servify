package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"servify/apps/server/internal/models"

	"gorm.io/gorm"
)

// CustomFieldService manages dynamic custom fields for resources (currently: tickets).
type CustomFieldService struct {
	db *gorm.DB
}

func NewCustomFieldService(db *gorm.DB) *CustomFieldService {
	return &CustomFieldService{db: db}
}

type CustomFieldCreateRequest struct {
	Resource   string      `json:"resource"` // default: ticket
	Key        string      `json:"key" binding:"required"`
	Name       string      `json:"name" binding:"required"`
	Type       string      `json:"type" binding:"required"`
	Required   bool        `json:"required"`
	Active     *bool       `json:"active"`
	Options    interface{} `json:"options"`
	Validation interface{} `json:"validation"`
	ShowWhen   interface{} `json:"show_when"`
}

type CustomFieldUpdateRequest struct {
	Name       *string     `json:"name"`
	Type       *string     `json:"type"`
	Required   *bool       `json:"required"`
	Active     *bool       `json:"active"`
	Options    interface{} `json:"options"`
	Validation interface{} `json:"validation"`
	ShowWhen   interface{} `json:"show_when"`
}

var customFieldKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func (s *CustomFieldService) List(ctx context.Context, resource string, activeOnly bool) ([]models.CustomField, error) {
	if resource == "" {
		resource = "ticket"
	}
	q := s.db.WithContext(ctx).Model(&models.CustomField{}).Where("resource = ?", resource).Order("id ASC")
	if activeOnly {
		q = q.Where("active = ?", true)
	}
	var fields []models.CustomField
	if err := q.Find(&fields).Error; err != nil {
		return nil, err
	}
	return fields, nil
}

func (s *CustomFieldService) Get(ctx context.Context, id uint) (*models.CustomField, error) {
	var field models.CustomField
	if err := s.db.WithContext(ctx).First(&field, id).Error; err != nil {
		return nil, err
	}
	return &field, nil
}

func (s *CustomFieldService) Create(ctx context.Context, req *CustomFieldCreateRequest) (*models.CustomField, error) {
	if req == nil {
		return nil, errors.New("request required")
	}
	resource := strings.TrimSpace(req.Resource)
	if resource == "" {
		resource = "ticket"
	}
	if resource != "ticket" {
		return nil, fmt.Errorf("unsupported resource: %s", resource)
	}

	key := strings.ToLower(strings.TrimSpace(req.Key))
	if !customFieldKeyRe.MatchString(key) {
		return nil, fmt.Errorf("invalid key: %s (must match %s)", key, customFieldKeyRe.String())
	}
	typ := strings.TrimSpace(req.Type)
	if !isAllowedCustomFieldType(typ) {
		return nil, fmt.Errorf("invalid type: %s", typ)
	}

	optionsJSON, err := marshalOptionalJSON(req.Options)
	if err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}
	validationJSON, err := marshalOptionalJSON(req.Validation)
	if err != nil {
		return nil, fmt.Errorf("invalid validation: %w", err)
	}
	showWhenJSON, err := marshalOptionalJSON(req.ShowWhen)
	if err != nil {
		return nil, fmt.Errorf("invalid show_when: %w", err)
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	now := time.Now()
	field := &models.CustomField{
		Resource:       resource,
		Key:            key,
		Name:           strings.TrimSpace(req.Name),
		Type:           typ,
		Required:       req.Required,
		Active:         active,
		OptionsJSON:    optionsJSON,
		ValidationJSON: validationJSON,
		ShowWhenJSON:   showWhenJSON,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if strings.TrimSpace(field.Name) == "" {
		return nil, errors.New("name required")
	}

	if err := s.db.WithContext(ctx).Create(field).Error; err != nil {
		return nil, err
	}
	return field, nil
}

func (s *CustomFieldService) Update(ctx context.Context, id uint, req *CustomFieldUpdateRequest) (*models.CustomField, error) {
	if req == nil {
		return nil, errors.New("request required")
	}
	var field models.CustomField
	if err := s.db.WithContext(ctx).First(&field, id).Error; err != nil {
		return nil, err
	}
	if req.Name != nil {
		field.Name = strings.TrimSpace(*req.Name)
	}
	if req.Type != nil {
		typ := strings.TrimSpace(*req.Type)
		if !isAllowedCustomFieldType(typ) {
			return nil, fmt.Errorf("invalid type: %s", typ)
		}
		field.Type = typ
	}
	if req.Required != nil {
		field.Required = *req.Required
	}
	if req.Active != nil {
		field.Active = *req.Active
	}
	if req.Options != nil {
		j, err := marshalOptionalJSON(req.Options)
		if err != nil {
			return nil, fmt.Errorf("invalid options: %w", err)
		}
		field.OptionsJSON = j
	}
	if req.Validation != nil {
		j, err := marshalOptionalJSON(req.Validation)
		if err != nil {
			return nil, fmt.Errorf("invalid validation: %w", err)
		}
		field.ValidationJSON = j
	}
	if req.ShowWhen != nil {
		j, err := marshalOptionalJSON(req.ShowWhen)
		if err != nil {
			return nil, fmt.Errorf("invalid show_when: %w", err)
		}
		field.ShowWhenJSON = j
	}

	field.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&field).Error; err != nil {
		return nil, err
	}
	return &field, nil
}

func (s *CustomFieldService) Delete(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.CustomField{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("custom field not found")
	}
	return nil
}

func isAllowedCustomFieldType(typ string) bool {
	switch typ {
	case "string", "number", "boolean", "date", "select", "multiselect":
		return true
	default:
		return false
	}
}

func marshalOptionalJSON(v interface{}) (string, error) {
	if v == nil {
		return "", nil
	}
	// allow callers to pass raw JSON string
	if s, ok := v.(string); ok {
		if strings.TrimSpace(s) == "" {
			return "", nil
		}
		var tmp interface{}
		if err := json.Unmarshal([]byte(s), &tmp); err != nil {
			return "", err
		}
		return strings.TrimSpace(s), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
