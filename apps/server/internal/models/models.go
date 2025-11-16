package models

import (
	"time"
	"gorm.io/gorm"
)

// 用户模型
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"unique;not null" json:"username"`
	Email     string         `gorm:"unique;not null" json:"email"`
	Name      string         `json:"name"`
	Phone     string         `json:"phone"`
	Avatar    string         `json:"avatar"`
	Role      string         `gorm:"default:'customer'" json:"role"` // customer, agent, admin
	Status    string         `gorm:"default:'active'" json:"status"` // active, inactive, banned
	LastLogin *time.Time     `json:"last_login"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联关系
	Sessions []Session `gorm:"foreignKey:UserID" json:"sessions,omitempty"`
	Tickets  []Ticket  `gorm:"foreignKey:CustomerID" json:"tickets,omitempty"`
}

// 客户信息扩展
type Customer struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"index" json:"user_id"`
	Company     string         `json:"company"`
	Industry    string         `json:"industry"`
	Source      string         `json:"source"`      // web, referral, marketing
	Tags        string         `json:"tags"`        // 标签，逗号分隔
	Notes       string         `gorm:"type:text" json:"notes"`
	Priority    string         `gorm:"default:'normal'" json:"priority"` // low, normal, high, urgent
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// 客服代理
type Agent struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	UserID          uint           `gorm:"index" json:"user_id"`
	Department      string         `json:"department"`
	Skills          string         `json:"skills"`          // 技能标签，逗号分隔
	Status          string         `gorm:"default:'offline'" json:"status"` // online, offline, busy
	MaxConcurrent   int            `gorm:"default:5" json:"max_concurrent"`  // 最大并发工单数
	CurrentLoad     int            `gorm:"default:0" json:"current_load"`    // 当前工单数
	Rating          float64        `gorm:"default:5.0" json:"rating"`        // 评分
	TotalTickets    int            `gorm:"default:0" json:"total_tickets"`   // 总处理工单数
	AvgResponseTime int            `gorm:"default:0" json:"avg_response_time"` // 平均响应时间(秒)
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	User    User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Tickets []Ticket `gorm:"foreignKey:AgentID" json:"tickets,omitempty"`
}

// 工单模型
type Ticket struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Title        string         `gorm:"not null" json:"title"`
	Description  string         `gorm:"type:text" json:"description"`
	CustomerID   uint           `gorm:"index" json:"customer_id"`
	AgentID      *uint          `gorm:"index" json:"agent_id"`
	SessionID    *string        `gorm:"index" json:"session_id"`
	Category     string         `json:"category"`   // technical, billing, general, complaint
	Priority     string         `gorm:"default:'normal'" json:"priority"` // low, normal, high, urgent
	Status       string         `gorm:"default:'open'" json:"status"`     // open, assigned, in_progress, resolved, closed
	Source       string         `json:"source"`     // web, email, phone, chat
	Tags         string         `json:"tags"`       // 标签，逗号分隔
	DueDate      *time.Time     `json:"due_date"`
	ResolvedAt   *time.Time     `json:"resolved_at"`
	ClosedAt     *time.Time     `json:"closed_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联关系
	Customer      User            `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	Agent         *User           `gorm:"foreignKey:AgentID" json:"agent,omitempty"`
	Session       *Session        `gorm:"foreignKey:SessionID" json:"session,omitempty"`
	Comments      []TicketComment `gorm:"foreignKey:TicketID" json:"comments,omitempty"`
	Attachments   []TicketFile    `gorm:"foreignKey:TicketID" json:"attachments,omitempty"`
	StatusHistory []TicketStatus  `gorm:"foreignKey:TicketID" json:"status_history,omitempty"`
}

// 工单评论
type TicketComment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TicketID  uint      `gorm:"index" json:"ticket_id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	Type      string    `gorm:"default:'comment'" json:"type"` // comment, internal_note, system
	CreatedAt time.Time `json:"created_at"`

	Ticket Ticket `gorm:"foreignKey:TicketID" json:"ticket,omitempty"`
	User   User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// 工单附件
type TicketFile struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TicketID  uint      `gorm:"index" json:"ticket_id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	FileName  string    `gorm:"not null" json:"file_name"`
	FilePath  string    `gorm:"not null" json:"file_path"`
	FileSize  int64     `json:"file_size"`
	MimeType  string    `json:"mime_type"`
	CreatedAt time.Time `json:"created_at"`

	Ticket Ticket `gorm:"foreignKey:TicketID" json:"ticket,omitempty"`
	User   User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// 工单状态历史
type TicketStatus struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TicketID  uint      `gorm:"index" json:"ticket_id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	FromStatus string   `json:"from_status"`
	ToStatus  string    `json:"to_status"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`

	Ticket Ticket `gorm:"foreignKey:TicketID" json:"ticket,omitempty"`
	User   User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// 会话模型（更新）
