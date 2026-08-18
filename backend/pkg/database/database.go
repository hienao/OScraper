package database

import (
	"fmt"
	"os"
	"path/filepath"

	"openlistscraper/config"
	"openlistscraper/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(cfg *config.Config) (*gorm.DB, error) {
	gormConfig := &gorm.Config{Logger: logger.Default.LogMode(logger.Error)}
	var dialector gorm.Dialector
	switch cfg.DBDriver {
	case "sqlite":
		if err := os.MkdirAll(filepath.Dir(cfg.SQLitePath), 0o755); err != nil {
			return nil, err
		}
		dialector = sqlite.Open(cfg.SQLitePath)
	case "postgres", "postgresql":
		if cfg.DatabaseURL == "" {
			return nil, fmt.Errorf("DATABASE_URL is required for postgres")
		}
		dialector = postgres.Open(cfg.DatabaseURL)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q", cfg.DBDriver)
	}
	db, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.OpenListConnection{}, &model.ScrapeTarget{},
		&model.ScanRun{}, &model.MediaCandidate{}, &model.ScrapePreview{},
		&model.SystemSetting{}, &model.AdminAuditLog{},
	); err != nil {
		return nil, err
	}
	return db, nil
}
