package model

import "time"

type ScrapeJob struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	TargetID       uint       `gorm:"index;not null" json:"target_id"`
	PreviewID      uint       `gorm:"index;not null" json:"preview_id"`
	CandidateID    uint       `gorm:"index;not null" json:"candidate_id"`
	SourceType     string     `gorm:"size:20;index;not null;default:openlist" json:"source_type"`
	SourceRoot     string     `gorm:"size:1000;not null;default:/" json:"source_root"`
	ConnectionID   uint       `gorm:"index" json:"connection_id,omitempty"`
	ActorID        uint       `gorm:"index;not null" json:"actor_id"`
	IdempotencyKey string     `gorm:"size:100;index" json:"-"`
	Status         string     `gorm:"size:20;index;not null" json:"status"`
	Stage          string     `gorm:"size:20;index;not null" json:"stage"`
	Progress       int        `gorm:"not null;default:0" json:"progress"`
	Message        string     `gorm:"size:500" json:"message,omitempty"`
	ErrorCode      string     `gorm:"size:100;index" json:"error_code,omitempty"`
	ErrorMessage   string     `gorm:"type:text" json:"error_message,omitempty"`
	Checkpoint     int        `gorm:"not null;default:0" json:"checkpoint"`
	Attempts       int        `gorm:"not null;default:1" json:"attempts"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CreatedAt      time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (ScrapeJob) TableName() string { return "scrape_jobs" }

type ScrapeJobOperation struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	JobID        uint       `gorm:"uniqueIndex:idx_job_sequence;not null" json:"job_id"`
	Sequence     int        `gorm:"uniqueIndex:idx_job_sequence;not null" json:"sequence"`
	Type         string     `gorm:"size:20;index;not null" json:"type"`
	SourcePath   string     `gorm:"size:2000" json:"source_path,omitempty"`
	TargetPath   string     `gorm:"size:2000;not null" json:"target_path"`
	ArtifactKind string     `gorm:"size:20" json:"artifact_kind,omitempty"`
	Artifact     int        `gorm:"not null;default:0" json:"-"`
	LocalPath    string     `gorm:"size:2000" json:"-"`
	ContentType  string     `gorm:"size:100" json:"content_type,omitempty"`
	Status       string     `gorm:"size:20;index;not null" json:"status"`
	Attempts     int        `gorm:"not null;default:0" json:"attempts"`
	LastError    string     `gorm:"type:text" json:"last_error,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (ScrapeJobOperation) TableName() string { return "scrape_job_operations" }
