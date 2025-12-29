package models

import "time"

// AutomationTrigger 自动化触发器定义
type AutomationTrigger struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"unique;not null" json:"name"`
	Event      string    `gorm:"not null" json:"event"`       // ticket_created, ticket_updated, sla_violation
	Conditions string    `gorm:"type:text" json:"conditions"` // JSON: [{field,op,value}]
	Actions    string    `gorm:"type:text" json:"actions"`    // JSON: [{type,params}]
	Active     bool      `gorm:"default:true" json:"active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// AutomationRun 执行记录用于审计
type AutomationRun struct {
	ID        uint              `gorm:"primaryKey" json:"id"`
	TriggerID uint              `gorm:"index" json:"trigger_id"`
	TicketID  uint              `gorm:"index" json:"ticket_id"`
	Status    string            `gorm:"index" json:"status"` // success, skipped, failed
	Message   string            `gorm:"type:text" json:"message"`
	CreatedAt time.Time         `json:"created_at"`
	Trigger   AutomationTrigger `gorm:"foreignKey:TriggerID" json:"trigger,omitempty"`
}
