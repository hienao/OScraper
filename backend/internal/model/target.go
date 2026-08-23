package model

import (
	"time"

	"gorm.io/gorm"
)

type ScrapeTarget struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	SourceType    string         `gorm:"size:20;index;not null;default:openlist" json:"source_type"`
	ConnectionID  *uint          `gorm:"index" json:"connection_id,omitempty"`
	Name          string         `gorm:"size:100;not null" json:"name"`
	RootPath      string         `gorm:"size:1000;not null" json:"root_path"`
	LibraryType   string         `gorm:"size:20;index;not null" json:"library_type"`
	RenameEnabled bool           `gorm:"not null;default:false" json:"rename_enabled"`
	Enabled       bool           `gorm:"not null;default:true" json:"enabled"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (ScrapeTarget) TableName() string { return "scrape_targets" }
