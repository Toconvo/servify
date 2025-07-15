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
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// 会话模型
type Session struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Status    string    `gorm:"default:'active'" json:"status"` // active, ended, transferred
	Platform  string    `json:"platform"`                      // web, telegram, wechat, etc.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	User     User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Messages []Message `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
}

// 消息模型
type Message struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SessionID string    `gorm:"index" json:"session_id"`
	Content   string    `gorm:"type:text" json:"content"`
	Type      string    `json:"type"`      // text, image, file, system
	Sender    string    `json:"sender"`    // user, ai, agent
	CreatedAt time.Time `json:"created_at"`
	
	Session Session `gorm:"foreignKey:SessionID" json:"session,omitempty"`
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