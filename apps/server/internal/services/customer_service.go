package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"servify/apps/server/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// CustomerService 客户管理服务
type CustomerService struct {
	db     *gorm.DB
	logger *logrus.Logger
}

// NewCustomerService 创建客户服务
func NewCustomerService(db *gorm.DB, logger *logrus.Logger) *CustomerService {
	if logger == nil {
		logger = logrus.New()
	}

	return &CustomerService{
		db:     db,
		logger: logger,
	}
}

// CustomerCreateRequest 创建客户请求
type CustomerCreateRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Company  string `json:"company"`
	Industry string `json:"industry"`
	Source   string `json:"source"`
	Tags     string `json:"tags"`
	Notes    string `json:"notes"`
	Priority string `json:"priority"`
}

// CustomerUpdateRequest 更新客户请求
type CustomerUpdateRequest struct {
	Name     *string `json:"name"`
	Phone    *string `json:"phone"`
	Company  *string `json:"company"`
	Industry *string `json:"industry"`
	Source   *string `json:"source"`
	Tags     *string `json:"tags"`
	Notes    *string `json:"notes"`
	Priority *string `json:"priority"`
	Status   *string `json:"status"`
}

// CustomerListRequest 客户列表请求
type CustomerListRequest struct {
	Page      int      `form:"page,default=1"`
	PageSize  int      `form:"page_size,default=20"`
	Search    string   `form:"search"`
	Industry  []string `form:"industry"`
	Source    []string `form:"source"`
	Priority  []string `form:"priority"`
	Status    []string `form:"status"`
	Tags      string   `form:"tags"`
	SortBy    string   `form:"sort_by,default=created_at"`
	SortOrder string   `form:"sort_order,default=desc"`
}

// CreateCustomer 创建客户
func (s *CustomerService) CreateCustomer(ctx context.Context, req *CustomerCreateRequest) (*models.User, error) {
	// 检查用户名和邮箱是否已存在
	var existingUser models.User
	if err := s.db.Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error; err == nil {
		return nil, fmt.Errorf("username or email already exists")
	}

	// 创建用户
	user := &models.User{
		Username: req.Username,
		Email:    req.Email,
		Name:     req.Name,
		Phone:    req.Phone,
		Role:     "customer",
		Status:   "active",
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// 创建客户扩展信息
	customer := &models.Customer{
		UserID:   user.ID,
		Company:  req.Company,
		Industry: req.Industry,
		Source:   req.Source,
		Tags:     req.Tags,
		Notes:    req.Notes,
		Priority: req.Priority,
	}

	if customer.Source == "" {
		customer.Source = "web"
	}
	if customer.Priority == "" {
		customer.Priority = "normal"
	}

	if err := s.db.Create(customer).Error; err != nil {
		// 如果创建客户信息失败，删除已创建的用户
		s.db.Delete(user)
		return nil, fmt.Errorf("failed to create customer info: %w", err)
	}

	s.logger.Infof("Created customer %d (%s)", user.ID, user.Email)

	// 返回完整的客户信息
	return s.GetCustomerByID(ctx, user.ID)
}

// GetCustomerByID 根据ID获取客户
func (s *CustomerService) GetCustomerByID(ctx context.Context, customerID uint) (*models.User, error) {
	var user models.User
	err := s.db.Preload("Sessions", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC").Limit(10)
	}).Preload("Tickets", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC").Limit(10)
	}).First(&user, customerID).Error

	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	// 获取客户扩展信息
	var customer models.Customer
	if err := s.db.Where("user_id = ?", customerID).First(&customer).Error; err == nil {
		// 将客户信息合并到用户对象中（这里可以使用自定义结构体）
		user.Name = customer.User.Name
	}

	return &user, nil
}

