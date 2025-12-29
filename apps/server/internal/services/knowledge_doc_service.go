package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"servify/apps/server/internal/models"

	"gorm.io/gorm"
)

type KnowledgeDocService struct {
	db *gorm.DB
}

func NewKnowledgeDocService(db *gorm.DB) *KnowledgeDocService {
	return &KnowledgeDocService{db: db}
}

type KnowledgeDocCreateRequest struct {
	Title    string   `json:"title" binding:"required"`
	Content  string   `json:"content" binding:"required"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
}

type KnowledgeDocUpdateRequest struct {
	Title    *string   `json:"title"`
	Content  *string   `json:"content"`
	Category *string   `json:"category"`
	Tags     *[]string `json:"tags"`
}

type KnowledgeDocListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Category string `form:"category"`
	Search   string `form:"search"`
}

func (s *KnowledgeDocService) Create(ctx context.Context, req *KnowledgeDocCreateRequest) (*models.KnowledgeDoc, error) {
	if req == nil {
		return nil, errors.New("request required")
	}
	title := strings.TrimSpace(req.Title)
	content := strings.TrimSpace(req.Content)
	if title == "" {
		return nil, errors.New("title required")
	}
	if content == "" {
		return nil, errors.New("content required")
	}

	now := time.Now()
	doc := &models.KnowledgeDoc{
		Title:     title,
		Content:   content,
		Category:  strings.TrimSpace(req.Category),
		Tags:      joinTagsCSV(req.Tags),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(doc).Error; err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *KnowledgeDocService) Get(ctx context.Context, id uint) (*models.KnowledgeDoc, error) {
	var doc models.KnowledgeDoc
	if err := s.db.WithContext(ctx).First(&doc, id).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *KnowledgeDocService) List(ctx context.Context, req *KnowledgeDocListRequest) ([]models.KnowledgeDoc, int64, error) {
	page := 1
	pageSize := 20
	if req != nil {
		if req.Page > 0 {
			page = req.Page
		}
		if req.PageSize > 0 {
			pageSize = req.PageSize
		}
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	q := s.db.WithContext(ctx).Model(&models.KnowledgeDoc{})
	if req != nil {
		if c := strings.TrimSpace(req.Category); c != "" {
			q = q.Where("category = ?", c)
		}
		if sTerm := strings.TrimSpace(req.Search); sTerm != "" {
			like := "%" + sTerm + "%"
			q = q.Where("title LIKE ? OR content LIKE ? OR tags LIKE ?", like, like, like)
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var docs []models.KnowledgeDoc
	if err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&docs).Error; err != nil {
		return nil, 0, err
	}
	return docs, total, nil
}

func (s *KnowledgeDocService) Update(ctx context.Context, id uint, req *KnowledgeDocUpdateRequest) (*models.KnowledgeDoc, error) {
	if req == nil {
		return nil, errors.New("request required")
	}
	var doc models.KnowledgeDoc
	if err := s.db.WithContext(ctx).First(&doc, id).Error; err != nil {
		return nil, err
	}

	if req.Title != nil {
		doc.Title = strings.TrimSpace(*req.Title)
	}
	if req.Content != nil {
		doc.Content = strings.TrimSpace(*req.Content)
	}
	if req.Category != nil {
		doc.Category = strings.TrimSpace(*req.Category)
	}
	if req.Tags != nil {
		doc.Tags = joinTagsCSV(*req.Tags)
	}
	if strings.TrimSpace(doc.Title) == "" {
		return nil, errors.New("title required")
	}
	if strings.TrimSpace(doc.Content) == "" {
		return nil, errors.New("content required")
	}

	doc.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&doc).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *KnowledgeDocService) Delete(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.KnowledgeDoc{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func joinTagsCSV(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return strings.Join(out, ",")
}
