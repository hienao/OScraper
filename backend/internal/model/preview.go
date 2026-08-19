package model

import "time"

type ScrapePreview struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TargetID    uint      `gorm:"index;not null" json:"target_id"`
	CandidateID uint      `gorm:"index;not null" json:"candidate_id"`
	ActorID     uint      `gorm:"index;not null" json:"actor_id"`
	TMDBID      int       `gorm:"index;not null" json:"tmdb_id"`
	MediaType   string    `gorm:"size:20;not null" json:"media_type"`
	Fingerprint string    `gorm:"size:80;not null" json:"fingerprint"`
	MatchJSON   string    `gorm:"type:text;not null" json:"-"`
	PlanJSON    string    `gorm:"type:text;not null" json:"-"`
	ExpiresAt   time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (ScrapePreview) TableName() string { return "scrape_previews" }
