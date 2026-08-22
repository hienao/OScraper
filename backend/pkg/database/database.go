package database

import (
	"os"
	"path/filepath"

	"oscraper/config"
	"oscraper/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(cfg *config.Config) (*gorm.DB, error) {
	gormConfig := &gorm.Config{Logger: logger.Default.LogMode(logger.Error)}
	if err := os.MkdirAll(filepath.Dir(cfg.SQLitePath), 0o755); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(cfg.SQLitePath), gormConfig)
	if err != nil {
		return nil, err
	}
	if c, err := db.DB(); err == nil && c != nil {
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
	migrations := []struct {
		version int
		name    string
		apply   func(*gorm.DB) error
	}{
		{version: 1, name: "initial_public_schema", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&model.User{}, &model.OpenListConnection{}, &model.ScrapeTarget{},
				&model.ScanRun{}, &model.MediaCandidate{}, &model.ScrapePreview{},
				&model.ScrapeJob{}, &model.ScrapeJobOperation{},
				&model.SystemSetting{}, &model.AdminAuditLog{},
			)
		}},
		{version: 2, name: "local_media_sources", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.ScrapeTarget{}, &model.ScrapeJob{})
		}},
		{version: 3, name: "asynchronous_scan_runtime", apply: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&model.ScanRun{})
		}},
	}
	for _, migration := range migrations {
		var count int64
		if err := db.Model(&model.SchemaMigration{}).Where("version = ?", migration.version).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := migration.apply(tx); err != nil {
				return err
			}
			return tx.Create(&model.SchemaMigration{Version: migration.version, Name: migration.name}).Error
		}); err != nil {
			return err
		}
	}
	return nil
}
