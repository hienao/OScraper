package maintenance

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"oscraper/internal/logging"
	"oscraper/internal/model"

	"gorm.io/gorm"
)

const cleanupInterval = 6 * time.Hour

type CleanupStats struct {
	Jobs       int64 `json:"jobs"`
	Operations int64 `json:"operations"`
	Previews   int64 `json:"previews"`
	Candidates int64 `json:"candidates"`
	Scans      int64 `json:"scans"`
}

type Status struct {
	Running     bool         `json:"running"`
	LastRunAt   *time.Time   `json:"last_run_at,omitempty"`
	LastError   string       `json:"last_error,omitempty"`
	LastCleanup CleanupStats `json:"last_cleanup"`
}

type Service struct {
	db                *gorm.DB
	dataRetentionDays int
	jobRetentionDays  atomic.Int32
	cancel            context.CancelFunc
	wait              sync.WaitGroup
	runMu             sync.Mutex
	mu                sync.RWMutex
	status            Status
}

func New(db *gorm.DB, dataRetentionDays int, jobRetentionDays ...int) *Service {
	if dataRetentionDays < 1 {
		dataRetentionDays = 30
	}
	jobDays := 7
	if len(jobRetentionDays) > 0 && jobRetentionDays[0] >= 1 && jobRetentionDays[0] <= 30 {
		jobDays = jobRetentionDays[0]
	}
	service := &Service{db: db, dataRetentionDays: dataRetentionDays}
	service.jobRetentionDays.Store(int32(jobDays))
	return service
}

func (s *Service) JobRetentionDays() int { return int(s.jobRetentionDays.Load()) }

func (s *Service) SetJobRetentionDays(days int) {
	if days >= 1 && days <= 30 {
		s.jobRetentionDays.Store(int32(days))
	}
}

func (s *Service) Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	if _, err := s.Run(ctx); err != nil {
		cancel()
		return err
	}
	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if stats, err := s.Run(ctx); err != nil {
					logging.Error("maintenance", "data cleanup failed", logging.Fields{"error": err})
				} else {
					logging.Info("maintenance", "data cleanup completed", logging.Fields{"jobs": stats.Jobs, "previews": stats.Previews, "candidates": stats.Candidates, "scans": stats.Scans})
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (s *Service) Shutdown(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	done := make(chan struct{})
	go func() { s.wait.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *Service) Run(ctx context.Context) (CleanupStats, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	s.mu.Lock()
	s.status.Running = true
	s.mu.Unlock()
	now := time.Now().UTC()
	jobCutoff := now.AddDate(0, 0, -s.JobRetentionDays())
	dataCutoff := now.AddDate(0, 0, -s.dataRetentionDays)
	stats := CleanupStats{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		terminalJobs := tx.Model(&model.ScrapeJob{}).Select("id").Where("status IN ? AND completed_at < ?", []string{"succeeded", "failed", "canceled"}, jobCutoff)
		result := tx.Where("job_id IN (?)", terminalJobs).Delete(&model.ScrapeJobOperation{})
		if result.Error != nil {
			return result.Error
		}
		stats.Operations = result.RowsAffected
		result = tx.Where("status IN ? AND completed_at < ?", []string{"succeeded", "failed", "canceled"}, jobCutoff).Delete(&model.ScrapeJob{})
		if result.Error != nil {
			return result.Error
		}
		stats.Jobs = result.RowsAffected

		result = tx.Where("expires_at < ? AND NOT EXISTS (SELECT 1 FROM scrape_jobs WHERE scrape_jobs.preview_id = scrape_previews.id)", time.Now().UTC()).Delete(&model.ScrapePreview{})
		if result.Error != nil {
			return result.Error
		}
		stats.Previews = result.RowsAffected

		result = tx.Where("created_at < ? AND NOT EXISTS (SELECT 1 FROM scrape_previews WHERE scrape_previews.candidate_id = media_candidates.id) AND NOT EXISTS (SELECT 1 FROM scrape_jobs WHERE scrape_jobs.candidate_id = media_candidates.id)", dataCutoff).Delete(&model.MediaCandidate{})
		if result.Error != nil {
			return result.Error
		}
		stats.Candidates = result.RowsAffected

		result = tx.Where("status IN ? AND completed_at < ? AND NOT EXISTS (SELECT 1 FROM media_candidates WHERE media_candidates.scan_id = scan_runs.id)", []string{"succeeded", "failed"}, dataCutoff).Delete(&model.ScanRun{})
		if result.Error != nil {
			return result.Error
		}
		stats.Scans = result.RowsAffected
		return nil
	})
	now = time.Now().UTC()
	s.mu.Lock()
	s.status.Running = false
	s.status.LastRunAt = &now
	s.status.LastCleanup = stats
	if err != nil {
		s.status.LastError = err.Error()
	} else {
		s.status.LastError = ""
	}
	s.mu.Unlock()
	return stats, err
}
