package services

import (
	"context"
	"fmt"
	"time"

	"servify/apps/server/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// StatisticsService 数据统计服务
type StatisticsService struct {
	db     *gorm.DB
	logger *logrus.Logger
}

// NewStatisticsService 创建统计服务
func NewStatisticsService(db *gorm.DB, logger *logrus.Logger) *StatisticsService {
	if logger == nil {
		logger = logrus.New()
	}

	return &StatisticsService{
		db:     db,
		logger: logger,
	}
}

// DashboardStats 仪表板统计数据
type DashboardStats struct {
	// 总体统计
	TotalCustomers int64 `json:"total_customers"`
	TotalAgents    int64 `json:"total_agents"`
	TotalTickets   int64 `json:"total_tickets"`
	TotalSessions  int64 `json:"total_sessions"`

	// 今日统计
	TodayTickets  int64 `json:"today_tickets"`
	TodaySessions int64 `json:"today_sessions"`
	TodayMessages int64 `json:"today_messages"`

	// 状态统计
	OpenTickets     int64 `json:"open_tickets"`
	AssignedTickets int64 `json:"assigned_tickets"`
	ResolvedTickets int64 `json:"resolved_tickets"`
	ClosedTickets   int64 `json:"closed_tickets"`

	// 在线状态
	OnlineAgents   int64 `json:"online_agents"`
	BusyAgents     int64 `json:"busy_agents"`
	ActiveSessions int64 `json:"active_sessions"`

	// 性能指标
	AvgResponseTime      float64 `json:"avg_response_time"`
	AvgResolutionTime    float64 `json:"avg_resolution_time"`
	CustomerSatisfaction float64 `json:"customer_satisfaction"`

	// AI 使用统计
	AIUsageToday      int64 `json:"ai_usage_today"`
	WeKnoraUsageToday int64 `json:"weknora_usage_today"`
}

// TimeRangeStats 时间范围统计
type TimeRangeStats struct {
	Date                 string  `json:"date"`
	Tickets              int64   `json:"tickets"`
	Sessions             int64   `json:"sessions"`
	Messages             int64   `json:"messages"`
	ResolvedTickets      int64   `json:"resolved_tickets"`
	AvgResponseTime      float64 `json:"avg_response_time"`
	CustomerSatisfaction float64 `json:"customer_satisfaction"`
}

// AgentPerformanceStats 客服绩效统计
type AgentPerformanceStats struct {
	AgentID           uint    `json:"agent_id"`
	AgentName         string  `json:"agent_name"`
	Department        string  `json:"department"`
	TotalTickets      int64   `json:"total_tickets"`
	ResolvedTickets   int64   `json:"resolved_tickets"`
	AvgResponseTime   float64 `json:"avg_response_time"`
	AvgResolutionTime float64 `json:"avg_resolution_time"`
	Rating            float64 `json:"rating"`
	OnlineTime        int64   `json:"online_time"` // 在线时长（分钟）
}

// CategoryStats 分类统计
type CategoryStats struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

// GetDashboardStats 获取仪表板统计数据
func (s *StatisticsService) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats := &DashboardStats{}
	today := time.Now().Truncate(24 * time.Hour)

	// 总体统计
	s.db.Model(&models.User{}).Where("role = ?", "customer").Count(&stats.TotalCustomers)
	s.db.Model(&models.Agent{}).Count(&stats.TotalAgents)
	s.db.Model(&models.Ticket{}).Count(&stats.TotalTickets)
	s.db.Model(&models.Session{}).Count(&stats.TotalSessions)

	// 今日统计
	s.db.Model(&models.Ticket{}).Where("created_at >= ?", today).Count(&stats.TodayTickets)
	s.db.Model(&models.Session{}).Where("created_at >= ?", today).Count(&stats.TodaySessions)
	s.db.Model(&models.Message{}).Where("created_at >= ?", today).Count(&stats.TodayMessages)

	// 状态统计
	s.db.Model(&models.Ticket{}).Where("status = ?", "open").Count(&stats.OpenTickets)
	s.db.Model(&models.Ticket{}).Where("status = ?", "assigned").Count(&stats.AssignedTickets)
	s.db.Model(&models.Ticket{}).Where("status = ?", "resolved").Count(&stats.ResolvedTickets)
	s.db.Model(&models.Ticket{}).Where("status = ?", "closed").Count(&stats.ClosedTickets)

	// 在线状态统计
	s.db.Model(&models.Agent{}).Where("status = ?", "online").Count(&stats.OnlineAgents)
	s.db.Model(&models.Agent{}).Where("status = ?", "busy").Count(&stats.BusyAgents)
	s.db.Model(&models.Session{}).Where("status = ?", "active").Count(&stats.ActiveSessions)

	// 性能指标
	s.db.Model(&models.Agent{}).Select("AVG(avg_response_time)").Row().Scan(&stats.AvgResponseTime)

	// 计算平均解决时间
	var avgResolution float64
	s.db.Model(&models.Ticket{}).
		Where("resolved_at IS NOT NULL").
		Select("AVG(EXTRACT(epoch FROM (resolved_at - created_at)))").
		Row().Scan(&avgResolution)
	stats.AvgResolutionTime = avgResolution

	// 客户满意度（模拟数据，实际应该从评价表获取）
	stats.CustomerSatisfaction = 4.2

	// AI 使用统计
	var dailyStat models.DailyStats
	if err := s.db.Where("date = ?", today).First(&dailyStat).Error; err == nil {
		stats.AIUsageToday = int64(dailyStat.AIUsageCount)
		stats.WeKnoraUsageToday = int64(dailyStat.WeKnoraUsageCount)
	}

	return stats, nil
}

