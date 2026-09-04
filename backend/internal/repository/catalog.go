package repository

import (
	"time"

	"oscraper/internal/model"

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

func (r *CatalogRepository) ActiveScanCount(targetID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.ScanRun{}).Where("target_id = ? AND status IN ?", targetID, []string{"pending", "running"}).Count(&count).Error
	return count, err
}

func (r *CatalogRepository) ClaimScan(id uint) (bool, error) {
	startedAt := time.Now().UTC()
	result := r.db.Model(&model.ScanRun{}).Where("id = ? AND status = ?", id, "pending").Updates(map[string]interface{}{
		"status": "running", "started_at": startedAt, "error_code": "", "error_message": "",
	})
	return result.RowsAffected == 1, result.Error
}

func (r *CatalogRepository) RecoverInterruptedScans() ([]model.ScanRun, error) {
	if err := r.db.Model(&model.ScanRun{}).Where("status = ?", "running").Updates(map[string]interface{}{
		"status": "pending", "started_at": nil, "error_code": "", "error_message": "",
	}).Error; err != nil {
		return nil, err
	}
	var scans []model.ScanRun
	err := r.db.Where("status = ?", "pending").Order("id ASC").Find(&scans).Error
	return scans, err
}

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

func (r *CatalogRepository) LatestSucceededScan(targetID uint) (*model.ScanRun, error) {
	var scan model.ScanRun
	if err := r.db.Where("target_id = ? AND status = ?", targetID, "succeeded").Order("id DESC").First(&scan).Error; err != nil {
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
