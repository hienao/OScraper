package repository

import (
	"oscraper/internal/model"

	"gorm.io/gorm"
)

type PreviewRepository struct{ db *gorm.DB }

func NewPreviewRepository(db *gorm.DB) *PreviewRepository { return &PreviewRepository{db: db} }

func (r *PreviewRepository) Create(preview *model.ScrapePreview) error {
	return r.db.Create(preview).Error
}

func (r *PreviewRepository) Find(id, targetID uint) (*model.ScrapePreview, error) {
	var preview model.ScrapePreview
	if err := r.db.Where("id = ? AND target_id = ?", id, targetID).First(&preview).Error; err != nil {
		return nil, err
	}
	return &preview, nil
}
