package logging

import (
	"path/filepath"
	"testing"
	"time"

	"oscraper/config"
	"oscraper/internal/model"
)

func TestManagerFlushPersistsQueuedOperationalLogs(t *testing.T) {
	cfg := &config.Config{APILogPath: filepath.Join(t.TempDir(), "logs.db"), APILogQueueSize: 100, APILogBatchSize: 10, LogRetentionDays: 7}
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.Submit(model.APIRequestLog{RequestID: "api", OccurredAt: time.Now().UTC(), Method: "GET", Route: "/api", StatusCode: 200})
	manager.SubmitApplication(model.ApplicationLog{OccurredAt: time.Now().UTC(), Level: "info", Source: "test", Message: "application"})
	manager.Flush()
	var apiCount, applicationCount int64
	if err := manager.DB.Model(&model.APIRequestLog{}).Count(&apiCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := manager.DB.Model(&model.ApplicationLog{}).Count(&applicationCount).Error; err != nil {
		t.Fatal(err)
	}
	if apiCount != 1 || applicationCount != 1 {
		t.Fatalf("unexpected retained log counts: api=%d application=%d", apiCount, applicationCount)
	}
}
