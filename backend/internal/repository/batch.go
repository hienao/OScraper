package repository

import (
	"time"

	"oscraper/internal/model"

	"gorm.io/gorm"
)

type BatchRepository struct{ db *gorm.DB }

func NewBatchRepository(db *gorm.DB) *BatchRepository { return &BatchRepository{db: db} }

func (r *BatchRepository) CreateBatch(batch *model.ScrapeBatchRun, items []model.ScrapeBatchItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(batch).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for index := range items {
			items[index].BatchID = batch.ID
		}
		return tx.Create(&items).Error
	})
}

func (r *BatchRepository) FindBatch(id, targetID uint) (*model.ScrapeBatchRun, error) {
	var batch model.ScrapeBatchRun
	if err := r.db.Where("id = ? AND target_id = ?", id, targetID).First(&batch).Error; err != nil {
		return nil, err
	}
	return &batch, nil
}

func (r *BatchRepository) Items(batchID uint) ([]model.ScrapeBatchItem, error) {
	var items []model.ScrapeBatchItem
	err := r.db.Where("batch_id = ?", batchID).Order("id ASC").Find(&items).Error
	return items, err
}

func (r *BatchRepository) ActiveBatchCount(targetID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.ScrapeBatchRun{}).Where("target_id = ? AND status IN ?", targetID, []string{"pending", "running"}).Count(&count).Error
	return count, err
}

func (r *BatchRepository) ClaimBatch(id uint) (bool, error) {
	startedAt := time.Now().UTC()
	result := r.db.Model(&model.ScrapeBatchRun{}).Where("id = ? AND status = ?", id, "pending").Updates(map[string]interface{}{
		"status": "running", "started_at": startedAt, "error_code": "", "error_message": "",
	})
	return result.RowsAffected == 1, result.Error
}

// SaveItem persists one item's outcome without touching sibling rows.
func (r *BatchRepository) SaveItem(item *model.ScrapeBatchItem) error { return r.db.Save(item).Error }

// UpdateBatchProgress writes only the counters so a concurrent cancel can keep the status.
func (r *BatchRepository) UpdateBatchProgress(id uint, submitted, skipped, failed int) error {
	return r.db.Model(&model.ScrapeBatchRun{}).Where("id = ?", id).Updates(map[string]interface{}{
		"submitted_count": submitted, "skipped_count": skipped, "failed_count": failed,
	}).Error
}

// CompleteBatch finalizes the batch; the status guard keeps a canceled batch canceled.
func (r *BatchRepository) CompleteBatch(id uint, status, errorCode, errorMessage string) error {
	updates := map[string]interface{}{"status": status, "completed_at": time.Now().UTC()}
	if errorCode != "" {
		updates["error_code"] = errorCode
		updates["error_message"] = errorMessage
	}
	return r.db.Model(&model.ScrapeBatchRun{}).Where("id = ? AND status IN ?", id, []string{"pending", "running"}).Updates(updates).Error
}

func (r *BatchRepository) CancelBatch(id uint) (bool, error) {
	result := r.db.Model(&model.ScrapeBatchRun{}).Where("id = ? AND status IN ?", id, []string{"pending", "running"}).Update("status", "canceled")
	return result.RowsAffected == 1, result.Error
}

// SkipPendingItems marks unfinished items after a cancel; returns the number of affected rows.
func (r *BatchRepository) SkipPendingItems(batchID uint, reason, detail string) (int64, error) {
	result := r.db.Model(&model.ScrapeBatchItem{}).Where("batch_id = ? AND status = ?", batchID, "pending").Updates(map[string]interface{}{
		"status": "skipped", "skip_reason": reason, "detail": detail,
	})
	return result.RowsAffected, result.Error
}

func (r *BatchRepository) RecoverInterruptedBatches() ([]model.ScrapeBatchRun, error) {
	if err := r.db.Model(&model.ScrapeBatchRun{}).Where("status = ?", "running").Updates(map[string]interface{}{
		"status": "failed", "error_code": "batch.interrupted", "error_message": "The scrape batch was interrupted by a service restart",
	}).Error; err != nil {
		return nil, err
	}
	var batches []model.ScrapeBatchRun
	err := r.db.Where("status = ?", "pending").Order("id ASC").Find(&batches).Error
	return batches, err
}
