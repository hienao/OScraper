package repository

import (
	"openlistscraper/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettingRepository struct{ db *gorm.DB }

func NewSettingRepository(db *gorm.DB) *SettingRepository { return &SettingRepository{db: db} }

func (r *SettingRepository) Get(key string) (*model.SystemSetting, error) {
	var setting model.SystemSetting
	if err := r.db.First(&setting, "key = ?", key).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}

func (r *SettingRepository) Upsert(settings []model.SystemSetting) error {
	if len(settings) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "is_secret", "updated_at"}),
	}).Create(&settings).Error
}

func (r *SettingRepository) Transaction(fn func(*gorm.DB) error) error { return r.db.Transaction(fn) }
