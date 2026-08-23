package model

import "time"

type ScanRun struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	TargetID       uint       `gorm:"index;not null" json:"target_id"`
	ActorID        uint       `gorm:"index;not null;default:0" json:"actor_id"`
	Refresh        bool       `gorm:"not null;default:false" json:"refresh"`
	Status         string     `gorm:"size:20;index;not null" json:"status"`
	CandidateCount int        `gorm:"not null;default:0" json:"candidate_count"`
	VideoCount     int        `gorm:"not null;default:0" json:"video_count"`
	ErrorCode      string     `gorm:"size:100" json:"error_code,omitempty"`
	ErrorMessage   string     `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt      *time.Time `gorm:"index" json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (ScanRun) TableName() string { return "scan_runs" }

type MediaCandidate struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	ScanID             uint      `gorm:"index;not null" json:"scan_id"`
	TargetID           uint      `gorm:"index;not null" json:"target_id"`
	Path               string    `gorm:"size:1000;not null" json:"path"`
	Kind               string    `gorm:"size:20;index;not null" json:"kind"`
	Fingerprint        string    `gorm:"size:80;not null" json:"fingerprint"`
	ManifestJSON       string    `gorm:"type:text" json:"-"`
	RepresentativeFile string    `gorm:"size:1000" json:"representative_file"`
	ParsedTitle        string    `gorm:"size:500" json:"parsed_title"`
	Year               *int      `json:"year,omitempty"`
	Season             *int      `json:"season,omitempty"`
	Episode            *int      `json:"episode,omitempty"`
	TMDBID             *int      `json:"tmdb_id,omitempty"`
	Confidence         int       `gorm:"not null;default:0" json:"confidence"`
	VideoCount         int       `gorm:"not null;default:0" json:"video_count"`
	Status             string    `gorm:"size:20;index;not null" json:"status"`
	CreatedAt          time.Time `json:"created_at"`
}

func (MediaCandidate) TableName() string { return "media_candidates" }
