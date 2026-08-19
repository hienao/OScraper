package model

import "time"

type APIRequestLog struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	RequestID     string    `gorm:"size:100;index;not null" json:"request_id"`
	OccurredAt    time.Time `gorm:"index;not null" json:"occurred_at"`
	Method        string    `gorm:"size:12;index;not null" json:"method"`
	Route         string    `gorm:"size:255;index;not null" json:"route"`
	StatusCode    int       `gorm:"index;not null" json:"status_code"`
	LatencyMs     int64     `json:"latency_ms"`
	RequestBytes  int64     `json:"request_bytes"`
	ResponseBytes int64     `json:"response_bytes"`
	ClientIP      string    `gorm:"size:64" json:"client_ip"`
	UserAgent     string    `gorm:"size:500" json:"user_agent"`
	UserID        uint      `gorm:"index" json:"user_id,omitempty"`
	Username      string    `gorm:"size:100" json:"username,omitempty"`
	ConnectionID  uint      `gorm:"index" json:"connection_id,omitempty"`
	TargetID      uint      `gorm:"index" json:"target_id,omitempty"`
	JobID         uint      `gorm:"index" json:"job_id,omitempty"`
	ErrorCode     string    `gorm:"size:100;index" json:"error_code,omitempty"`
	ErrorMessage  string    `gorm:"size:1000" json:"error_message,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

func (APIRequestLog) TableName() string { return "api_request_logs" }

type ApplicationLog struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	OccurredAt   time.Time `gorm:"index;not null" json:"occurred_at"`
	Level        string    `gorm:"size:10;index;not null" json:"level"`
	Source       string    `gorm:"size:100;index;not null" json:"source"`
	Message      string    `gorm:"type:text;not null" json:"message"`
	Fields       string    `gorm:"type:text" json:"fields,omitempty"`
	RequestID    string    `gorm:"size:100;index" json:"request_id,omitempty"`
	UserID       uint      `gorm:"index" json:"user_id,omitempty"`
	ConnectionID uint      `gorm:"index" json:"connection_id,omitempty"`
	TargetID     uint      `gorm:"index" json:"target_id,omitempty"`
	JobID        uint      `gorm:"index" json:"job_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func (ApplicationLog) TableName() string { return "application_logs" }
