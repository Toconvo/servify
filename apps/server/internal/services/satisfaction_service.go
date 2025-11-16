package services

import (
	"context"
	"fmt"
	"time"

	"servify/apps/server/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// SatisfactionService 客户满意度管理服务
type SatisfactionService struct {
	db     *gorm.DB
	logger *logrus.Logger
}

// NewSatisfactionService 创建满意度服务
func NewSatisfactionService(db *gorm.DB, logger *logrus.Logger) *SatisfactionService {
	if logger == nil {
		logger = logrus.New()
	}

	return &SatisfactionService{
		db:     db,
		logger: logger,
	}
}

// SatisfactionCreateRequest 创建满意度评价请求
type SatisfactionCreateRequest struct {
	TicketID   uint   `json:"ticket_id" binding:"required"`
	CustomerID uint   `json:"customer_id" binding:"required"`
	AgentID    *uint  `json:"agent_id"`
	Rating     int    `json:"rating" binding:"required,min=1,max=5"`
	Comment    string `json:"comment"`
	Category   string `json:"category"` // service_quality, response_time, resolution_quality, overall
}

// SatisfactionListRequest 满意度评价列表请求
type SatisfactionListRequest struct {
	Page       int      `form:"page,default=1"`
	PageSize   int      `form:"page_size,default=20"`
	TicketID   *uint    `form:"ticket_id"`
	CustomerID *uint    `form:"customer_id"`
	AgentID    *uint    `form:"agent_id"`
	Rating     []int    `form:"rating"`
	Category   []string `form:"category"`
	DateFrom   *time.Time `form:"date_from"`
	DateTo     *time.Time `form:"date_to"`
	SortBy     string   `form:"sort_by,default=created_at"`
	SortOrder  string   `form:"sort_order,default=desc"`
}

// SatisfactionStatsResponse 满意度统计响应
type SatisfactionStatsResponse struct {
	TotalRatings     int                          `json:"total_ratings"`
	AverageRating    float64                      `json:"average_rating"`
	RatingDistribution map[int]int                `json:"rating_distribution"` // rating -> count
	CategoryStats    map[string]SatisfactionStat `json:"category_stats"`
	TrendData        []SatisfactionTrend          `json:"trend_data"`
}

// SatisfactionStat 满意度统计
type SatisfactionStat struct {
	Count         int     `json:"count"`
	AverageRating float64 `json:"average_rating"`
}

// SatisfactionTrend 满意度趋势数据
type SatisfactionTrend struct {
	Date          string  `json:"date"`
	Count         int     `json:"count"`
	AverageRating float64 `json:"average_rating"`
}

// CreateSatisfaction 创建满意度评价
func (s *SatisfactionService) CreateSatisfaction(ctx context.Context, req *SatisfactionCreateRequest) (*models.CustomerSatisfaction, error) {
	// 验证工单是否存在
	var ticket models.Ticket
	if err := s.db.First(&ticket, req.TicketID).Error; err != nil {
		return nil, fmt.Errorf("ticket not found: %w", err)
	}

	// 验证客户是否存在
	var customer models.User
	if err := s.db.First(&customer, req.CustomerID).Error; err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	// 验证客户是否是工单的所有者
	if ticket.CustomerID != req.CustomerID {
		return nil, fmt.Errorf("customer is not the owner of this ticket")
	}

	// 检查是否已经有评价
	var existingSatisfaction models.CustomerSatisfaction
	if err := s.db.Where("ticket_id = ? AND customer_id = ?", req.TicketID, req.CustomerID).First(&existingSatisfaction).Error; err == nil {
		return nil, fmt.Errorf("satisfaction rating already exists for this ticket")
	}

	// 验证客服（如果提供）
	if req.AgentID != nil {
		var agent models.User
		if err := s.db.First(&agent, *req.AgentID).Error; err != nil {
			return nil, fmt.Errorf("agent not found: %w", err)
		}
	}

	// 设置默认分类
	if req.Category == "" {
		req.Category = "overall"
	}

	// 创建满意度评价
	satisfaction := &models.CustomerSatisfaction{
		TicketID:   req.TicketID,
		CustomerID: req.CustomerID,
		AgentID:    req.AgentID,
		Rating:     req.Rating,
		Comment:    req.Comment,
		Category:   req.Category,
		CreatedAt:  time.Now(),
	}

	if err := s.db.Create(satisfaction).Error; err != nil {
		s.logger.Errorf("Failed to create satisfaction: %v", err)
		return nil, fmt.Errorf("failed to create satisfaction: %w", err)
	}

	// 预加载关联数据
	if err := s.db.Preload("Ticket").Preload("Customer").Preload("Agent").First(satisfaction, satisfaction.ID).Error; err != nil {
		s.logger.Warnf("Failed to preload satisfaction data: %v", err)
	}

	s.logger.Infof("Created satisfaction rating: ticket_id=%d, customer_id=%d, rating=%d",
		req.TicketID, req.CustomerID, req.Rating)

	return satisfaction, nil
}

// GetSatisfaction 获取满意度评价
func (s *SatisfactionService) GetSatisfaction(ctx context.Context, id uint) (*models.CustomerSatisfaction, error) {
	var satisfaction models.CustomerSatisfaction
	if err := s.db.Preload("Ticket").Preload("Customer").Preload("Agent").First(&satisfaction, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("satisfaction not found")
		}
		return nil, fmt.Errorf("failed to get satisfaction: %w", err)
	}

	return &satisfaction, nil
}

