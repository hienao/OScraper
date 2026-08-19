package model

import (
	"time"

	"gorm.io/gorm"
)

type OpenListConnection struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	Name           string         `gorm:"size:100;not null" json:"name"`
	BaseURL        string         `gorm:"size:500;not null" json:"base_url"`
	EncryptedToken string         `gorm:"type:text;not null" json:"-"`
	Username       string         `gorm:"size:100" json:"username"`
	BasePath       string         `gorm:"size:1000;not null;default:/" json:"base_path"`
	QPSLimit       int            `gorm:"not null;default:5" json:"qps_limit"`
	QPMLimit       int            `gorm:"not null;default:120" json:"qpm_limit"`
	Enabled        bool           `gorm:"not null;default:true" json:"enabled"`
	LastTestedAt   *time.Time     `json:"last_tested_at,omitempty"`
	LastTestOK     bool           `gorm:"not null;default:false" json:"last_test_ok"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (OpenListConnection) TableName() string { return "openlist_connections" }
