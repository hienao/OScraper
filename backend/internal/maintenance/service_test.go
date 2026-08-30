package maintenance

import (
	"context"
	"testing"
	"time"

	"oscraper/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRunDeletesOnlyExpiredUnreferencedOperationalData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:maintenance?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ScanRun{}, &model.MediaCandidate{}, &model.ScrapePreview{}, &model.ScrapeJob{}, &model.ScrapeJobOperation{}, &model.AdminAuditLog{}); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().AddDate(0, 0, -45)
	recent := time.Now().UTC().Add(-time.Hour)
	oldScan := model.ScanRun{TargetID: 1, Status: "succeeded", StartedAt: &old, CompletedAt: &old, CreatedAt: old}
	recentScan := model.ScanRun{TargetID: 1, Status: "succeeded", StartedAt: &recent, CompletedAt: &recent, CreatedAt: recent}
	if err := db.Create(&oldScan).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&recentScan).Error; err != nil {
		t.Fatal(err)
	}
	oldCandidate := model.MediaCandidate{ScanID: oldScan.ID, TargetID: 1, Path: "/old", Kind: "movie", Fingerprint: "old", Status: "ready", CreatedAt: old}
	recentCandidate := model.MediaCandidate{ScanID: recentScan.ID, TargetID: 1, Path: "/recent", Kind: "movie", Fingerprint: "recent", Status: "ready", CreatedAt: recent}
	if err := db.Create(&[]model.MediaCandidate{oldCandidate, recentCandidate}).Error; err != nil {
		t.Fatal(err)
	}
	var candidates []model.MediaCandidate
	if err := db.Order("id ASC").Find(&candidates).Error; err != nil {
		t.Fatal(err)
	}
	oldPreview := model.ScrapePreview{TargetID: 1, CandidateID: candidates[0].ID, ActorID: 1, TMDBID: 1, MediaType: "movie", Fingerprint: "old", MatchJSON: "{}", PlanJSON: "{}", ExpiresAt: old, CreatedAt: old}
	recentPreview := model.ScrapePreview{TargetID: 1, CandidateID: candidates[1].ID, ActorID: 1, TMDBID: 2, MediaType: "movie", Fingerprint: "recent", MatchJSON: "{}", PlanJSON: "{}", ExpiresAt: recent.Add(24 * time.Hour), CreatedAt: recent}
	if err := db.Create(&[]model.ScrapePreview{oldPreview, recentPreview}).Error; err != nil {
		t.Fatal(err)
	}
	var previews []model.ScrapePreview
	if err := db.Order("id ASC").Find(&previews).Error; err != nil {
		t.Fatal(err)
	}
	oldJob := model.ScrapeJob{TargetID: 1, PreviewID: previews[0].ID, CandidateID: candidates[0].ID, ActorID: 1, SourceType: "local", SourceRoot: "/media", Status: "succeeded", Stage: "completed", CompletedAt: &old, CreatedAt: old}
	recentJob := model.ScrapeJob{TargetID: 1, PreviewID: previews[1].ID, CandidateID: candidates[1].ID, ActorID: 1, SourceType: "local", SourceRoot: "/media", Status: "pending", Stage: "preparing", CreatedAt: recent}
	if err := db.Create(&[]model.ScrapeJob{oldJob, recentJob}).Error; err != nil {
		t.Fatal(err)
	}
	var jobs []model.ScrapeJob
	if err := db.Order("id ASC").Find(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]model.ScrapeJobOperation{
		{JobID: jobs[0].ID, Sequence: 1, Type: "upload", TargetPath: "/old.nfo", Status: "succeeded"},
		{JobID: jobs[1].ID, Sequence: 1, Type: "upload", TargetPath: "/recent.nfo", Status: "pending"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AdminAuditLog{ActorID: 1, Action: "old.audit", Target: "test", OccurredAt: old}).Error; err != nil {
		t.Fatal(err)
	}

	stats, err := New(db, 30).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Jobs != 1 || stats.Operations != 1 || stats.Previews != 1 || stats.Candidates != 1 || stats.Scans != 1 {
		t.Fatalf("unexpected cleanup stats: %#v", stats)
	}
	for modelValue, expected := range map[any]int64{
		&model.ScrapeJob{}: 1, &model.ScrapeJobOperation{}: 1, &model.ScrapePreview{}: 1,
		&model.MediaCandidate{}: 1, &model.ScanRun{}: 1, &model.AdminAuditLog{}: 1,
	} {
		var count int64
		if err := db.Model(modelValue).Count(&count).Error; err != nil || count != expected {
			t.Fatalf("%T count=%d err=%v, want %d", modelValue, count, err, expected)
		}
	}
}

func TestRunUsesIndependentJobAndCatalogRetentionPeriods(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:maintenance-retention?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ScanRun{}, &model.MediaCandidate{}, &model.ScrapePreview{}, &model.ScrapeJob{}, &model.ScrapeJobOperation{}); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().AddDate(0, 0, -10)
	completed := model.ScrapeJob{TargetID: 1, PreviewID: 1, CandidateID: 1, ActorID: 1, SourceType: "local", SourceRoot: "/media", Status: "succeeded", Stage: "completed", CompletedAt: &old, CreatedAt: old}
	active := model.ScrapeJob{TargetID: 2, PreviewID: 2, CandidateID: 2, ActorID: 1, SourceType: "local", SourceRoot: "/media", Status: "running", Stage: "uploading", CreatedAt: old}
	if err := db.Create(&[]model.ScrapeJob{completed, active}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ScanRun{TargetID: 3, Status: "succeeded", StartedAt: &old, CompletedAt: &old, CreatedAt: old}).Error; err != nil {
		t.Fatal(err)
	}
	stats, err := New(db, 30, 7).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Jobs != 1 || stats.Scans != 0 {
		t.Fatalf("retention periods were not independent: %#v", stats)
	}
	var jobs, scans int64
	_ = db.Model(&model.ScrapeJob{}).Count(&jobs).Error
	_ = db.Model(&model.ScanRun{}).Count(&scans).Error
	if jobs != 1 || scans != 1 {
		t.Fatalf("unexpected retained records: jobs=%d scans=%d", jobs, scans)
	}
}