// ListSatisfactions 获取满意度评价列表
func (s *SatisfactionService) ListSatisfactions(ctx context.Context, req *SatisfactionListRequest) ([]models.CustomerSatisfaction, int64, error) {
	query := s.db.Model(&models.CustomerSatisfaction{})

	// 应用筛选
	if req.TicketID != nil {
		query = query.Where("ticket_id = ?", *req.TicketID)
	}
	if req.CustomerID != nil {
		query = query.Where("customer_id = ?", *req.CustomerID)
	}
	if req.AgentID != nil {
		query = query.Where("agent_id = ?", *req.AgentID)
	}
	if len(req.Rating) > 0 {
		query = query.Where("rating IN ?", req.Rating)
	}
	if len(req.Category) > 0 {
		query = query.Where("category IN ?", req.Category)
	}
	if req.DateFrom != nil {
		query = query.Where("created_at >= ?", *req.DateFrom)
	}
	if req.DateTo != nil {
		query = query.Where("created_at <= ?", *req.DateTo)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count satisfactions: %w", err)
	}

	// 应用排序
	sortField := req.SortBy
	if sortField == "" {
		sortField = "created_at"
	}
	sortOrder := req.SortOrder
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortField, sortOrder))

	// 应用分页
	if req.PageSize > 0 {
		offset := (req.Page - 1) * req.PageSize
		query = query.Offset(offset).Limit(req.PageSize)
	}

	var satisfactions []models.CustomerSatisfaction
	if err := query.Preload("Ticket").Preload("Customer").Preload("Agent").Find(&satisfactions).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list satisfactions: %w", err)
	}

	return satisfactions, total, nil
}

// GetSatisfactionByTicket 根据工单获取满意度评价
func (s *SatisfactionService) GetSatisfactionByTicket(ctx context.Context, ticketID uint) (*models.CustomerSatisfaction, error) {
	var satisfaction models.CustomerSatisfaction
	if err := s.db.Where("ticket_id = ?", ticketID).Preload("Ticket").Preload("Customer").Preload("Agent").First(&satisfaction).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // 返回 nil 表示未找到，这是正常情况
		}
		return nil, fmt.Errorf("failed to get satisfaction by ticket: %w", err)
	}

	return &satisfaction, nil
}

