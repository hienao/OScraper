package service

import (
	"context"
	"fmt"
	"testing"

	"oscraper/internal/model"
	"oscraper/internal/openlist"
	"oscraper/pkg/cryptoutil"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type catalogBrowser struct {
	levels map[string][]openlist.DirectoryEntry
}

func (b catalogBrowser) ListDirectory(_ context.Context, _, _, remotePath string, _ bool) ([]openlist.DirectoryEntry, error) {
	entries, ok := b.levels[remotePath]
	if !ok {
		return nil, fmt.Errorf("unexpected path %s", remotePath)
	}
	return entries, nil
}

func newCatalogTestService(t *testing.T, libraryType string, levels map[string][]openlist.DirectoryEntry) (*CatalogService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:catalog-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.OpenListConnection{}, &model.ScrapeTarget{}, &model.ScanRun{}, &model.MediaCandidate{}, &model.ScrapePreview{}, &model.AdminAuditLog{}); err != nil {
		t.Fatal(err)
	}
	cipher, err := cryptoutil.New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	token, err := cipher.Encrypt("token")
	if err != nil {
		t.Fatal(err)
	}
	connection := model.OpenListConnection{Name: "Home", BaseURL: "http://openlist.example", BasePath: "/media", EncryptedToken: token, Enabled: true}
	if err := db.Create(&connection).Error; err != nil {
		t.Fatal(err)
	}
	target := model.ScrapeTarget{ConnectionID: connection.ID, Name: "Library", RootPath: "/media/library", LibraryType: libraryType, Enabled: true}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	return NewCatalogService(db, cipher, catalogBrowser{levels: levels}), db
}

func TestMovieScanFindsFoldersAndFlatVideos(t *testing.T) {
	service, _ := newCatalogTestService(t, "movie", map[string][]openlist.DirectoryEntry{
		"/media/library": {
			{Name: "Arrival (2016)", Path: "/media/library/Arrival (2016)", IsDir: true},
			{Name: "Inception.2010.mkv", Path: "/media/library/Inception.2010.mkv", Size: 100},
			{Name: "poster.jpg", Path: "/media/library/poster.jpg", Size: 20},
		},
		"/media/library/Arrival (2016)": {
			{Name: "Arrival.mkv", Path: "/media/library/Arrival (2016)/Arrival.mkv", Size: 200, Modified: "2026-01-01"},
		},
	})
	result, err := service.Scan(context.Background(), 1, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "succeeded" || result.CandidateCount != 2 || result.VideoCount != 2 {
		t.Fatalf("unexpected scan: %#v", result.ScanRun)
	}
	if result.Candidates[0].ParsedTitle != "Arrival" || result.Candidates[0].Year == nil || *result.Candidates[0].Year != 2016 {
		t.Fatalf("unexpected directory candidate: %#v", result.Candidates[0])
	}
	if result.Candidates[1].ParsedTitle != "Inception" || result.Candidates[1].Year == nil || *result.Candidates[1].Year != 2010 {
		t.Fatalf("unexpected flat candidate: %#v", result.Candidates[1])
	}
	if result.Candidates[0].Fingerprint[:7] != "sha256:" {
		t.Fatalf("unexpected fingerprint: %s", result.Candidates[0].Fingerprint)
	}
}

func TestTVScanUsesRecursiveRepresentativeEpisode(t *testing.T) {
	service, _ := newCatalogTestService(t, "tv", map[string][]openlist.DirectoryEntry{
		"/media/library":                               {{Name: "Breaking Bad (2008)", Path: "/media/library/Breaking Bad (2008)", IsDir: true}},
		"/media/library/Breaking Bad (2008)":           {{Name: "Season 05", Path: "/media/library/Breaking Bad (2008)/Season 05", IsDir: true}},
		"/media/library/Breaking Bad (2008)/Season 05": {{Name: "Breaking.Bad.S05E14.mkv", Path: "/media/library/Breaking Bad (2008)/Season 05/Breaking.Bad.S05E14.mkv", Size: 300}},
	})
	result, err := service.Scan(context.Background(), 1, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	candidate := result.Candidates[0]
	if candidate.ParsedTitle != "Breaking Bad" || candidate.Season == nil || *candidate.Season != 5 || candidate.Episode == nil || *candidate.Episode != 14 {
		t.Fatalf("unexpected candidate: %#v", candidate)
	}
	loaded, err := service.Candidates(1, 0)
	if err != nil || len(loaded) != 1 || loaded[0].ID == 0 {
		t.Fatalf("candidate was not persisted: %#v, %v", loaded, err)
	}
}

func TestScanRejectsEntryOutsideCandidateRoot(t *testing.T) {
	service, _ := newCatalogTestService(t, "movie", map[string][]openlist.DirectoryEntry{
		"/media/library":      {{Name: "Film", Path: "/media/library/Film", IsDir: true}},
		"/media/library/Film": {{Name: "escape.mkv", Path: "/media/other/escape.mkv", Size: 1}},
	})
	_, err := service.Scan(context.Background(), 1, 1, false)
	if err == nil {
		t.Fatal("expected boundary violation")
	}
	serviceError, ok := err.(*Error)
	if !ok || serviceError.Code != "scan.path_outside_candidate" {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestCandidateInspectionDetectsStaleFingerprint(t *testing.T) {
	levels := map[string][]openlist.DirectoryEntry{
		"/media/library":                {{Name: "Arrival (2016)", Path: "/media/library/Arrival (2016)", IsDir: true}},
		"/media/library/Arrival (2016)": {{Name: "Arrival.mkv", Path: "/media/library/Arrival (2016)/Arrival.mkv", Size: 100, Modified: "v1"}},
	}
	service, _ := newCatalogTestService(t, "movie", levels)
	scan, err := service.Scan(context.Background(), 1, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if scan.Candidates[0].ManifestJSON == "" {
		t.Fatal("scan did not persist the candidate manifest")
	}
	inspection, err := service.InspectCandidate(context.Background(), 1, scan.Candidates[0].ID, true)
	if err != nil || inspection.Stale {
		t.Fatalf("fresh candidate reported stale: %#v %v", inspection, err)
	}
	levels["/media/library/Arrival (2016)"] = []openlist.DirectoryEntry{{Name: "Arrival.mkv", Path: "/media/library/Arrival (2016)/Arrival.mkv", Size: 200, Modified: "v2"}}
	inspection, err = service.InspectCandidate(context.Background(), 1, scan.Candidates[0].ID, true)
	if err != nil || !inspection.Stale {
		t.Fatalf("changed candidate was not stale: %#v %v", inspection, err)
	}
}

func TestFlatMovieFingerprintIncludesCompanionFiles(t *testing.T) {
	levels := map[string][]openlist.DirectoryEntry{
		"/media/library": {
			{Name: "Arrival.mkv", Path: "/media/library/Arrival.mkv", Size: 100, Modified: "v1"},
			{Name: "Arrival.zh-CN.ass", Path: "/media/library/Arrival.zh-CN.ass", Size: 10, Modified: "v1"},
		},
	}
	service, _ := newCatalogTestService(t, "movie", levels)
	scan, err := service.Scan(context.Background(), 1, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	levels["/media/library"][1].Modified = "v2"
	inspection, err := service.InspectCandidate(context.Background(), 1, scan.Candidates[0].ID, true)
	if err != nil || !inspection.Stale {
		t.Fatalf("changed companion file was not included in the fingerprint: %#v %v", inspection, err)
	}
}
