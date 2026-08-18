package database

import (
	"path/filepath"
	"testing"

	"openlistscraper/config"
	"openlistscraper/internal/model"
)

func TestOpenAppliesInitialMigrationIdempotently(t *testing.T) {
	cfg := &config.Config{DBDriver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "app.db")}
	for attempt := 0; attempt < 2; attempt++ {
		db, err := Open(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if !db.Migrator().HasTable(&model.ScrapeJob{}) || !db.Migrator().HasTable(&model.ScrapeJobOperation{}) {
			t.Fatal("scrape job tables were not created")
		}
		var migrations int64
		if err := db.Model(&model.SchemaMigration{}).Count(&migrations).Error; err != nil {
			t.Fatal(err)
		}
		if migrations != 1 {
			t.Fatalf("expected one applied migration, got %d", migrations)
		}
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		if err := sqlDB.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
