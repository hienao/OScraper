package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"oscraper/internal/logging"
	"oscraper/internal/model"
	"oscraper/internal/repository"

	"gorm.io/gorm"
)

const (
	settingLogRetentionDays = "logs.retention_days"
	defaultLogRetentionDays = 7
	logCleanupInterval      = 6 * time.Hour
)

type LogSettingsResponse struct {
	RetentionDays int `json:"retention_days"`
}

type LogCleanupStats struct {
	API         int64 `json:"api"`
	Application int64 `json:"application"`
	Audit       int64 `json:"audit"`
}

type LogService struct {
	manager     *logging.Manager
	businessDB  *gorm.DB
	settings    *repository.SettingRepository
	audit       *repository.AuditRepository
	defaultDays int
	cancel      context.CancelFunc
	wait        sync.WaitGroup
	operationMu sync.Mutex
}

func NewLogService(manager *logging.Manager, businessDB *gorm.DB, defaultDays int) *LogService {
	if defaultDays < 1 || defaultDays > 30 {
		defaultDays = defaultLogRetentionDays
	}
	return &LogService{
		manager: manager, businessDB: businessDB, settings: repository.NewSettingRepository(businessDB),
		audit: repository.NewAuditRepository(businessDB), defaultDays: defaultDays,
	}
}

func (s *LogService) Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	if _, err := s.Cleanup(ctx); err != nil {
		cancel()
		return err
	}
	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
		ticker := time.NewTicker(logCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				stats, err := s.Cleanup(ctx)
				if err != nil {
					logging.Error("maintenance", "log cleanup failed", logging.Fields{"error": err})
				} else {
					logging.Info("maintenance", "log cleanup completed", logging.Fields{"api": stats.API, "application": stats.Application, "audit": stats.Audit})
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (s *LogService) Shutdown(ctx context.Context) error {
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

func (s *LogService) Settings() (*LogSettingsResponse, error) {
	days, err := s.retentionDays()
	if err != nil {
		return nil, err
	}
	return &LogSettingsResponse{RetentionDays: days}, nil
}

func (s *LogService) SaveSettings(ctx context.Context, actorID uint, retentionDays int) (*LogSettingsResponse, error) {
	if retentionDays < 1 || retentionDays > 30 {
		return nil, BadRequest("logs.invalid_retention_days", "Log retention days must be between 1 and 30")
	}
	if err := s.settings.Upsert([]model.SystemSetting{{Key: settingLogRetentionDays, Value: strconv.Itoa(retentionDays)}}); err != nil {
		return nil, Internal("logs.settings_save_failed", "Failed to save log settings", err)
	}
	stats, err := s.Cleanup(ctx)
	if err != nil {
		return nil, err
	}
	detail := fmt.Sprintf(`{"retention_days":%d,"deleted":{"api":%d,"application":%d,"audit":%d}}`, retentionDays, stats.API, stats.Application, stats.Audit)
	if err := s.audit.Record(actorID, "logs.retention.update", "logs", detail); err != nil {
		return nil, Internal("logs.audit_failed", "Failed to record log setting change", err)
	}
	return &LogSettingsResponse{RetentionDays: retentionDays}, nil
}

func (s *LogService) Cleanup(ctx context.Context) (LogCleanupStats, error) {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	days, err := s.retentionDays()
	if err != nil {
		return LogCleanupStats{}, err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	s.manager.Flush()
	return s.deleteWhere(ctx, "all", "occurred_at < ?", cutoff)
}

func (s *LogService) Clear(ctx context.Context, actorID uint, logType string) (LogCleanupStats, error) {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	if logType != "api" && logType != "application" && logType != "audit" && logType != "all" {
		return LogCleanupStats{}, BadRequest("logs.invalid_type", "Log type must be api, application, audit, or all")
	}
	s.manager.Flush()
	stats, err := s.deleteWhere(ctx, logType, "1 = 1")
	if err != nil {
		return stats, err
	}
	detail := fmt.Sprintf(`{"type":%q,"deleted":{"api":%d,"application":%d,"audit":%d}}`, logType, stats.API, stats.Application, stats.Audit)
	if err := s.audit.Record(actorID, "logs.clear", logType, detail); err != nil {
		return stats, Internal("logs.audit_failed", "Failed to record log clear action", err)
	}
	return stats, nil
}

func (s *LogService) deleteWhere(ctx context.Context, logType, query string, args ...any) (LogCleanupStats, error) {
	stats := LogCleanupStats{}
	if logType == "api" || logType == "all" {
		result := s.manager.DB.WithContext(ctx).Where(query, args...).Delete(&model.APIRequestLog{})
		if result.Error != nil {
			return stats, Internal("logs.clear_failed", "Failed to delete API logs", result.Error)
		}
		stats.API = result.RowsAffected
	}
	if logType == "application" || logType == "all" {
		result := s.manager.DB.WithContext(ctx).Where(query, args...).Delete(&model.ApplicationLog{})
		if result.Error != nil {
			return stats, Internal("logs.clear_failed", "Failed to delete application logs", result.Error)
		}
		stats.Application = result.RowsAffected
	}
	if logType == "audit" || logType == "all" {
		result := s.businessDB.WithContext(ctx).Where(query, args...).Delete(&model.AdminAuditLog{})
		if result.Error != nil {
			return stats, Internal("logs.clear_failed", "Failed to delete audit logs", result.Error)
		}
		stats.Audit = result.RowsAffected
	}
	return stats, nil
}

func (s *LogService) retentionDays() (int, error) {
	setting, err := s.settings.Get(settingLogRetentionDays)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.defaultDays, nil
	}
	if err != nil {
		return 0, Internal("logs.settings_load_failed", "Failed to load log settings", err)
	}
	days, parseErr := strconv.Atoi(setting.Value)
	if parseErr != nil || days < 1 || days > 30 {
		return 0, Internal("logs.settings_invalid", "Stored log retention setting is invalid", parseErr)
	}
	return days, nil
}
