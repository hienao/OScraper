package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"openlistscraper/internal/model"
	"openlistscraper/internal/openlist"
	"openlistscraper/pkg/cryptoutil"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubDirectoryBrowser struct {
	entries []openlist.DirectoryEntry
	err     error
}

func (s stubDirectoryBrowser) ListDirectory(context.Context, string, string, string, bool) ([]openlist.DirectoryEntry, error) {
	return s.entries, s.err
}

func newTargetTestService(t *testing.T) (*TargetService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:target-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.OpenListConnection{}, &model.ScrapeTarget{}, &model.ScanRun{}, &model.MediaCandidate{}, &model.ScrapePreview{}, &model.ScrapeJob{}, &model.ScrapeJobOperation{}, &model.AdminAuditLog{}); err != nil {
		t.Fatal(err)
	}
	cipher, err := cryptoutil.New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := cipher.Encrypt("token")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.OpenListConnection{Name: "Home", BaseURL: "http://openlist.example", BasePath: "/media", EncryptedToken: encrypted, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	service := NewTargetService(db, cipher, stubDirectoryBrowser{entries: []openlist.DirectoryEntry{{Name: "Movies", Path: "/media/Movies", IsDir: true}}})
	return service, db
}

func TestTargetCreateValidatesAccountBoundary(t *testing.T) {
	targetService, _ := newTargetTestService(t)
	_, err := targetService.Create(context.Background(), 1, TargetRequest{ConnectionID: 1, Name: "Escape", RootPath: "/media-old", LibraryType: "movie", Enabled: true})
	if err == nil {
		t.Fatal("expected target outside account root to fail")
	}
	serviceError, ok := err.(*Error)
	if !ok || serviceError.Code != "target.path_outside_account" {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestBrowseRejectsPathOutsideTargetRoot(t *testing.T) {
	targetService, _ := newTargetTestService(t)
	target, err := targetService.Create(context.Background(), 1, TargetRequest{ConnectionID: 1, Name: "Movies", RootPath: "/media/Movies", LibraryType: "movie", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = targetService.Browse(context.Background(), target.ID, "/media/TV", false)
	if err == nil {
		t.Fatal("expected path outside target root to fail")
	}
	serviceError, ok := err.(*Error)
	if !ok || serviceError.Code != "target.path_outside_root" {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestDeleteTargetRemovesScanCatalogAtomically(t *testing.T) {
	targetService, db := newTargetTestService(t)
	target, err := targetService.Create(context.Background(), 1, TargetRequest{ConnectionID: 1, Name: "Movies", RootPath: "/media/Movies", LibraryType: "movie", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	scan := model.ScanRun{TargetID: target.ID, Status: "succeeded", StartedAt: time.Now(), CandidateCount: 1}
	if err := db.Create(&scan).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.MediaCandidate{ScanID: scan.ID, TargetID: target.ID, Path: "/media/Movies/Film", Kind: "movie", Fingerprint: "sha256:test", Status: "ready"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ScrapePreview{TargetID: target.ID, CandidateID: 1, ActorID: 1, TMDBID: 1, MediaType: "movie", Fingerprint: "sha256:test", MatchJSON: "{}", PlanJSON: "{}", ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := targetService.Delete(target.ID, 1); err != nil {
		t.Fatal(err)
	}
	var scanCount, candidateCount, previewCount int64
	db.Model(&model.ScanRun{}).Where("target_id = ?", target.ID).Count(&scanCount)
	db.Model(&model.MediaCandidate{}).Where("target_id = ?", target.ID).Count(&candidateCount)
	db.Model(&model.ScrapePreview{}).Where("target_id = ?", target.ID).Count(&previewCount)
	if scanCount != 0 || candidateCount != 0 || previewCount != 0 {
		t.Fatalf("catalog was not deleted: scans=%d candidates=%d previews=%d", scanCount, candidateCount, previewCount)
	}
}
