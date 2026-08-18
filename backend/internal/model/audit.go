package model

import "time"

type AdminAuditLog struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ActorID    uint      `gorm:"index;not null" json:"actor_id"`
	Action     string    `gorm:"size:100;index;not null" json:"action"`
	Target     string    `gorm:"size:150;not null" json:"target"`
	Detail     string    `gorm:"type:text" json:"detail"`
	OccurredAt time.Time `gorm:"index;not null" json:"occurred_at"`
}

func (AdminAuditLog) TableName() string { return "admin_audit_logs" }
