package repository

import (
	"strings"
	"time"

	"oscraper/internal/model"

	"gorm.io/gorm"
)

type JobRepository struct{ db *gorm.DB }

func NewJobRepository(db *gorm.DB) *JobRepository { return &JobRepository{db: db} }

func (r *JobRepository) Create(job *model.ScrapeJob, operations []model.ScrapeJobOperation) error {
	return retryLocked(func() error {
		return r.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(job).Error; err != nil {
				return err
			}
			for index := range operations {
				operations[index].JobID = job.ID
			}
			if len(operations) > 0 {
				return tx.Create(&operations).Error
			}
			return nil
		})
	})
}

func (r *JobRepository) Find(id uint) (*model.ScrapeJob, error) {
	var job model.ScrapeJob
	if err := retryLocked(func() error { return r.db.First(&job, id).Error }); err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *JobRepository) FindIdempotent(actorID, previewID uint, key string) (*model.ScrapeJob, error) {
	var job model.ScrapeJob
	if err := retryLocked(func() error {
		return r.db.Where("actor_id = ? AND preview_id = ? AND idempotency_key = ?", actorID, previewID, key).First(&job).Error
	}); err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *JobRepository) ActiveByTarget(targetID uint) (int64, error) {
	var count int64
	err := retryLocked(func() error {
		return r.db.Model(&model.ScrapeJob{}).
			Where("target_id = ? AND status IN ?", targetID, []string{"pending", "running"}).
			Count(&count).Error
	})
	return count, err
}

func (r *JobRepository) ActiveByCandidate(candidateID uint) (int64, error) {
	var count int64
	err := retryLocked(func() error {
		return r.db.Model(&model.ScrapeJob{}).
			Where("candidate_id = ? AND status IN ?", candidateID, []string{"pending", "running"}).
			Count(&count).Error
	})
	return count, err
}

func (r *JobRepository) ActiveByConnection(connectionID uint) (int64, error) {
	var count int64
	err := retryLocked(func() error {
		return r.db.Model(&model.ScrapeJob{}).Where("connection_id = ? AND status IN ?", connectionID, []string{"pending", "running"}).Count(&count).Error
	})
	return count, err
}

func (r *JobRepository) Claim(id uint) (bool, error) {
	now := time.Now().UTC()
	var rows int64
	err := retryLocked(func() error {
		result := r.db.Model(&model.ScrapeJob{}).Where("id = ? AND status = ?", id, "pending").Updates(map[string]any{
			"status": "running", "stage": "preparing", "started_at": now, "completed_at": nil,
		})
		rows = result.RowsAffected
		return result.Error
	})
	return rows == 1, err
}

func (r *JobRepository) Save(job *model.ScrapeJob) error {
	return retryLocked(func() error { return r.db.Save(job).Error })
}

func (r *JobRepository) Operations(jobID uint) ([]model.ScrapeJobOperation, error) {
	var operations []model.ScrapeJobOperation
	err := retryLocked(func() error { return r.db.Where("job_id = ?", jobID).Order("sequence ASC").Find(&operations).Error })
	return operations, err
}

func (r *JobRepository) SaveOperation(operation *model.ScrapeJobOperation) error {
	return retryLocked(func() error { return r.db.Save(operation).Error })
}

func (r *JobRepository) List(status string, targetID uint, page, size int) ([]model.ScrapeJob, int64, error) {
	query := r.db.Model(&model.ScrapeJob{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if targetID > 0 {
		query = query.Where("target_id = ?", targetID)
	}
	var total int64
	if err := retryLocked(func() error { return query.Count(&total).Error }); err != nil {
		return nil, 0, err
	}
	var jobs []model.ScrapeJob
	err := retryLocked(func() error {
		return query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&jobs).Error
	})
	return jobs, total, err
}

func (r *JobRepository) CancelPending(id uint) (bool, error) {
	now := time.Now().UTC()
	var rows int64
	err := retryLocked(func() error {
		result := r.db.Model(&model.ScrapeJob{}).Where("id = ? AND status = ?", id, "pending").Updates(map[string]any{
			"status": "canceled", "stage": "completed", "message": "Canceled before execution", "completed_at": now,
		})
		rows = result.RowsAffected
		return result.Error
	})
	return rows == 1, err
}

func (r *JobRepository) ResetFailed(id uint) (bool, error) {
	returnValue := false
	err := retryLocked(func() error {
		returnValue = false
		return r.db.Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&model.ScrapeJob{}).Where("id = ? AND status = ?", id, "failed").Updates(map[string]any{
				"status": "pending", "stage": "preparing", "message": "Queued for retry", "error_code": "", "error_message": "", "completed_at": nil,
				"attempts": gorm.Expr("attempts + 1"),
			})
			if result.Error != nil || result.RowsAffected != 1 {
				return result.Error
			}
			returnValue = true
			return tx.Model(&model.ScrapeJobOperation{}).Where("job_id = ? AND status IN ?", id, []string{"running", "failed"}).Updates(map[string]any{
				"status": "pending", "last_error": "", "started_at": nil, "completed_at": nil,
			}).Error
		})
	})
	return returnValue, err
}

func (r *JobRepository) RecoverInterrupted() (int64, error) {
	now := time.Now().UTC()
	var rows int64
	err := retryLocked(func() error {
		result := r.db.Model(&model.ScrapeJob{}).Where("status IN ?", []string{"pending", "running"}).Updates(map[string]any{
			"status": "failed", "error_code": "job.interrupted", "error_message": "Application stopped while the job was running",
			"message": "Interrupted; retry from checkpoint", "completed_at": now,
		})
		rows = result.RowsAffected
		return result.Error
	})
	if err != nil {
		return 0, err
	}
	if rows > 0 {
		if err := retryLocked(func() error {
			return r.db.Model(&model.ScrapeJobOperation{}).Where("status = ?", "running").Updates(map[string]any{"status": "failed", "last_error": "Application stopped"}).Error
		}); err != nil {
			return rows, err
		}
	}
	return rows, nil
}

func retryLocked(operation func() error) error {
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		err = operation()
		if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "locked") && !strings.Contains(strings.ToLower(err.Error()), "busy")) {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	return err
}
