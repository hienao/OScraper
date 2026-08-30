package service

import (
	"context"
	"fmt"
	"testing"

	"oscraper/internal/maintenance"
	"oscraper/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newJobRecordSettingsTestService(t *testing.T) (*JobRecordSettingsService, *maintenance.Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:job-settings-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.SystemSetting{}, &model.AdminAuditLog{}, &model.ScanRun{}, &model.MediaCandidate{},
		&model.ScrapePreview{}, &model.ScrapeJob{}, &model.ScrapeJobOperation{},
	); err != nil {
		t.Fatal(err)
	}
	maintenanceService := maintenance.New(db, 30, 7)
	return NewJobRecordSettingsService(db, maintenanceService, 7), maintenanceService, db
}

func TestJobRecordSettingsPersistAndUpdateMaintenance(t *testing.T) {
	settings, maintenanceService, db := newJobRecordSettingsTestService(t)
	value, err := settings.Settings()
	if err != nil || value.RetentionDays != 7 {
		t.Fatalf("unexpected defaults: %#v %v", value, err)
	}
	if _, err := settings.Save(context.Background(), 1, 31); err == nil {
		t.Fatal("expected invalid retention to fail")
	}
	value, err = settings.Save(context.Background(), 1, 14)
	if err != nil || value.RetentionDays != 14 || maintenanceService.JobRetentionDays() != 14 {
		t.Fatalf("unexpected saved retention: %#v days=%d err=%v", value, maintenanceService.JobRetentionDays(), err)
	}
	reloadedMaintenance := maintenance.New(db, 30, 7)
	reloaded := NewJobRecordSettingsService(db, reloadedMaintenance, 7)
	if err := reloaded.Initialize(); err != nil || reloadedMaintenance.JobRetentionDays() != 14 {
		t.Fatalf("persisted retention was not restored: days=%d err=%v", reloadedMaintenance.JobRetentionDays(), err)
	}
}
