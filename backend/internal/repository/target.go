package repository

import (
	"oscraper/internal/model"

	"gorm.io/gorm"
)

type TargetRepository struct{ db *gorm.DB }

func NewTargetRepository(db *gorm.DB) *TargetRepository { return &TargetRepository{db: db} }

func (r *TargetRepository) List() ([]model.ScrapeTarget, error) {
	var targets []model.ScrapeTarget
	err := r.db.Order("created_at DESC").Find(&targets).Error
	return targets, err
}

func (r *TargetRepository) Find(id uint) (*model.ScrapeTarget, error) {
	var target model.ScrapeTarget
	if err := r.db.First(&target, id).Error; err != nil {
		return nil, err
	}
	return &target, nil
}

func (r *TargetRepository) Create(target *model.ScrapeTarget) error { return r.db.Create(target).Error }
func (r *TargetRepository) Update(target *model.ScrapeTarget) error { return r.db.Save(target).Error }
func (r *TargetRepository) DeleteWithCatalog(target *model.ScrapeTarget) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("target_id = ?", target.ID).Delete(&model.ScrapePreview{}).Error; err != nil {
			return err
		}
		if err := tx.Where("target_id = ?", target.ID).Delete(&model.MediaCandidate{}).Error; err != nil {
			return err
		}
		if err := tx.Where("target_id = ?", target.ID).Delete(&model.ScanRun{}).Error; err != nil {
			return err
		}
		return tx.Delete(target).Error
	})
}

func (r *TargetRepository) CountByConnection(connectionID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.ScrapeTarget{}).Where("connection_id = ?", connectionID).Count(&count).Error
	return count, err
}
