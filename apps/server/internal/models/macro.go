package models

import "time"

// Macro 快捷回复/宏定义
// 可用于快捷插入到工单评论或设置字段
// Variables 可在前端解析（如 {{customer.name}} 等）
type Macro struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"unique;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Language    string    `gorm:"default:'zh'" json:"language"`
	Content     string    `gorm:"type:text" json:"content"`
	Active      bool      `gorm:"default:true" json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
