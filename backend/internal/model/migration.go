package model

import "time"

type SchemaMigration struct {
	Version   int       `gorm:"primaryKey" json:"version"`
	Name      string    `gorm:"size:200;not null" json:"name"`
	AppliedAt time.Time `gorm:"not null" json:"applied_at"`
}

func (SchemaMigration) TableName() string { return "schema_migrations" }
