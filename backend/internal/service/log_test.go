package service

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"oscraper/config"
	"oscraper/internal/logging"
	"oscraper/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newLogTestService(t *testing.T) (*LogService, *logging.Manager, *gorm.DB) {
	t.Helper()
	businessDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:logs-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := businessDB.AutoMigrate(&model.SystemSetting{}, &model.AdminAuditLog{}); err != nil {
		t.Fatal(err)
	}
	manager, err := logging.NewManager(&config.Config{APILogPath: filepath.Join(t.TempDir(), "logs.db"), APILogQueueSize: 100, APILogBatchSize: 10, LogRetentionDays: 7})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Close)
	return NewLogService(manager, businessDB, 7), manager, businessDB
}

func TestLogServiceUsesDefaultAndCleansAllExpiredLogTypes(t *testing.T) {
	service, manager, businessDB := newLogTestService(t)
	settings, err := service.Settings()
	if err != nil || settings.RetentionDays != 7 {
		t.Fatalf("unexpected default settings: %#v %v", settings, err)
	}
	old := time.Now().UTC().AddDate(0, 0, -8)
	recent := time.Now().UTC().Add(-time.Hour)
	if err := manager.DB.Create(&[]model.APIRequestLog{
		{RequestID: "old", OccurredAt: old, Method: "GET", Route: "/old", StatusCode: 200},
		{RequestID: "recent", OccurredAt: recent, Method: "GET", Route: "/recent", StatusCode: 200},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := manager.DB.Create(&[]model.ApplicationLog{
		{OccurredAt: old, Level: "info", Source: "test", Message: "old"},
		{OccurredAt: recent, Level: "info", Source: "test", Message: "recent"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := businessDB.Create(&[]model.AdminAuditLog{
		{ActorID: 1, Action: "old", Target: "test", OccurredAt: old},
		{ActorID: 1, Action: "recent", Target: "test", OccurredAt: recent},
	}).Error; err != nil {
		t.Fatal(err)
	}
	stats, err := service.Cleanup(context.Background())
	if err != nil || stats.API != 1 || stats.Application != 1 || stats.Audit != 1 {
		t.Fatalf("unexpected cleanup: %#v %v", stats, err)
	}
}

func TestLogServiceSavesRetentionAndClearsSelectedOrAllLogs(t *testing.T) {
	service, manager, businessDB := newLogTestService(t)
	if _, err := service.SaveSettings(context.Background(), 7, 0); err == nil {
		t.Fatal("expected invalid retention days to fail")
	}
	settings, err := service.SaveSettings(context.Background(), 7, 3)
	if err != nil || settings.RetentionDays != 3 {
		t.Fatalf("unexpected saved settings: %#v %v", settings, err)
	}
	reloadedSettings, err := NewLogService(manager, businessDB, 7).Settings()
	if err != nil || reloadedSettings.RetentionDays != 3 {
		t.Fatalf("retention setting was not persisted: %#v %v", reloadedSettings, err)
	}
	if err := manager.DB.Create(&model.APIRequestLog{RequestID: "api", OccurredAt: time.Now().UTC(), Method: "GET", Route: "/api", StatusCode: 200}).Error; err != nil {
		t.Fatal(err)
	}
	if err := manager.DB.Create(&model.ApplicationLog{OccurredAt: time.Now().UTC(), Level: "info", Source: "test", Message: "application"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := businessDB.Create(&model.AdminAuditLog{ActorID: 1, Action: "manual", Target: "test", OccurredAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	stats, err := service.Clear(context.Background(), 7, "application")
	if err != nil || stats.Application != 1 || stats.API != 0 || stats.Audit != 0 {
		t.Fatalf("unexpected selected clear: %#v %v", stats, err)
	}
	stats, err = service.Clear(context.Background(), 7, "all")
	if err != nil || stats.API != 1 || stats.Application != 0 || stats.Audit != 3 {
		t.Fatalf("unexpected all clear: %#v %v", stats, err)
	}
	var apiCount, applicationCount, auditCount int64
	_ = manager.DB.Model(&model.APIRequestLog{}).Count(&apiCount).Error
	_ = manager.DB.Model(&model.ApplicationLog{}).Count(&applicationCount).Error
	_ = businessDB.Model(&model.AdminAuditLog{}).Count(&auditCount).Error
	if apiCount != 0 || applicationCount != 0 || auditCount != 1 {
		t.Fatalf("unexpected post-clear counts: api=%d application=%d audit=%d", apiCount, applicationCount, auditCount)
	}
}
