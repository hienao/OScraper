package repository

import (
	"openlistscraper/internal/model"

	"gorm.io/gorm"
)

type CatalogRepository struct{ db *gorm.DB }

func NewCatalogRepository(db *gorm.DB) *CatalogRepository { return &CatalogRepository{db: db} }

func (r *CatalogRepository) CreateScan(scan *model.ScanRun) error { return r.db.Create(scan).Error }

func (r *CatalogRepository) CompleteScan(scan *model.ScanRun, candidates []model.MediaCandidate) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if len(candidates) > 0 {
			if err := tx.Create(&candidates).Error; err != nil {
				return err
			}
		}
		return tx.Save(scan).Error
	})
}

func (r *CatalogRepository) SaveScan(scan *model.ScanRun) error { return r.db.Save(scan).Error }

func (r *CatalogRepository) FindScan(id, targetID uint) (*model.ScanRun, error) {
	var scan model.ScanRun
	if err := r.db.Where("id = ? AND target_id = ?", id, targetID).First(&scan).Error; err != nil {
		return nil, err
	}
	return &scan, nil
}

func (r *CatalogRepository) LatestScan(targetID uint) (*model.ScanRun, error) {
	var scan model.ScanRun
	if err := r.db.Where("target_id = ?", targetID).Order("id DESC").First(&scan).Error; err != nil {
		return nil, err
	}
	return &scan, nil
}

func (r *CatalogRepository) Candidates(targetID, scanID uint) ([]model.MediaCandidate, error) {
	var candidates []model.MediaCandidate
	err := r.db.Where("target_id = ? AND scan_id = ?", targetID, scanID).Order("path ASC").Find(&candidates).Error
	return candidates, err
}

func (r *CatalogRepository) FindCandidate(id, targetID uint) (*model.MediaCandidate, error) {
	var candidate model.MediaCandidate
	if err := r.db.Where("id = ? AND target_id = ?", id, targetID).First(&candidate).Error; err != nil {
		return nil, err
	}
	return &candidate, nil
}
