package logging

import (
	"path/filepath"
	"testing"
	"time"

	"oscraper/config"
	"oscraper/internal/model"
)

func TestManagerRemovesExpiredOperationalLogs(t *testing.T) {
	cfg := &config.Config{APILogPath: filepath.Join(t.TempDir(), "logs.db"), APILogQueueSize: 100, APILogBatchSize: 10, LogRetentionDays: 7}
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().AddDate(0, 0, -8)
	recent := time.Now().UTC().Add(-time.Hour)
	if err := manager.DB.Create(&model.APIRequestLog{RequestID: "old", OccurredAt: old, Method: "GET", Route: "/old", StatusCode: 200}).Error; err != nil {
		t.Fatal(err)
	}
	if err := manager.DB.Create(&model.ApplicationLog{OccurredAt: old, Level: "info", Source: "test", Message: "old"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := manager.DB.Create(&model.APIRequestLog{RequestID: "recent", OccurredAt: recent, Method: "GET", Route: "/recent", StatusCode: 200}).Error; err != nil {
		t.Fatal(err)
	}
	manager.Close()

	manager, err = NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	var apiCount, applicationCount int64
	if err := manager.DB.Model(&model.APIRequestLog{}).Count(&apiCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := manager.DB.Model(&model.ApplicationLog{}).Count(&applicationCount).Error; err != nil {
		t.Fatal(err)
	}
	if apiCount != 1 || applicationCount != 0 {
		t.Fatalf("unexpected retained log counts: api=%d application=%d", apiCount, applicationCount)
	}
}
