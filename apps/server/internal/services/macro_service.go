package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"servify/apps/server/internal/models"

	"gorm.io/gorm"
)

// MacroService 管理宏模板
//
//nolint:revive
type MacroService struct {
	db *gorm.DB
}

func NewMacroService(db *gorm.DB) *MacroService { return &MacroService{db: db} }

// MacroCreateRequest 创建请求
type MacroCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Content     string `json:"content" binding:"required"`
	Language    string `json:"language"`
}

// MacroUpdateRequest 更新请求
type MacroUpdateRequest struct {
	Description *string `json:"description"`
	Content     *string `json:"content"`
	Language    *string `json:"language"`
	Active      *bool   `json:"active"`
}

func (s *MacroService) List(ctx context.Context) ([]models.Macro, error) {
	var macros []models.Macro
	if err := s.db.WithContext(ctx).Order("updated_at DESC").Find(&macros).Error; err != nil {
		return nil, err
	}
	return macros, nil
}

func (s *MacroService) Create(ctx context.Context, req *MacroCreateRequest) (*models.Macro, error) {
	if req == nil {
		return nil, errors.New("request required")
	}
	macro := &models.Macro{
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		Language:    defaultLang(req.Language),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(macro).Error; err != nil {
		return nil, err
	}
	return macro, nil
}

func (s *MacroService) Update(ctx context.Context, id uint, req *MacroUpdateRequest) (*models.Macro, error) {
	if req == nil {
		return nil, errors.New("request required")
	}
	var macro models.Macro
	if err := s.db.WithContext(ctx).First(&macro, id).Error; err != nil {
		return nil, err
	}
	if req.Description != nil {
		macro.Description = *req.Description
	}
	if req.Content != nil {
		macro.Content = *req.Content
	}
	if req.Language != nil {
		macro.Language = defaultLang(*req.Language)
	}
	if req.Active != nil {
		macro.Active = *req.Active
	}
	macro.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&macro).Error; err != nil {
		return nil, err
	}
	return &macro, nil
}

func (s *MacroService) Delete(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.Macro{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("macro not found")
	}
	return nil
}

func (s *MacroService) ApplyToTicket(ctx context.Context, macroID, ticketID, actorID uint) (*models.TicketComment, error) {
	var macro models.Macro
	if err := s.db.WithContext(ctx).First(&macro, macroID).Error; err != nil {
		return nil, err
	}
	if !macro.Active {
		return nil, fmt.Errorf("macro inactive")
	}
	comment := &models.TicketComment{
		TicketID:  ticketID,
		UserID:    actorID,
		Content:   macro.Content,
		Type:      "system",
		CreatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(comment).Error; err != nil {
		return nil, err
	}
	return comment, nil
}

func defaultLang(lang string) string {
	if lang == "" {
		return "zh"
	}
	return lang
}