type Session struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	AgentID   *uint     `gorm:"index" json:"agent_id"`
	TicketID  *uint     `gorm:"index" json:"ticket_id"`
	Status    string    `gorm:"default:'active'" json:"status"` // active, ended, transferred
	Platform  string    `json:"platform"`                      // web, telegram, wechat, etc.
	StartedAt time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	User     User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Agent    *User     `gorm:"foreignKey:AgentID" json:"agent,omitempty"`
	Ticket   *Ticket   `gorm:"foreignKey:TicketID" json:"ticket,omitempty"`
	Messages []Message `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
}

// 消息模型（更新）
type Message struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SessionID string    `gorm:"index" json:"session_id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Content   string    `gorm:"type:text" json:"content"`
	Type      string    `json:"type"`      // text, image, file, system
	Sender    string    `json:"sender"`    // user, ai, agent
	CreatedAt time.Time `json:"created_at"`

	Session Session `gorm:"foreignKey:SessionID" json:"session,omitempty"`
	User    User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// 知识库文档
type KnowledgeDoc struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Title     string    `json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	Category  string    `json:"category"`
	Tags      string    `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebRTC 连接信息
type WebRTCConnection struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	SessionID     string    `gorm:"index" json:"session_id"`
	Status        string    `gorm:"default:'connecting'" json:"status"` // connecting, connected, disconnected
	ConnectionType string   `json:"connection_type"`                    // data, video, screen
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SLA 配置
type SLAConfig struct {
	ID                 uint    `gorm:"primaryKey" json:"id"`
	Name               string  `gorm:"unique;not null" json:"name"`
	Priority           string  `gorm:"not null" json:"priority"`           // low, normal, high, urgent
	FirstResponseTime  int     `gorm:"not null" json:"first_response_time"` // 分钟
	ResolutionTime     int     `gorm:"not null" json:"resolution_time"`     // 分钟
	EscalationTime     int     `gorm:"not null" json:"escalation_time"`     // 分钟
	BusinessHoursOnly  bool    `gorm:"default:false" json:"business_hours_only"`
	Active             bool    `gorm:"default:true" json:"active"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// SLA 违约记录
type SLAViolation struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	TicketID       uint      `gorm:"index" json:"ticket_id"`
	SLAConfigID    uint      `gorm:"index" json:"sla_config_id"`
	ViolationType  string    `gorm:"not null" json:"violation_type"` // first_response, resolution, escalation
	ExpectedTime   time.Time `json:"expected_time"`
	ActualTime     *time.Time `json:"actual_time"`
	ViolationTime  int       `json:"violation_time"` // 违约时间（分钟）
	Resolved       bool      `gorm:"default:false" json:"resolved"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	Ticket    Ticket    `gorm:"foreignKey:TicketID" json:"ticket,omitempty"`
	SLAConfig SLAConfig `gorm:"foreignKey:SLAConfigID" json:"sla_config,omitempty"`
}

// 客户满意度评价
type CustomerSatisfaction struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TicketID  uint      `gorm:"index" json:"ticket_id"`
	CustomerID uint     `gorm:"index" json:"customer_id"`
	AgentID   *uint     `gorm:"index" json:"agent_id"`
	Rating    int       `gorm:"not null;check:rating >= 1 AND rating <= 5" json:"rating"` // 1-5星
	Comment   string    `gorm:"type:text" json:"comment"`
	Category  string    `json:"category"` // service_quality, response_time, resolution_quality, overall
	CreatedAt time.Time `json:"created_at"`

	Ticket   Ticket    `gorm:"foreignKey:TicketID" json:"ticket,omitempty"`
	Customer Customer  `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	Agent    *User     `gorm:"foreignKey:AgentID" json:"agent,omitempty"`
}

// 班次管理
type ShiftSchedule struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   uint      `gorm:"index" json:"agent_id"`
	ShiftType string    `gorm:"not null" json:"shift_type"` // morning, afternoon, evening, night
	StartTime time.Time `gorm:"not null" json:"start_time"`
	EndTime   time.Time `gorm:"not null" json:"end_time"`
	Date      time.Time `gorm:"index" json:"date"`
	Status    string    `gorm:"default:'scheduled'" json:"status"` // scheduled, active, completed, cancelled
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Agent User `gorm:"foreignKey:AgentID" json:"agent,omitempty"`
}

// 统计表
type DailyStats struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Date             time.Time `gorm:"uniqueIndex" json:"date"`
	TotalSessions    int       `gorm:"default:0" json:"total_sessions"`
	TotalMessages    int       `gorm:"default:0" json:"total_messages"`
	TotalTickets     int       `gorm:"default:0" json:"total_tickets"`
	ResolvedTickets  int       `gorm:"default:0" json:"resolved_tickets"`
	AvgResponseTime  int       `gorm:"default:0" json:"avg_response_time"`    // 秒
	AvgResolutionTime int      `gorm:"default:0" json:"avg_resolution_time"` // 秒
	CustomerSatisfaction float64 `gorm:"default:0" json:"customer_satisfaction"` // 平均满意度
	AIUsageCount     int       `gorm:"default:0" json:"ai_usage_count"`
	WeKnoraUsageCount int      `gorm:"default:0" json:"weknora_usage_count"`
	SLAViolations    int       `gorm:"default:0" json:"sla_violations"`      // SLA违约次数
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