// UpdateCustomer 更新客户信息
func (s *CustomerService) UpdateCustomer(ctx context.Context, customerID uint, req *CustomerUpdateRequest) (*models.User, error) {
	// 更新用户基本信息
	userUpdates := make(map[string]interface{})
	if req.Name != nil {
		userUpdates["name"] = *req.Name
	}
	if req.Phone != nil {
		userUpdates["phone"] = *req.Phone
	}
	if req.Status != nil {
		userUpdates["status"] = *req.Status
	}

	if len(userUpdates) > 0 {
		if err := s.db.Model(&models.User{}).Where("id = ?", customerID).Updates(userUpdates).Error; err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
	}

	// 更新客户扩展信息
	customerUpdates := make(map[string]interface{})
	if req.Company != nil {
		customerUpdates["company"] = *req.Company
	}
	if req.Industry != nil {
		customerUpdates["industry"] = *req.Industry
	}
	if req.Source != nil {
		customerUpdates["source"] = *req.Source
	}
	if req.Tags != nil {
		customerUpdates["tags"] = *req.Tags
	}
	if req.Notes != nil {
		customerUpdates["notes"] = *req.Notes
	}
	if req.Priority != nil {
		customerUpdates["priority"] = *req.Priority
	}

	if len(customerUpdates) > 0 {
		if err := s.db.Model(&models.Customer{}).Where("user_id = ?", customerID).Updates(customerUpdates).Error; err != nil {
			return nil, fmt.Errorf("failed to update customer: %w", err)
		}
	}

	s.logger.Infof("Updated customer %d", customerID)

	// 返回更新后的客户信息
	return s.GetCustomerByID(ctx, customerID)
}

// ListCustomers 获取客户列表
func (s *CustomerService) ListCustomers(ctx context.Context, req *CustomerListRequest) ([]CustomerInfo, int64, error) {
	// 构建查询
	query := s.db.Table("users").
		Select("users.*, customers.company, customers.industry, customers.source, customers.tags, customers.notes, customers.priority").
		Joins("LEFT JOIN customers ON users.id = customers.user_id").
		Where("users.role = ?", "customer")

	// 应用过滤条件
	if len(req.Industry) > 0 {
		query = query.Where("customers.industry IN ?", req.Industry)
	}
	if len(req.Source) > 0 {
		query = query.Where("customers.source IN ?", req.Source)
	}
	if len(req.Priority) > 0 {
		query = query.Where("customers.priority IN ?", req.Priority)
	}
	if len(req.Status) > 0 {
		query = query.Where("users.status IN ?", req.Status)
	}

	// 标签过滤
	if req.Tags != "" {
		tags := strings.Split(req.Tags, ",")
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				query = query.Where("customers.tags ILIKE ?", "%"+tag+"%")
			}
		}
	}

	// 搜索条件
	if req.Search != "" {
		searchTerm := "%" + req.Search + "%"
		query = query.Where("users.name ILIKE ? OR users.email ILIKE ? OR users.username ILIKE ? OR customers.company ILIKE ?",
			searchTerm, searchTerm, searchTerm, searchTerm)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count customers: %w", err)
	}

	// 排序
	orderBy := fmt.Sprintf("users.%s %s", req.SortBy, req.SortOrder)
	query = query.Order(orderBy)

	// 分页
	offset := (req.Page - 1) * req.PageSize
	query = query.Offset(offset).Limit(req.PageSize)

	// 获取数据
	var customers []CustomerInfo
	if err := query.Scan(&customers).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list customers: %w", err)
	}

	return customers, total, nil
}

// GetCustomerActivity 获取客户活动记录
func (s *CustomerService) GetCustomerActivity(ctx context.Context, customerID uint, limit int) (*CustomerActivity, error) {
	activity := &CustomerActivity{
		CustomerID: customerID,
	}

	// 获取最近的会话
	s.db.Where("user_id = ?", customerID).
		Order("created_at DESC").
		Limit(limit).
		Find(&activity.RecentSessions)

	// 获取最近的工单
	s.db.Where("customer_id = ?", customerID).
		Order("created_at DESC").
		Limit(limit).
		Find(&activity.RecentTickets)

	// 获取最近的消息
	s.db.Joins("JOIN sessions ON messages.session_id = sessions.id").
		Where("sessions.user_id = ?", customerID).
		Order("messages.created_at DESC").
		Limit(limit).
		Find(&activity.RecentMessages)

	return activity, nil
}

