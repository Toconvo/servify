package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"servify/apps/server/internal/models"

	"github.com/google/uuid"
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
	Page       int        `form:"page,default=1"`
	PageSize   int        `form:"page_size,default=20"`
	TicketID   *uint      `form:"ticket_id"`
	CustomerID *uint      `form:"customer_id"`
	AgentID    *uint      `form:"agent_id"`
	Rating     []int      `form:"rating"`
	Category   []string   `form:"category"`
	DateFrom   *time.Time `form:"date_from"`
	DateTo     *time.Time `form:"date_to"`
	SortBy     string     `form:"sort_by,default=created_at"`
	SortOrder  string     `form:"sort_order,default=desc"`
}

// SatisfactionStatsResponse 满意度统计响应
type SatisfactionStatsResponse struct {
	TotalRatings       int                         `json:"total_ratings"`
	AverageRating      float64                     `json:"average_rating"`
	RatingDistribution map[int]int                 `json:"rating_distribution"` // rating -> count
	CategoryStats      map[string]SatisfactionStat `json:"category_stats"`
	TrendData          []SatisfactionTrend         `json:"trend_data"`
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

var (
	// ErrSurveyNotFound 表示调查不存在或 token 无效
	ErrSurveyNotFound = errors.New("survey not found")
	// ErrSurveyExpired 表示调查已过期
	ErrSurveyExpired = errors.New("survey expired")
	// ErrSurveyCompleted 表示调查已完成
	ErrSurveyCompleted = errors.New("survey already completed")
)

// SatisfactionSurveyListRequest CSAT 调查分页查询参数
type SatisfactionSurveyListRequest struct {
	Page       int      `form:"page,default=1"`
	PageSize   int      `form:"page_size,default=20"`
	TicketID   *uint    `form:"ticket_id"`
	CustomerID *uint    `form:"customer_id"`
	Status     []string `form:"status"`
	Channel    []string `form:"channel"`
}

// SatisfactionSurveyPreview 提供公共页面展示所需的信息
type SatisfactionSurveyPreview struct {
	TicketID    uint       `json:"ticket_id"`
	TicketTitle string     `json:"ticket_title"`
	AgentName   string     `json:"agent_name"`
	Status      string     `json:"status"`
	Channel     string     `json:"channel"`
	ResolvedAt  *time.Time `json:"resolved_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

const (
	defaultSurveyTTL = 7 * 24 * time.Hour
)

// ScheduleSurvey 在工单关闭后调度并发送满意度调查
func (s *SatisfactionService) ScheduleSurvey(ctx context.Context, ticket *models.Ticket) (*models.SatisfactionSurvey, error) {
	if ticket == nil {
		return nil, fmt.Errorf("ticket required for survey scheduling")
	}

	// 如果已有评价则不再发送调查
	var satisfactionCount int64
	if err := s.db.WithContext(ctx).
		Model(&models.CustomerSatisfaction{}).
		Where("ticket_id = ?", ticket.ID).
		Count(&satisfactionCount).Error; err != nil {
		return nil, fmt.Errorf("failed to check satisfaction status: %w", err)
	}
	if satisfactionCount > 0 {
		return nil, nil
	}

	// 如果存在待发送调查则直接复用
	var existing models.SatisfactionSurvey
	if err := s.db.WithContext(ctx).
		Where("ticket_id = ? AND status IN ?", ticket.ID, []string{"queued", "sent"}).
		Order("created_at DESC").
		First(&existing).Error; err == nil {
		return &existing, nil
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to load existing survey: %w", err)
	}

	now := time.Now()
	expires := now.Add(defaultSurveyTTL)
	channel := detectSurveyChannel(ticket.Source)

	survey := &models.SatisfactionSurvey{
		TicketID:    ticket.ID,
		CustomerID:  ticket.CustomerID,
		AgentID:     ticket.AgentID,
		Channel:     channel,
		Status:      "sent",
		SurveyToken: uuid.NewString(),
		SentAt:      &now,
		ExpiresAt:   &expires,
	}

	if err := s.db.WithContext(ctx).Create(survey).Error; err != nil {
		return nil, fmt.Errorf("failed to schedule satisfaction survey: %w", err)
	}

	s.logger.Infof("Scheduled CSAT survey for ticket %d (token=%s)", ticket.ID, survey.SurveyToken)
	return survey, nil
}

// ListSurveys 获取 CSAT 调查列表
func (s *SatisfactionService) ListSurveys(ctx context.Context, req *SatisfactionSurveyListRequest) ([]models.SatisfactionSurvey, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.SatisfactionSurvey{})

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 20
	}

	if req.TicketID != nil {
		query = query.Where("ticket_id = ?", *req.TicketID)
	}
	if req.CustomerID != nil {
		query = query.Where("customer_id = ?", *req.CustomerID)
	}
	if len(req.Status) > 0 {
		query = query.Where("status IN ?", req.Status)
	}
	if len(req.Channel) > 0 {
		query = query.Where("channel IN ?", req.Channel)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count satisfaction surveys: %w", err)
	}

	if req.PageSize > 0 {
		offset := (req.Page - 1) * req.PageSize
		query = query.Offset(offset).Limit(req.PageSize)
	}

	var surveys []models.SatisfactionSurvey
	if err := query.Order("created_at DESC").Find(&surveys).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list satisfaction surveys: %w", err)
	}

	return surveys, total, nil
}

// GetSurveyPreviewByToken 获取展示用的调查信息
func (s *SatisfactionService) GetSurveyPreviewByToken(ctx context.Context, token string) (*SatisfactionSurveyPreview, error) {
	if token == "" {
		return nil, ErrSurveyNotFound
	}

	var survey models.SatisfactionSurvey
	if err := s.db.WithContext(ctx).Where("survey_token = ?", token).First(&survey).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrSurveyNotFound
		}
		return nil, fmt.Errorf("failed to load survey: %w", err)
	}

	var ticket models.Ticket
	if err := s.db.WithContext(ctx).Preload("Agent").First(&ticket, survey.TicketID).Error; err != nil {
		return nil, fmt.Errorf("failed to load ticket for survey: %w", err)
	}

	agentName := ""
	if ticket.Agent != nil {
		agentName = ticket.Agent.Name
	}

	return &SatisfactionSurveyPreview{
		TicketID:    ticket.ID,
		TicketTitle: ticket.Title,
		AgentName:   agentName,
		Status:      survey.Status,
		Channel:     survey.Channel,
		ResolvedAt:  ticket.ResolvedAt,
		ExpiresAt:   survey.ExpiresAt,
		CompletedAt: survey.CompletedAt,
	}, nil
}

// RespondSurvey 根据 token 记录客户满意度评价
func (s *SatisfactionService) RespondSurvey(ctx context.Context, token string, rating int, comment string) (*models.CustomerSatisfaction, error) {
	if token == "" {
		return nil, ErrSurveyNotFound
	}

	var survey models.SatisfactionSurvey
	if err := s.db.WithContext(ctx).Where("survey_token = ?", token).First(&survey).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrSurveyNotFound
		}
		return nil, fmt.Errorf("failed to load survey: %w", err)
	}

	if survey.Status == "completed" {
		return nil, ErrSurveyCompleted
	}
	if survey.ExpiresAt != nil && time.Now().After(*survey.ExpiresAt) {
		_ = s.db.WithContext(ctx).Model(&models.SatisfactionSurvey{}).
			Where("id = ?", survey.ID).
			Update("status", "expired")
		return nil, ErrSurveyExpired
	}

	req := &SatisfactionCreateRequest{
		TicketID:   survey.TicketID,
		CustomerID: survey.CustomerID,
		AgentID:    survey.AgentID,
		Rating:     rating,
		Comment:    comment,
		Category:   "overall",
	}

	satisfaction, err := s.CreateSatisfaction(ctx, req)
	if err != nil {
		// 若已有评价则返回已存在的记录
		if strings.Contains(err.Error(), "already exists") {
			existing, getErr := s.GetSatisfactionByTicket(ctx, survey.TicketID)
			if getErr == nil && existing != nil {
				return existing, nil
			}
		}
		return nil, err
	}

	completed := time.Now()
	if err := s.db.WithContext(ctx).
		Model(&models.SatisfactionSurvey{}).
		Where("id = ?", survey.ID).
		Updates(map[string]interface{}{
			"status":          "completed",
			"completed_at":    completed,
			"satisfaction_id": satisfaction.ID,
		}).Error; err != nil {
		s.logger.Warnf("Failed to update survey %d after response: %v", survey.ID, err)
	}

	return satisfaction, nil
}

// ResendSurvey 重新发送调查邮件/链接
func (s *SatisfactionService) ResendSurvey(ctx context.Context, id uint) (*models.SatisfactionSurvey, error) {
	var survey models.SatisfactionSurvey
	if err := s.db.WithContext(ctx).First(&survey, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrSurveyNotFound
		}
		return nil, fmt.Errorf("failed to load survey: %w", err)
	}

	if survey.Status == "completed" && survey.SatisfactionID != nil {
		return nil, ErrSurveyCompleted
	}

	originalCompleted := survey.Status == "completed"
	now := time.Now()
	expires := now.Add(defaultSurveyTTL)
	if survey.SurveyToken == "" {
		survey.SurveyToken = uuid.NewString()
	}

	survey.Status = "sent"
	survey.SentAt = &now
	survey.ExpiresAt = &expires
	if !originalCompleted {
		survey.CompletedAt = nil
		survey.SatisfactionID = nil
	}

	if err := s.db.WithContext(ctx).Save(&survey).Error; err != nil {
		return nil, fmt.Errorf("failed to resend survey: %w", err)
	}

	s.logger.Infof("Resent CSAT survey for ticket %d", survey.TicketID)
	return &survey, nil
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

func detectSurveyChannel(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "chat", "im":
		return "chat"
	case "voice", "phone":
		return "voice"
	case "email":
		return "email"
	default:
		return "email"
	}
}