// GetSatisfactionStats 获取满意度统计
func (s *SatisfactionService) GetSatisfactionStats(ctx context.Context, dateFrom, dateTo *time.Time) (*SatisfactionStatsResponse, error) {
	query := s.db.Model(&models.CustomerSatisfaction{})

	// 应用日期筛选
	if dateFrom != nil {
		query = query.Where("created_at >= ?", *dateFrom)
	}
	if dateTo != nil {
		query = query.Where("created_at <= ?", *dateTo)
	}

	// 获取基础统计
	var totalRatings int64
	var avgRating float64

	if err := query.Count(&totalRatings).Error; err != nil {
		return nil, fmt.Errorf("failed to count ratings: %w", err)
	}

	var avgResult struct {
		Average float64
	}
	if err := query.Select("AVG(rating) as average").Scan(&avgResult).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate average rating: %w", err)
	}
	avgRating = avgResult.Average

	// 获取评分分布
	var ratingDistributionResult []struct {
		Rating int
		Count  int
	}
	if err := query.Select("rating, COUNT(*) as count").Group("rating").Scan(&ratingDistributionResult).Error; err != nil {
		return nil, fmt.Errorf("failed to get rating distribution: %w", err)
	}

	ratingDistribution := make(map[int]int)
	for _, item := range ratingDistributionResult {
		ratingDistribution[item.Rating] = item.Count
	}

	// 获取分类统计
	var categoryStatsResult []struct {
		Category string
		Count    int
		Average  float64
	}
	if err := query.Select("category, COUNT(*) as count, AVG(rating) as average").Group("category").Scan(&categoryStatsResult).Error; err != nil {
		return nil, fmt.Errorf("failed to get category stats: %w", err)
	}

	categoryStats := make(map[string]SatisfactionStat)
	for _, item := range categoryStatsResult {
		categoryStats[item.Category] = SatisfactionStat{
			Count:         item.Count,
			AverageRating: item.Average,
		}
	}

	// 获取趋势数据（最近30天）
	var trendData []SatisfactionTrend

	// 根据日期范围决定分组粒度
	var dateFormat string = "DATE(created_at)"

	var trendResult []struct {
		Date    string
		Count   int
		Average float64
	}

	trendQuery := s.db.Model(&models.CustomerSatisfaction{}).
		Select(fmt.Sprintf("%s as date, COUNT(*) as count, AVG(rating) as average", dateFormat)).
		Group("date").
		Order("date")

	if dateFrom != nil {
		trendQuery = trendQuery.Where("created_at >= ?", *dateFrom)
	}
	if dateTo != nil {
		trendQuery = trendQuery.Where("created_at <= ?", *dateTo)
	}

	if err := trendQuery.Scan(&trendResult).Error; err != nil {
		s.logger.Warnf("Failed to get trend data: %v", err)
		// 不返回错误，只是没有趋势数据
	} else {
		for _, item := range trendResult {
			trendData = append(trendData, SatisfactionTrend{
				Date:          item.Date,
				Count:         item.Count,
				AverageRating: item.Average,
			})
		}
	}

	return &SatisfactionStatsResponse{
		TotalRatings:       int(totalRatings),
		AverageRating:      avgRating,
		RatingDistribution: ratingDistribution,
		CategoryStats:      categoryStats,
		TrendData:          trendData,
	}, nil
}

// DeleteSatisfaction 删除满意度评价
func (s *SatisfactionService) DeleteSatisfaction(ctx context.Context, id uint) error {
	result := s.db.Delete(&models.CustomerSatisfaction{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete satisfaction: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("satisfaction not found")
	}

	s.logger.Infof("Deleted satisfaction rating: id=%d", id)
	return nil
}

// UpdateSatisfaction 更新满意度评价（仅允许更新评论）
func (s *SatisfactionService) UpdateSatisfaction(ctx context.Context, id uint, comment string) (*models.CustomerSatisfaction, error) {
	var satisfaction models.CustomerSatisfaction
	if err := s.db.First(&satisfaction, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("satisfaction not found")
		}
		return nil, fmt.Errorf("failed to find satisfaction: %w", err)
	}

	// 只允许更新评论
	satisfaction.Comment = comment

	if err := s.db.Save(&satisfaction).Error; err != nil {
		s.logger.Errorf("Failed to update satisfaction: %v", err)
		return nil, fmt.Errorf("failed to update satisfaction: %w", err)
	}

	// 重新加载关联数据
	if err := s.db.Preload("Ticket").Preload("Customer").Preload("Agent").First(&satisfaction, id).Error; err != nil {
		s.logger.Warnf("Failed to preload satisfaction data: %v", err)
	}

	s.logger.Infof("Updated satisfaction comment: id=%d", id)
	return &satisfaction, nil
}