// AddCustomerNote 添加客户备注
func (s *CustomerService) AddCustomerNote(ctx context.Context, customerID uint, note string, userID uint) error {
	// 获取现有备注
	var customer models.Customer
	if err := s.db.Where("user_id = ?", customerID).First(&customer).Error; err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	// 添加新备注（时间戳 + 用户 + 内容）
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	newNote := fmt.Sprintf("[%s] 用户%d: %s", timestamp, userID, note)

	var updatedNotes string
	if customer.Notes == "" {
		updatedNotes = newNote
	} else {
		updatedNotes = customer.Notes + "\n" + newNote
	}

	// 更新数据库
	if err := s.db.Model(&models.Customer{}).
		Where("user_id = ?", customerID).
		Update("notes", updatedNotes).Error; err != nil {
		return fmt.Errorf("failed to add note: %w", err)
	}

	s.logger.Infof("Added note to customer %d by user %d", customerID, userID)

	return nil
}

// UpdateCustomerTags 更新客户标签
func (s *CustomerService) UpdateCustomerTags(ctx context.Context, customerID uint, tags []string) error {
	tagsStr := strings.Join(tags, ",")

	if err := s.db.Model(&models.Customer{}).
		Where("user_id = ?", customerID).
		Update("tags", tagsStr).Error; err != nil {
		return fmt.Errorf("failed to update tags: %w", err)
	}

	s.logger.Infof("Updated tags for customer %d: %s", customerID, tagsStr)

	return nil
}

// GetCustomerStats 获取客户统计信息
func (s *CustomerService) GetCustomerStats(ctx context.Context) (*CustomerStats, error) {
	stats := &CustomerStats{}

	// 总客户数
	s.db.Model(&models.User{}).Where("role = ?", "customer").Count(&stats.Total)

	// 活跃客户数（最近30天有活动）
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	s.db.Model(&models.User{}).
		Where("role = ? AND last_login > ?", "customer", thirtyDaysAgo).
		Count(&stats.Active)

	// 新客户数（最近7天注册）
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	s.db.Model(&models.User{}).
		Where("role = ? AND created_at > ?", "customer", sevenDaysAgo).
		Count(&stats.NewThisWeek)

	// 按来源统计
	s.db.Model(&models.Customer{}).
		Select("source, COUNT(*) as count").
		Group("source").
		Scan(&stats.BySource)

	// 按行业统计
	s.db.Model(&models.Customer{}).
		Select("industry, COUNT(*) as count").
		Group("industry").
		Having("industry != ''").
		Scan(&stats.ByIndustry)

	// 按优先级统计
	s.db.Model(&models.Customer{}).
		Select("priority, COUNT(*) as count").
		Group("priority").
		Scan(&stats.ByPriority)

	return stats, nil
}

// CustomerInfo 客户信息（用于列表显示）
type CustomerInfo struct {
	models.User
	Company  string `json:"company"`
	Industry string `json:"industry"`
	Source   string `json:"source"`
	Tags     string `json:"tags"`
	Notes    string `json:"notes"`
	Priority string `json:"priority"`
}

// CustomerActivity 客户活动记录
type CustomerActivity struct {
	CustomerID     uint                `json:"customer_id"`
	RecentSessions []models.Session    `json:"recent_sessions"`
	RecentTickets  []models.Ticket     `json:"recent_tickets"`
	RecentMessages []models.Message    `json:"recent_messages"`
}

// CustomerStats 客户统计信息
type CustomerStats struct {
	Total       int64                   `json:"total"`
	Active      int64                   `json:"active"`
	NewThisWeek int64                   `json:"new_this_week"`
	BySource    []SourceCount           `json:"by_source"`
	ByIndustry  []IndustryCount         `json:"by_industry"`
	ByPriority  []PriorityCount         `json:"by_priority"`
}

type SourceCount struct {
	Source string `json:"source"`
	Count  int64  `json:"count"`
}

type IndustryCount struct {
	Industry string `json:"industry"`
	Count    int64  `json:"count"`
}
