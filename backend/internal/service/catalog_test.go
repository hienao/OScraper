package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"oscraper/internal/media"
	"oscraper/internal/model"
	"oscraper/internal/openlist"
	"oscraper/pkg/cryptoutil"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type catalogBrowser struct {
	levels map[string][]openlist.DirectoryEntry
}

type stubMediaRecognizer struct {
	info  media.Info
	calls int
}

func (s *stubMediaRecognizer) Recognize(_ context.Context, fileName, relativePath, libraryType string) (media.Info, bool, error) {
	s.calls++
	if fileName != "unknown.mkv" || relativePath != "Mystery Show/unknown.mkv" || libraryType != "tv" {
		return media.Info{}, false, fmt.Errorf("unexpected AI input: %s %s %s", fileName, relativePath, libraryType)
	}
	return s.info, true, nil
}

func TestLocalMovieScanUsesMountedDirectory(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "movies")
	film := filepath.Join(library, "Arrival (2016)")
	if err := os.MkdirAll(film, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(film, "Arrival.mkv"), []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:local-catalog-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ScrapeTarget{}, &model.ScanRun{}, &model.MediaCandidate{}, &model.AdminAuditLog{}); err != nil {
		t.Fatal(err)
	}
	target := model.ScrapeTarget{SourceType: "local", Name: "Local movies", RootPath: filepath.ToSlash(library), LibraryType: "movie", Enabled: true}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	cipher, _ := cryptoutil.New("0123456789abcdef0123456789abcdef")
	service := NewCatalogServiceWithLocalRoot(db, cipher, catalogBrowser{levels: map[string][]openlist.DirectoryEntry{}}, nil, root)
	result, err := service.Scan(context.Background(), target.ID, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.CandidateCount != 1 || result.VideoCount != 1 || result.Candidates[0].ParsedTitle != "Arrival" {
		t.Fatalf("unexpected local scan: %#v", result)
	}
}

func TestLocalScanReportsMissingDirectoryClearly(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "removed-library")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:missing-local-catalog-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ScrapeTarget{}, &model.ScanRun{}, &model.MediaCandidate{}, &model.AdminAuditLog{}); err != nil {
		t.Fatal(err)
	}
	target := model.ScrapeTarget{SourceType: "local", Name: "Removed library", RootPath: filepath.ToSlash(library), LibraryType: "movie", Enabled: true}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	cipher, _ := cryptoutil.New("0123456789abcdef0123456789abcdef")
	service := NewCatalogServiceWithLocalRoot(db, cipher, catalogBrowser{levels: map[string][]openlist.DirectoryEntry{}}, nil, root)
	_, scanErr := service.Scan(context.Background(), target.ID, 1, false)
	var serviceErr *Error
	if !errors.As(scanErr, &serviceErr) || serviceErr.Code != "local.path_unavailable" || serviceErr.Message != "The directory does not exist. Please select a directory again" {
		t.Fatalf("unexpected missing directory error: %#v", scanErr)
	}
	var scan model.ScanRun
	if err := db.First(&scan).Error; err != nil {
		t.Fatal(err)
	}
	if scan.Status != "failed" || scan.ErrorCode != "local.path_unavailable" || scan.ErrorMessage != serviceErr.Message {
		t.Fatalf("unexpected stored scan failure: %#v", scan)
	}
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
	// Scan recovery dispatches a worker goroutine while tests poll the result; the
	// shared in-memory database needs a single connection to avoid table lock errors.
	if sqlDB, err := db.DB(); err != nil {
		t.Fatal(err)
	} else {
		sqlDB.SetMaxOpenConns(1)
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
	connectionID := connection.ID
	target := model.ScrapeTarget{SourceType: "openlist", ConnectionID: &connectionID, Name: "Library", RootPath: "/media/library", LibraryType: libraryType, Enabled: true}
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
			{Name: "Inception.2010.mkv" + scrapeMarkerSuffix, Path: "/media/library/Inception.2010.mkv" + scrapeMarkerSuffix, Size: int64(len(scrapeMarkerContent))},
			{Name: "poster.jpg", Path: "/media/library/poster.jpg", Size: 20},
		},
		"/media/library/Arrival (2016)": {
			{Name: "Arrival.mkv", Path: "/media/library/Arrival (2016)/Arrival.mkv", Size: 200, Modified: "2026-01-01"},
			{Name: scrapeMarkerName, Path: "/media/library/Arrival (2016)/" + scrapeMarkerName, Size: int64(len(scrapeMarkerContent))},
		},
	})
	result, err := service.Scan(context.Background(), 1, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "succeeded" || result.CandidateCount != 2 || result.ScrapedCount != 2 || result.VideoCount != 2 {
		t.Fatalf("unexpected scan: %#v", result.ScanRun)
	}
	if result.Candidates[0].ParsedTitle != "Arrival" || result.Candidates[0].Year == nil || *result.Candidates[0].Year != 2016 {
		t.Fatalf("unexpected directory candidate: %#v", result.Candidates[0])
	}
	if result.Candidates[1].ParsedTitle != "Inception" || result.Candidates[1].Year == nil || *result.Candidates[1].Year != 2010 {
		t.Fatalf("unexpected flat candidate: %#v", result.Candidates[1])
	}
	if !result.Candidates[0].Scraped || !result.Candidates[1].Scraped {
		t.Fatalf("scrape markers were not detected: %#v", result.Candidates)
	}
	if result.Candidates[0].Fingerprint[:7] != "sha256:" {
		t.Fatalf("unexpected fingerprint: %s", result.Candidates[0].Fingerprint)
	}
}

