package service

import (
	"context"
	"fmt"
	"testing"

	"openlistscraper/internal/model"
	"openlistscraper/internal/provider/tmdb"
	"openlistscraper/pkg/cryptoutil"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubTMDBCatalog struct {
	results []tmdb.SearchResult
	detail  *tmdb.Detail
}

func (s *stubTMDBCatalog) Search(context.Context, tmdb.Config, string, string, int) ([]tmdb.SearchResult, error) {
	return append([]tmdb.SearchResult(nil), s.results...), nil
}

func (s *stubTMDBCatalog) Detail(context.Context, tmdb.Config, string, int) (*tmdb.Detail, error) {
	copy := *s.detail
	return &copy, nil
}

func newPreviewTestService(t *testing.T, provider *stubTMDBCatalog) (*PreviewService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:preview-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ScrapeTarget{}, &model.ScanRun{}, &model.MediaCandidate{}, &model.ScrapePreview{}, &model.SystemSetting{}, &model.AdminAuditLog{}); err != nil {
		t.Fatal(err)
	}
	cipher, err := cryptoutil.New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := cipher.Encrypt("tmdb-key")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SystemSetting{Key: settingTMDBAPIKey, Value: encrypted, IsSecret: true}).Error; err != nil {
		t.Fatal(err)
	}
	target := model.ScrapeTarget{Name: "Movies", RootPath: "/movies", LibraryType: "movie", RenameEnabled: true, Enabled: true}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	scan := model.ScanRun{TargetID: target.ID, Status: "succeeded"}
	if err := db.Create(&scan).Error; err != nil {
		t.Fatal(err)
	}
	candidate := model.MediaCandidate{
		ScanID: scan.ID, TargetID: target.ID, Path: "/movies/Arrival.mkv", Kind: "movie",
		Fingerprint: "sha256:original", RepresentativeFile: "Arrival.mkv", ParsedTitle: "Arrival", VideoCount: 1, Status: "ready",
	}
	year := 2016
	candidate.Year = &year
	if err := db.Create(&candidate).Error; err != nil {
		t.Fatal(err)
	}
	settings := NewSettingService(db, cipher, stubTMDBTester{})
	return NewPreviewService(db, settings, provider), db
}

func TestPreviewSearchPrioritizesExactYear(t *testing.T) {
	provider := &stubTMDBCatalog{results: []tmdb.SearchResult{
		{ID: 1, Title: "Arrival", Year: 2024, VoteAverage: 9.0},
		{ID: 2, Title: "Arrival", Year: 2016, VoteAverage: 7.6},
	}}
	service, _ := newPreviewTestService(t, provider)
	results, err := service.Search(context.Background(), 1, PreviewSearchRequest{CandidateID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].ID != 2 {
		t.Fatalf("exact year was not preferred: %#v", results)
	}
}

func TestCreatePreviewPersistsImmutableMatchAndPlan(t *testing.T) {
	provider := &stubTMDBCatalog{detail: &tmdb.Detail{ID: 329865, MediaType: "movie", Title: "Arrival/降临", OriginalTitle: "Arrival", Year: 2016, Overview: "First snapshot"}}
	service, _ := newPreviewTestService(t, provider)
	created, err := service.Create(context.Background(), 1, 1, CreatePreviewRequest{CandidateID: 1, TMDBID: 329865})
	if err != nil {
		t.Fatal(err)
	}
	if !created.Plan.ReadOnly || !created.Plan.Ready || !created.Plan.OrganizeFlatMovie || created.Fingerprint != "sha256:original" {
		t.Fatalf("unexpected preview: %#v", created)
	}
	if created.Plan.ProposedDirectoryName != "Arrival／降临 (2016) {tmdbid-329865}" || len(created.Plan.GeneratedFiles) != 3 {
		t.Fatalf("unexpected plan: %#v", created.Plan)
	}
	provider.detail.Overview = "Changed provider data"
	loaded, err := service.Get(1, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Match.Overview != "First snapshot" || loaded.Plan.ProposedDirectoryName != created.Plan.ProposedDirectoryName {
		t.Fatalf("preview snapshot was not immutable: %#v", loaded)
	}
}

func TestRenameDisabledKeepsMetadataAtCurrentLocation(t *testing.T) {
	provider := &stubTMDBCatalog{detail: &tmdb.Detail{ID: 329865, MediaType: "movie", Title: "Arrival", Year: 2016}}
	service, db := newPreviewTestService(t, provider)
	if err := db.Model(&model.ScrapeTarget{}).Where("id = ?", 1).Update("rename_enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	created, err := service.Create(context.Background(), 1, 1, CreatePreviewRequest{CandidateID: 1, TMDBID: 329865})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Plan.ProposedDirectoryRenames) != 0 || len(created.Plan.ProposedFileRenames) != 0 || created.Plan.ProposedDirectoryPath != "/movies" {
		t.Fatalf("rename-disabled plan contains mutations: %#v", created.Plan)
	}
	if created.Plan.GeneratedFiles[0] != "/movies/Arrival (2016) {tmdbid-329865}.nfo" {
		t.Fatalf("metadata target escaped current directory: %#v", created.Plan.GeneratedFiles)
	}
}
