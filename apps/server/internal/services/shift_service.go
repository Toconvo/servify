package services

import (
	"context"
	"fmt"
	"time"

	"servify/apps/server/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ShiftService 班次管理服务
type ShiftService struct {
	db     *gorm.DB
	logger *logrus.Logger
}

// NewShiftService creates a new ShiftService.
func NewShiftService(db *gorm.DB, logger *logrus.Logger) *ShiftService {
	if logger == nil {
		logger = logrus.New()
	}

	return &ShiftService{db: db, logger: logger}
}

// ShiftCreateRequest 创建班次请求
type ShiftCreateRequest struct {
	AgentID   uint      `json:"agent_id" binding:"required"`
	ShiftType string    `json:"shift_type" binding:"required"` // morning, afternoon, evening, night
	StartTime time.Time `json:"start_time" binding:"required"`
	EndTime   time.Time `json:"end_time" binding:"required"`
	Status    string    `json:"status"` // scheduled, active, completed, cancelled
}

// ShiftListRequest 班次列表请求
type ShiftListRequest struct {
	Page      int        `form:"page,default=1"`
	PageSize  int        `form:"page_size,default=20"`
	AgentID   *uint      `form:"agent_id"`
	ShiftType []string   `form:"shift_type"`
	Status    []string   `form:"status"`
	DateFrom  *time.Time `form:"date_from"`
	DateTo    *time.Time `form:"date_to"`
	SortBy    string     `form:"sort_by,default=start_time"`
	SortOrder string     `form:"sort_order,default=asc"`
}

// ShiftUpdateRequest 更新班次请求
type ShiftUpdateRequest struct {
	ShiftType *string    `json:"shift_type"`
	StartTime *time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
	Status    *string    `json:"status"`
}

// ShiftStatsResponse 班次统计响应
type ShiftStatsResponse struct {
	Total       int            `json:"total"`
	ByType      map[string]int `json:"by_type"`
	ByStatus    map[string]int `json:"by_status"`
	Upcoming    int            `json:"upcoming"`
	TodayActive int            `json:"today_active"`
}

// CreateShift 创建班次
func (s *ShiftService) CreateShift(ctx context.Context, req *ShiftCreateRequest) (*models.ShiftSchedule, error) {
	// basic validation: end > start
	if !req.EndTime.After(req.StartTime) {
		return nil, fmt.Errorf("end_time must be after start_time")
	}

	shift := &models.ShiftSchedule{
		AgentID:   req.AgentID,
		ShiftType: req.ShiftType,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Status:    "scheduled",
		Date:      req.StartTime.Truncate(24 * time.Hour),
	}
	if req.Status != "" {
		shift.Status = req.Status
	}

	if err := s.db.WithContext(ctx).Create(shift).Error; err != nil {
		return nil, fmt.Errorf("failed to create shift: %w", err)
	}

	return shift, nil
}

// ListShifts 获取班次列表
func (s *ShiftService) ListShifts(ctx context.Context, req *ShiftListRequest) ([]models.ShiftSchedule, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.ShiftSchedule{}).Preload("Agent")

	if req.AgentID != nil {
		query = query.Where("agent_id = ?", *req.AgentID)
	}
	if len(req.ShiftType) > 0 {
		query = query.Where("shift_type IN ?", req.ShiftType)
	}
	if len(req.Status) > 0 {
		query = query.Where("status IN ?", req.Status)
	}
	if req.DateFrom != nil {
		query = query.Where("date >= ?", req.DateFrom)
	}
	if req.DateTo != nil {
		query = query.Where("date <= ?", req.DateTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count shifts: %w", err)
	}

	sortField := req.SortBy
	if sortField == "" {
		sortField = "start_time"
	}
	sortOrder := req.SortOrder
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}

	query = query.Order(fmt.Sprintf("%s %s", sortField, sortOrder))

	if req.PageSize > 0 {
		offset := (req.Page - 1) * req.PageSize
		query = query.Offset(offset).Limit(req.PageSize)
	}

	var shifts []models.ShiftSchedule
	if err := query.Find(&shifts).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list shifts: %w", err)
	}

	return shifts, total, nil
}

// UpdateShift 更新班次
func (s *ShiftService) UpdateShift(ctx context.Context, id uint, req *ShiftUpdateRequest) (*models.ShiftSchedule, error) {
	var shift models.ShiftSchedule
	if err := s.db.WithContext(ctx).First(&shift, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("shift not found")
		}
		return nil, fmt.Errorf("failed to get shift: %w", err)
	}

	if req.ShiftType != nil {
		shift.ShiftType = *req.ShiftType
	}
	if req.StartTime != nil {
		shift.StartTime = *req.StartTime
		shift.Date = req.StartTime.Truncate(24 * time.Hour)
	}
	if req.EndTime != nil {
		shift.EndTime = *req.EndTime
	}
	if req.Status != nil {
		shift.Status = *req.Status
	}

	if !shift.EndTime.After(shift.StartTime) {
		return nil, fmt.Errorf("end_time must be after start_time")
	}

	if err := s.db.WithContext(ctx).Save(&shift).Error; err != nil {
		return nil, fmt.Errorf("failed to update shift: %w", err)
	}

	return &shift, nil
}

// DeleteShift 删除班次
func (s *ShiftService) DeleteShift(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.ShiftSchedule{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete shift: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("shift not found")
	}
	return nil
}

// GetShiftStats 获取班次统计
func (s *ShiftService) GetShiftStats(ctx context.Context) (*ShiftStatsResponse, error) {
	stats := &ShiftStatsResponse{
		ByType:   make(map[string]int),
		ByStatus: make(map[string]int),
	}

	var total int64
	if err := s.db.WithContext(ctx).Model(&models.ShiftSchedule{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count shifts: %w", err)
	}
	stats.Total = int(total)

	var byType []struct {
		ShiftType string
		Count     int
	}
	if err := s.db.WithContext(ctx).Model(&models.ShiftSchedule{}).
		Select("shift_type, COUNT(*) as count").
		Group("shift_type").Scan(&byType).Error; err != nil {
		return nil, fmt.Errorf("failed to aggregate by shift_type: %w", err)
	}
	for _, item := range byType {
		stats.ByType[item.ShiftType] = item.Count
	}

	var byStatus []struct {
		Status string
		Count  int
	}
	if err := s.db.WithContext(ctx).Model(&models.ShiftSchedule{}).
		Select("status, COUNT(*) as count").
		Group("status").Scan(&byStatus).Error; err != nil {
		return nil, fmt.Errorf("failed to aggregate by status: %w", err)
	}
	for _, item := range byStatus {
		stats.ByStatus[item.Status] = item.Count
	}

	// upcoming: start in the future
	now := time.Now()
	var upcoming int64
	if err := s.db.WithContext(ctx).Model(&models.ShiftSchedule{}).
		Where("start_time > ?", now).
		Count(&upcoming).Error; err != nil {
		return nil, fmt.Errorf("failed to count upcoming shifts: %w", err)
	}
	stats.Upcoming = int(upcoming)

	// today active: overlapping today
	dayStart := time.Now().Truncate(24 * time.Hour)
	dayEnd := dayStart.Add(24 * time.Hour)
	var todayActive int64
	if err := s.db.WithContext(ctx).Model(&models.ShiftSchedule{}).
		Where("start_time < ? AND end_time > ?", dayEnd, dayStart).
		Count(&todayActive).Error; err != nil {
		return nil, fmt.Errorf("failed to count today active shifts: %w", err)
	}
	stats.TodayActive = int(todayActive)

	return stats, nil
}
