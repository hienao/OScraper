package model

import "time"

type ScrapeBatchRun struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	TargetID       uint       `gorm:"index;not null" json:"target_id"`
	ActorID        uint       `gorm:"index;not null;default:0" json:"actor_id"`
	ScanID         uint       `gorm:"not null" json:"scan_id"`
	Status         string     `gorm:"size:20;index;not null" json:"status"`
	TotalCount     int        `gorm:"not null;default:0" json:"total_count"`
	SubmittedCount int        `gorm:"not null;default:0" json:"submitted_count"`
	SkippedCount   int        `gorm:"not null;default:0" json:"skipped_count"`
	FailedCount    int        `gorm:"not null;default:0" json:"failed_count"`
	IncludeScraped bool       `gorm:"not null;default:false" json:"include_scraped"`
	ErrorCode      string     `gorm:"size:100" json:"error_code,omitempty"`
	ErrorMessage   string     `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt      *time.Time `gorm:"index" json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (ScrapeBatchRun) TableName() string { return "scrape_batch_runs" }

type ScrapeBatchItem struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	BatchID     uint      `gorm:"index;not null" json:"batch_id"`
	CandidateID uint      `gorm:"index;not null" json:"candidate_id"`
	Path        string    `gorm:"size:1000;not null" json:"path"`
	TMDBID      *int      `json:"tmdb_id,omitempty"`
	JobID       *uint     `json:"job_id,omitempty"`
	Status      string    `gorm:"size:20;index;not null" json:"status"`
	SkipReason  string    `gorm:"size:40" json:"skip_reason,omitempty"`
	Detail      string    `gorm:"type:text" json:"detail,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (ScrapeBatchItem) TableName() string { return "scrape_batch_items" }