// GetTimeRangeStats 获取时间范围统计
func (s *StatisticsService) GetTimeRangeStats(ctx context.Context, startDate, endDate time.Time) ([]TimeRangeStats, error) {
	var stats []TimeRangeStats

	// 生成日期范围
	current := startDate.Truncate(24 * time.Hour)
	end := endDate.Truncate(24 * time.Hour)

	for current.Before(end) || current.Equal(end) {
		nextDay := current.Add(24 * time.Hour)

		stat := TimeRangeStats{
			Date: current.Format("2006-01-02"),
		}

		// 统计当天数据
		s.db.Model(&models.Ticket{}).
			Where("created_at >= ? AND created_at < ?", current, nextDay).
			Count(&stat.Tickets)

		s.db.Model(&models.Session{}).
			Where("created_at >= ? AND created_at < ?", current, nextDay).
			Count(&stat.Sessions)

		s.db.Model(&models.Message{}).
			Where("created_at >= ? AND created_at < ?", current, nextDay).
			Count(&stat.Messages)

		s.db.Model(&models.Ticket{}).
			Where("resolved_at >= ? AND resolved_at < ?", current, nextDay).
			Count(&stat.ResolvedTickets)

		// 从 DailyStats 表获取其他数据
		var dailyStats models.DailyStats
		if err := s.db.Where("date = ?", current).First(&dailyStats).Error; err == nil {
			stat.AvgResponseTime = float64(dailyStats.AvgResponseTime)
			stat.CustomerSatisfaction = dailyStats.CustomerSatisfaction
		}

		stats = append(stats, stat)
		current = nextDay
	}

	return stats, nil
}

