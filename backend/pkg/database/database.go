package database

import (
	"errors"
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
	if c, err := db.DB(); err == nil && c != nil && cfg.DBDriver == "sqlite" {
		c.SetMaxOpenConns(4)
		_, _ = c.Exec("PRAGMA journal_mode=WAL")
		_, _ = c.Exec("PRAGMA busy_timeout=5000")
	}
	if err := applyMigrations(db); err != nil {
		return nil, err
	}
	return db, nil
}

func applyMigrations(db *gorm.DB) error {
	if err := db.AutoMigrate(&model.SchemaMigration{}); err != nil {
		return err
	}
	var migration model.SchemaMigration
	err := db.First(&migration, 1).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(
			&model.User{}, &model.OpenListConnection{}, &model.ScrapeTarget{},
			&model.ScanRun{}, &model.MediaCandidate{}, &model.ScrapePreview{},
			&model.ScrapeJob{}, &model.ScrapeJobOperation{},
			&model.SystemSetting{}, &model.AdminAuditLog{},
		); err != nil {
			return err
		}
		return tx.Create(&model.SchemaMigration{Version: 1, Name: "initial_public_schema"}).Error
	})
}