func TestMovieScanGroupsLooseVersionsByTitleAndYear(t *testing.T) {
	service, _ := newCatalogTestService(t, "movie", map[string][]openlist.DirectoryEntry{
		"/media/library": {
			{Name: "Arrival.2016.2160p.mkv", Path: "/media/library/Arrival.2016.2160p.mkv", Size: 200},
			{Name: "Arrival.2016.2160p.zh.srt", Path: "/media/library/Arrival.2016.2160p.zh.srt", Size: 10},
			{Name: "Arrival.2016.1080p.mp4", Path: "/media/library/Arrival.2016.1080p.mp4", Size: 100},
			{Name: "Unrelated.mkv", Path: "/media/library/Unrelated.mkv", Size: 50},
		},
	})
	result, err := service.Scan(context.Background(), 1, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.CandidateCount != 2 || result.VideoCount != 3 {
		t.Fatalf("unexpected grouped scan totals: %#v", result.ScanRun)
	}
	var grouped *model.MediaCandidate
	for index := range result.Candidates {
		if result.Candidates[index].ParsedTitle == "Arrival" {
			grouped = &result.Candidates[index]
		}
	}
	if grouped == nil || grouped.VideoCount != 2 {
		t.Fatalf("loose versions were not grouped: %#v", result.Candidates)
	}
	var manifest []openlist.DirectoryEntry
	if err := json.Unmarshal([]byte(grouped.ManifestJSON), &manifest); err != nil || len(manifest) != 3 {
		t.Fatalf("group assets were not persisted: %#v, %v", manifest, err)
	}
}

func TestMovieScanKeepsLooseFilesWithoutYearSeparate(t *testing.T) {
	service, _ := newCatalogTestService(t, "movie", map[string][]openlist.DirectoryEntry{
		"/media/library": {
			{Name: "Arrival.2160p.mkv", Path: "/media/library/Arrival.2160p.mkv", Size: 200},
			{Name: "Arrival.1080p.mp4", Path: "/media/library/Arrival.1080p.mp4", Size: 100},
		},
	})
	result, err := service.Scan(context.Background(), 1, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.CandidateCount != 2 || result.Candidates[0].VideoCount != 1 || result.Candidates[1].VideoCount != 1 {
		t.Fatalf("yearless loose files were grouped unsafely: %#v", result.Candidates)
	}
}

func TestScrapeMarkersDoNotChangeCandidateFingerprint(t *testing.T) {
	video := openlist.DirectoryEntry{Name: "Arrival.mkv", Path: "/media/library/Arrival/Arrival.mkv", Size: 100, Modified: "v1"}
	marker := openlist.DirectoryEntry{Name: scrapeMarkerName, Path: "/media/library/Arrival/" + scrapeMarkerName, Size: int64(len(scrapeMarkerContent)), Modified: "v1"}
	first := fingerprint([]openlist.DirectoryEntry{video, marker})
	marker.Modified = "v2"
	marker.Size = 999
	second := fingerprint([]openlist.DirectoryEntry{video, marker})
	if first != second {
		t.Fatalf("scrape marker changed candidate fingerprint: %s != %s", first, second)
	}
}

func TestCatalogStartRecoversPendingScan(t *testing.T) {
	service, _ := newCatalogTestService(t, "movie", map[string][]openlist.DirectoryEntry{
		"/media/library":                {{Name: "Arrival (2016)", Path: "/media/library/Arrival (2016)", IsDir: true}},
		"/media/library/Arrival (2016)": {{Name: "Arrival.mkv", Path: "/media/library/Arrival (2016)/Arrival.mkv", Size: 100}},
	})
	scan := model.ScanRun{TargetID: 1, ActorID: 7, Status: "pending"}
	if err := service.catalog.CreateScan(&scan); err != nil {
		t.Fatal(err)
	}
	if err := service.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Shutdown(context.Background()) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		result, err := service.GetScan(1, scan.ID)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status == "succeeded" {
			if result.CandidateCount != 1 || len(result.Candidates) != 1 {
				t.Fatalf("unexpected recovered scan result: %#v", result)
			}
			break
		}
		if result.Status == "failed" || time.Now().After(deadline) {
			t.Fatalf("pending scan was not recovered: %#v", result.ScanRun)
		}
		time.Sleep(10 * time.Millisecond)
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

func TestLowConfidenceCandidateUsesAIRecognition(t *testing.T) {
	service, _ := newCatalogTestService(t, "tv", map[string][]openlist.DirectoryEntry{
		"/media/library":              {{Name: "Mystery Show", Path: "/media/library/Mystery Show", IsDir: true}},
		"/media/library/Mystery Show": {{Name: "unknown.mkv", Path: "/media/library/Mystery Show/unknown.mkv", Size: 100}},
	})
	season, episode := 2, 7
	recognizer := &stubMediaRecognizer{info: media.Info{Title: "Recognized Show", Season: &season, Episode: &episode, Confidence: 95}}
	service.recognizer = recognizer
	result, err := service.Scan(context.Background(), 1, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	candidate := result.Candidates[0]
	if recognizer.calls != 1 || candidate.ParsedTitle != "Recognized Show" || candidate.Status != "ready" || candidate.Episode == nil || *candidate.Episode != 7 {
		t.Fatalf("AI recognition was not applied: %#v, calls=%d", candidate, recognizer.calls)
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