// GetAgentPerformanceStats 获取客服绩效统计
func (s *StatisticsService) GetAgentPerformanceStats(ctx context.Context, startDate, endDate time.Time, limit int) ([]AgentPerformanceStats, error) {
	var stats []AgentPerformanceStats

	query := `
		SELECT
			a.user_id as agent_id,
			u.name as agent_name,
			a.department,
			COUNT(t.id) as total_tickets,
			COUNT(CASE WHEN t.status = 'resolved' OR t.status = 'closed' THEN 1 END) as resolved_tickets,
			a.avg_response_time,
			AVG(CASE WHEN t.resolved_at IS NOT NULL
				THEN EXTRACT(epoch FROM (t.resolved_at - t.created_at))
				END) as avg_resolution_time,
			a.rating
		FROM agents a
		LEFT JOIN users u ON a.user_id = u.id
		LEFT JOIN tickets t ON a.user_id = t.agent_id
			AND t.created_at >= ? AND t.created_at <= ?
		GROUP BY a.user_id, u.name, a.department, a.avg_response_time, a.rating
		ORDER BY total_tickets DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	if err := s.db.Raw(query, startDate, endDate).Scan(&stats).Error; err != nil {
		return nil, fmt.Errorf("failed to get agent performance stats: %w", err)
	}

	return stats, nil
}

// GetTicketCategoryStats 获取工单分类统计
func (s *StatisticsService) GetTicketCategoryStats(ctx context.Context, startDate, endDate time.Time) ([]CategoryStats, error) {
	var stats []CategoryStats

	err := s.db.Model(&models.Ticket{}).
		Select("category, COUNT(*) as count").
		Where("created_at >= ? AND created_at <= ?", startDate, endDate).
		Group("category").
		Order("count DESC").
		Scan(&stats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get category stats: %w", err)
	}

	return stats, nil
}

// GetTicketPriorityStats 获取工单优先级统计
func (s *StatisticsService) GetTicketPriorityStats(ctx context.Context, startDate, endDate time.Time) ([]CategoryStats, error) {
	var stats []CategoryStats

	err := s.db.Model(&models.Ticket{}).
		Select("priority as category, COUNT(*) as count").
		Where("created_at >= ? AND created_at <= ?", startDate, endDate).
		Group("priority").
		Order("count DESC").
		Scan(&stats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get priority stats: %w", err)
	}

	return stats, nil
}

// GetCustomerSourceStats 获取客户来源统计
func (s *StatisticsService) GetCustomerSourceStats(ctx context.Context) ([]CategoryStats, error) {
	var stats []CategoryStats

	err := s.db.Model(&models.Customer{}).
		Select("source as category, COUNT(*) as count").
		Group("source").
		Order("count DESC").
		Scan(&stats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get customer source stats: %w", err)
	}

	return stats, nil
}

// UpdateDailyStats 更新每日统计数据
func (s *StatisticsService) UpdateDailyStats(ctx context.Context, date time.Time) error {
	date = date.Truncate(24 * time.Hour)
	nextDay := date.Add(24 * time.Hour)

	// 获取或创建 DailyStats 记录
	var dailyStats models.DailyStats
	err := s.db.Where("date = ?", date).First(&dailyStats).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			dailyStats = models.DailyStats{Date: date}
		} else {
			return fmt.Errorf("failed to query daily stats: %w", err)
		}
	}

	// 统计当天数据
	var totalSessions int64
	var totalMessages int64

	s.db.Model(&models.Session{}).
		Where("created_at >= ? AND created_at < ?", date, nextDay).
		Count(&totalSessions)

	s.db.Model(&models.Message{}).
		Where("created_at >= ? AND created_at < ?", date, nextDay).
		Count(&totalMessages)

	// 转换为int
	dailyStats.TotalSessions = int(totalSessions)
	dailyStats.TotalMessages = int(totalMessages)

	var totalTickets int64
	s.db.Model(&models.Ticket{}).
		Where("created_at >= ? AND created_at < ?", date, nextDay).
		Count(&totalTickets)
	dailyStats.TotalTickets = int(totalTickets)

	var resolvedTickets int64
	s.db.Model(&models.Ticket{}).
		Where("resolved_at >= ? AND resolved_at < ?", date, nextDay).
		Count(&resolvedTickets)
	dailyStats.ResolvedTickets = int(resolvedTickets)

	// 计算平均响应时间
	var avgResponseTime float64
	s.db.Model(&models.Agent{}).
		Select("AVG(avg_response_time)").
		Row().Scan(&avgResponseTime)
	dailyStats.AvgResponseTime = int(avgResponseTime)

	// 计算平均解决时间
	var avgResolutionTime float64
	s.db.Model(&models.Ticket{}).
		Where("resolved_at >= ? AND resolved_at < ? AND resolved_at IS NOT NULL", date, nextDay).
		Select("AVG(EXTRACT(epoch FROM (resolved_at - created_at)))").
		Row().Scan(&avgResolutionTime)
	dailyStats.AvgResolutionTime = int(avgResolutionTime)

	// 客户满意度（这里使用模拟数据，实际应该从评价系统获取）
	dailyStats.CustomerSatisfaction = 4.2

	// 保存或更新统计数据
	if dailyStats.ID == 0 {
		err = s.db.Create(&dailyStats).Error
	} else {
		err = s.db.Save(&dailyStats).Error
	}

	if err != nil {
		return fmt.Errorf("failed to save daily stats: %w", err)
	}

	s.logger.Infof("Updated daily stats for %s", date.Format("2006-01-02"))
	return nil
}

// IncrementAIUsage 增加 AI 使用计数
func (s *StatisticsService) IncrementAIUsage(ctx context.Context) {
	today := time.Now().Truncate(24 * time.Hour)

	var dailyStats models.DailyStats
	err := s.db.Where("date = ?", today).First(&dailyStats).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			dailyStats = models.DailyStats{
				Date:         today,
				AIUsageCount: 1,
			}
			s.db.Create(&dailyStats)
		}
		return
	}

	s.db.Model(&dailyStats).UpdateColumn("ai_usage_count", gorm.Expr("ai_usage_count + 1"))
}

// IncrementWeKnoraUsage 增加 WeKnora 使用计数
func (s *StatisticsService) IncrementWeKnoraUsage(ctx context.Context) {
	today := time.Now().Truncate(24 * time.Hour)

	var dailyStats models.DailyStats
	err := s.db.Where("date = ?", today).First(&dailyStats).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			dailyStats = models.DailyStats{
				Date:              today,
				WeKnoraUsageCount: 1,
			}
			s.db.Create(&dailyStats)
		}
		return
	}

	s.db.Model(&dailyStats).UpdateColumn("weknora_usage_count", gorm.Expr("weknora_usage_count + 1"))
}

// StartDailyStatsWorker 启动每日统计后台任务
func (s *StatisticsService) StartDailyStatsWorker() {
	ticker := time.NewTicker(1 * time.Hour) // 每小时检查一次
	defer ticker.Stop()

	// 立即更新今天的统计
	go func() {
		if err := s.UpdateDailyStats(context.Background(), time.Now()); err != nil {
			s.logger.Errorf("Failed to update daily stats: %v", err)
		}
	}()

	for range ticker.C {
		// 更新今天的统计
		if err := s.UpdateDailyStats(context.Background(), time.Now()); err != nil {
			s.logger.Errorf("Failed to update daily stats: %v", err)
		}

		// 如果是新的一天，也更新昨天的统计（确保数据完整性）
		yesterday := time.Now().AddDate(0, 0, -1)
		if err := s.UpdateDailyStats(context.Background(), yesterday); err != nil {
			s.logger.Errorf("Failed to update yesterday stats: %v", err)
		}
	}
}
