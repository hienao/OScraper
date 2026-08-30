package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"oscraper/internal/maintenance"
	"oscraper/internal/model"
	"oscraper/internal/repository"

	"gorm.io/gorm"
)

const (
	settingJobRecordRetentionDays = "jobs.retention_days"
	defaultJobRecordRetentionDays = 7
)

type JobRecordSettingsResponse struct {
	RetentionDays int `json:"retention_days"`
}

type JobRecordSettingsService struct {
	settings    *repository.SettingRepository
	audit       *repository.AuditRepository
	maintenance *maintenance.Service
	defaultDays int
}

func NewJobRecordSettingsService(db *gorm.DB, maintenanceService *maintenance.Service, defaultDays int) *JobRecordSettingsService {
	if defaultDays < 1 || defaultDays > 30 {
		defaultDays = defaultJobRecordRetentionDays
	}
	return &JobRecordSettingsService{
		settings: repository.NewSettingRepository(db), audit: repository.NewAuditRepository(db),
		maintenance: maintenanceService, defaultDays: defaultDays,
	}
}

func (s *JobRecordSettingsService) Initialize() error {
	days, err := s.retentionDays()
	if err != nil {
		return err
	}
	s.maintenance.SetJobRetentionDays(days)
	return nil
}

func (s *JobRecordSettingsService) Settings() (*JobRecordSettingsResponse, error) {
	days, err := s.retentionDays()
	if err != nil {
		return nil, err
	}
	return &JobRecordSettingsResponse{RetentionDays: days}, nil
}

func (s *JobRecordSettingsService) Save(ctx context.Context, actorID uint, retentionDays int) (*JobRecordSettingsResponse, error) {
	if retentionDays < 1 || retentionDays > 30 {
		return nil, BadRequest("job.invalid_retention_days", "Job record retention days must be between 1 and 30")
	}
	if err := s.settings.Upsert([]model.SystemSetting{{Key: settingJobRecordRetentionDays, Value: strconv.Itoa(retentionDays)}}); err != nil {
		return nil, Internal("job.settings_save_failed", "Failed to save job record settings", err)
	}
	s.maintenance.SetJobRetentionDays(retentionDays)
	stats, err := s.maintenance.Run(ctx)
	if err != nil {
		return nil, Internal("job.cleanup_failed", "Failed to apply job record retention", err)
	}
	detail := fmt.Sprintf(`{"retention_days":%d,"deleted_jobs":%d,"deleted_operations":%d}`, retentionDays, stats.Jobs, stats.Operations)
	if err := s.audit.Record(actorID, "jobs.retention.update", "scrape_jobs", detail); err != nil {
		return nil, Internal("job.audit_failed", "Failed to record job retention change", err)
	}
	return &JobRecordSettingsResponse{RetentionDays: retentionDays}, nil
}

func (s *JobRecordSettingsService) retentionDays() (int, error) {
	setting, err := s.settings.Get(settingJobRecordRetentionDays)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.defaultDays, nil
	}
	if err != nil {
		return 0, Internal("job.settings_load_failed", "Failed to load job record settings", err)
	}
	days, parseErr := strconv.Atoi(setting.Value)
	if parseErr != nil || days < 1 || days > 30 {
		return 0, Internal("job.settings_invalid", "Stored job record retention setting is invalid", parseErr)
	}
	return days, nil
}
