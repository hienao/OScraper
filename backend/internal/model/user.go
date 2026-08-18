package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	Username           string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	PasswordHash       string         `gorm:"size:255;not null" json:"-"`
	IsAdmin            bool           `gorm:"default:false;not null" json:"is_admin"`
	RequiresAdminSetup bool           `gorm:"default:false;not null" json:"requires_admin_setup"`
	TokenVersion       int            `gorm:"default:1;not null" json:"-"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string { return "users" }
