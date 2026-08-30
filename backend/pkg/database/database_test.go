package database

import (
	"path/filepath"
	"testing"
	"time"

	"oscraper/config"
	"oscraper/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type legacyScrapeTarget struct {
	ID            uint `gorm:"primaryKey"`
	ConnectionID  uint `gorm:"index;not null"`
	Name          string
	RootPath      string
	LibraryType   string
	RenameEnabled bool
	Enabled       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (legacyScrapeTarget) TableName() string { return "scrape_targets" }

type legacyScrapeJob struct {
	ID           uint `gorm:"primaryKey"`
	TargetID     uint
	ConnectionID uint `gorm:"not null"`
}

func (legacyScrapeJob) TableName() string { return "scrape_jobs" }

func TestOpenAppliesInitialMigrationIdempotently(t *testing.T) {
	cfg := &config.Config{SQLitePath: filepath.Join(t.TempDir(), "app.db")}
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
		if migrations != 4 {
			t.Fatalf("expected four applied migrations, got %d", migrations)
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

func TestLocalSourceMigrationUpgradesExistingOpenListTargets(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SchemaMigration{}, &legacyScrapeTarget{}, &legacyScrapeJob{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SchemaMigration{Version: 1, Name: "initial_public_schema"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legacyScrapeTarget{ConnectionID: 7, Name: "Movies", RootPath: "/movies", LibraryType: "movie", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(db); err != nil {
		t.Fatal(err)
	}
	var target model.ScrapeTarget
	if err := db.First(&target).Error; err != nil {
		t.Fatal(err)
	}
	if target.SourceType != "openlist" || target.ConnectionID == nil || *target.ConnectionID != 7 {
		t.Fatalf("legacy target was not upgraded safely: %#v", target)
	}
	var migrations int64
	if err := db.Model(&model.SchemaMigration{}).Count(&migrations).Error; err != nil {
		t.Fatal(err)
	}
	if migrations != 4 {
		t.Fatalf("expected all migrations after upgrade, got %d", migrations)
	}
}
