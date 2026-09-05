package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"oscraper/internal/model"
	"oscraper/internal/provider/tmdb"
	"oscraper/pkg/cryptoutil"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubBatchPreviewer struct {
	searchResults      map[uint][]tmdb.SearchResult
	searchErr          error
	preview            *PreviewResponse
	previewByCandidate map[uint]*PreviewResponse
	createErr          error
	createDelay        time.Duration
	searches           []SearchPreviewCommand
}

func (s *stubBatchPreviewer) Search(_ context.Context, _ uint, request SearchPreviewCommand) ([]tmdb.SearchResult, error) {
	s.searches = append(s.searches, request)
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return append([]tmdb.SearchResult(nil), s.searchResults[request.CandidateID]...), nil
}

func (s *stubBatchPreviewer) Create(ctx context.Context, _ uint, _ uint, request CreatePreviewCommand) (*PreviewResponse, error) {
	if s.createDelay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(s.createDelay):
		}
	}
	if s.createErr != nil {
		return nil, s.createErr
	}
	source := s.preview
	if perCandidate, ok := s.previewByCandidate[request.CandidateID]; ok {
		source = perCandidate
	}
	if source == nil {
		return nil, NotFound("preview.not_found", "Scrape preview not found")
	}
	preview := *source
	preview.CandidateID = request.CandidateID
	if preview.Match.ID == 0 {
		preview.Match.ID = request.TMDBID
	}
	return &preview, nil
}

type stubBatchJobSubmitter struct {
	jobs   []SubmitJobCommand
	keys   []string
	err    error
	errs   []error
	called chan struct{}
}

func (s *stubBatchJobSubmitter) Submit(targetID, actorID uint, request SubmitJobCommand, key string) (*model.ScrapeJob, error) {
	s.jobs = append(s.jobs, request)
	s.keys = append(s.keys, key)
	if s.called != nil {
		select {
		case s.called <- struct{}{}:
		default:
		}
	}
	if index := len(s.jobs) - 1; index < len(s.errs) && s.errs[index] != nil {
		return nil, s.errs[index]
	}
	if s.err != nil {
		return nil, s.err
	}
	return &model.ScrapeJob{ID: uint(len(s.jobs)), TargetID: targetID, PreviewID: request.PreviewID, ActorID: actorID, Status: "pending"}, nil
}

var batchTestDBCounter int

func newBatchTestService(t *testing.T, previews BatchPreviewer, jobs BatchJobSubmitter, candidates []model.MediaCandidate) (*BatchScrapeService, *gorm.DB) {
	t.Helper()
	batchTestDBCounter++
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:batch-%s-%d?mode=memory&cache=shared", t.Name(), batchTestDBCounter)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// The batch runner writes from a background goroutine while tests poll; the
	// shared in-memory database needs a single connection to avoid lock errors.
	if sqlDB, err := db.DB(); err != nil {
		t.Fatal(err)
	} else {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&model.ScrapeTarget{}, &model.ScanRun{}, &model.MediaCandidate{}, &model.ScrapePreview{},
		&model.ScrapeBatchRun{}, &model.ScrapeBatchItem{}, &model.SystemSetting{}, &model.AdminAuditLog{}); err != nil {
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
	target := model.ScrapeTarget{Name: "Movies", RootPath: "/movies", LibraryType: "movie", Enabled: true}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	scan := model.ScanRun{TargetID: target.ID, Status: "succeeded"}
	if err := db.Create(&scan).Error; err != nil {
		t.Fatal(err)
	}
	for index := range candidates {
		candidates[index].ScanID = scan.ID
		candidates[index].TargetID = target.ID
		if candidates[index].Kind == "" {
			candidates[index].Kind = "movie"
		}
		if candidates[index].Status == "" {
			candidates[index].Status = "ready"
		}
		if err := db.Create(&candidates[index]).Error; err != nil {
			t.Fatal(err)
		}
	}
	settings := NewSettingService(db, cipher, stubTMDBTester{})
	originalDelay := batchItemDelay
	batchItemDelay = time.Millisecond
	t.Cleanup(func() { batchItemDelay = originalDelay })
	return NewBatchScrapeService(db, settings, previews, jobs, 1, 5), db
}

func batchCandidate(path, title string) model.MediaCandidate {
	return model.MediaCandidate{Path: path, Fingerprint: "sha256:" + path, RepresentativeFile: title + ".mkv", ParsedTitle: title, VideoCount: 1}
}

func waitForBatch(t *testing.T, service *BatchScrapeService, targetID, batchID uint, timeout time.Duration) *BatchScrapeResponse {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		response, err := service.GetBatch(targetID, batchID)
		if err != nil {
			t.Fatal(err)
		}
		if response.Status != "pending" && response.Status != "running" {
			return response
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("batch did not finish in time")
	return nil
}

func readyPreview() *PreviewResponse {
	return &PreviewResponse{
		Fingerprint: "sha256:preview",
		Match:       tmdb.Detail{ID: 329865, MediaType: "movie", Title: "Arrival", Year: 2016},
		Plan: PreviewPlan{ReadOnly: true, Ready: true, SourcePath: "/movies/Arrival.mkv",
			ProposedFileRenames: []RenameItem{{SourcePath: "/movies/Arrival.mkv", TargetPath: "/movies/Arrival (2016)/Arrival.mkv"}}},
	}
}

func TestBatchScrapeSubmitsUniqueMatchesAndSkipsAmbiguous(t *testing.T) {
	previewer := &stubBatchPreviewer{
		searchResults: map[uint][]tmdb.SearchResult{
			1: {{ID: 329865, Title: "Arrival", Year: 2016}},
			2: {{ID: 11, Title: "Alien", Year: 1979}, {ID: 12, Title: "Aliens", Year: 1986}},
		},
		preview: readyPreview(),
	}
	submitter := &stubBatchJobSubmitter{}
	known := 4
	candidates := []model.MediaCandidate{
		batchCandidate("/movies/Arrival/Arrival.mkv", "Arrival"),
		batchCandidate("/movies/Aliens/Aliens.mkv", "Aliens"),
		batchCandidate("/movies/Unknown/Unknown.mkv", "Unknown"),
		func() model.MediaCandidate {
			c := batchCandidate("/movies/Known/Known.mkv", "Known")
			c.TMDBID = &known
			return c
		}(),
	}
	service, _ := newBatchTestService(t, previewer, submitter, candidates)
	batch, err := service.StartBatch(1, 7, BatchScrapeCommand{})
	if err != nil {
		t.Fatal(err)
	}
	result := waitForBatch(t, service, 1, batch.ID, 10*time.Second)
	if result.Status != "succeeded" || result.SubmittedCount != 2 || result.SkippedCount != 2 || result.FailedCount != 0 {
		t.Fatalf("unexpected batch result: %#v", result.ScrapeBatchRun)
	}
	// Every submitted item must carry the deterministic idempotency key batch-<batch>-<item>.
	var expectedKeys []string
	for _, item := range result.Items {
		if item.Status == "submitted" {
			expectedKeys = append(expectedKeys, fmt.Sprintf("batch-%d-%d", batch.ID, item.ID))
		}
	}
	if len(submitter.jobs) != 2 || len(submitter.keys) != 2 {
		t.Fatalf("unexpected submitter calls: %#v", submitter.jobs)
	}
	for _, command := range submitter.jobs {
		if !command.RenameMedia || command.ConfirmDirectoryFingerprint != "sha256:preview" {
			t.Fatalf("unexpected submit command: %#v", command)
		}
	}
	sort.Strings(expectedKeys)
	sort.Strings(submitter.keys)
	for index, key := range submitter.keys {
		if key != expectedKeys[index] {
			t.Fatalf("unexpected idempotency key at %d: %s", index, key)
		}
	}
	byStatus := map[uint]model.ScrapeBatchItem{}
	for _, item := range result.Items {
		byStatus[item.CandidateID] = item
	}
	if byStatus[2].Status != "skipped" || byStatus[2].SkipReason != "multiple_matches" {
		t.Fatalf("ambiguous candidate was not skipped: %#v", byStatus[2])
	}
	if byStatus[3].Status != "skipped" || byStatus[3].SkipReason != "no_match" {
		t.Fatalf("unmatched candidate was not skipped: %#v", byStatus[3])
	}
	if byStatus[1].Status != "submitted" || byStatus[1].JobID == nil || byStatus[1].TMDBID == nil || *byStatus[1].TMDBID != 329865 {
		t.Fatalf("unique match was not submitted: %#v", byStatus[1])
	}
	if byStatus[4].Status != "submitted" {
		t.Fatalf("candidate with known TMDB id was not submitted: %#v", byStatus[4])
	}
	if len(previewer.searches) != 3 {
		t.Fatalf("search should run for every candidate without a TMDB id: %#v", previewer.searches)
	}
}

func TestBatchScrapeSkipsBlockedPlansAndActiveMedia(t *testing.T) {
	blocked := readyPreview()
	blocked.Plan.Ready = false
	previewer := &stubBatchPreviewer{
		searchResults: map[uint][]tmdb.SearchResult{
			1: {{ID: 1, Title: "One"}},
			2: {{ID: 2, Title: "Two"}},
			3: {{ID: 3, Title: "Three"}},
		},
		preview:            blocked,
		previewByCandidate: map[uint]*PreviewResponse{3: readyPreview()},
	}
	submitter := &stubBatchJobSubmitter{err: &Error{Code: "job.already_active"}}
	service, _ := newBatchTestService(t, previewer, submitter, []model.MediaCandidate{
		batchCandidate("/movies/One/One.mkv", "One"),
		batchCandidate("/movies/Two/Two.mkv", "Two"),
		batchCandidate("/movies/Three/Three.mkv", "Three"),
	})
	batch, err := service.StartBatch(1, 7, BatchScrapeCommand{})
	if err != nil {
		t.Fatal(err)
	}
	result := waitForBatch(t, service, 1, batch.ID, 10*time.Second)
	if result.Status != "succeeded" || result.SubmittedCount != 0 || result.SkippedCount != 3 {
		t.Fatalf("unexpected batch result: %#v", result.ScrapeBatchRun)
	}
	reasons := map[uint]string{}
	for _, item := range result.Items {
		reasons[item.CandidateID] = item.SkipReason
	}
	if reasons[1] != "plan_conflicts" || reasons[2] != "plan_conflicts" || reasons[3] != "already_active" {
		t.Fatalf("unexpected skip reasons: %#v", reasons)
	}
	if len(submitter.jobs) != 1 {
		t.Fatalf("blocked plans must not reach the job queue: %#v", submitter.jobs)
	}
}

func TestBatchScrapeWaitsForBusyTargetAndRetriesSubmission(t *testing.T) {
	previewer := &stubBatchPreviewer{
		searchResults: map[uint][]tmdb.SearchResult{1: {{ID: 1, Title: "One"}}},
		preview:       readyPreview(),
	}
	submitter := &stubBatchJobSubmitter{errs: []error{&Error{Code: "job.target_busy"}}}
	service, _ := newBatchTestService(t, previewer, submitter, []model.MediaCandidate{
		batchCandidate("/movies/One/One.mkv", "One"),
	})
	originalPollPeriod := batchQueuePollPeriod
	batchQueuePollPeriod = time.Millisecond
	t.Cleanup(func() { batchQueuePollPeriod = originalPollPeriod })

	batch, err := service.StartBatch(1, 7, BatchScrapeCommand{})
	if err != nil {
		t.Fatal(err)
	}
	result := waitForBatch(t, service, 1, batch.ID, 10*time.Second)
	if result.Status != "succeeded" || result.SubmittedCount != 1 || result.SkippedCount != 0 || result.FailedCount != 0 {
		t.Fatalf("unexpected batch result after target became idle: %#v", result.ScrapeBatchRun)
	}
	if len(submitter.jobs) != 2 || len(submitter.keys) != 2 || submitter.keys[0] != submitter.keys[1] {
		t.Fatalf("target-busy submission was not retried idempotently: jobs=%d keys=%#v", len(submitter.jobs), submitter.keys)
	}
}

func TestBatchScrapeCancelStopsBusyTargetWait(t *testing.T) {
	previewer := &stubBatchPreviewer{
		searchResults: map[uint][]tmdb.SearchResult{1: {{ID: 1, Title: "One"}}},
		preview:       readyPreview(),
	}
	called := make(chan struct{}, 1)
	submitter := &stubBatchJobSubmitter{err: &Error{Code: "job.target_busy"}, called: called}
	service, _ := newBatchTestService(t, previewer, submitter, []model.MediaCandidate{
		batchCandidate("/movies/One/One.mkv", "One"),
	})
	originalPollPeriod := batchQueuePollPeriod
	batchQueuePollPeriod = time.Millisecond
	t.Cleanup(func() { batchQueuePollPeriod = originalPollPeriod })

	batch, err := service.StartBatch(1, 7, BatchScrapeCommand{})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("batch did not reach the target-busy wait")
	}
	if _, err := service.Cancel(1, batch.ID, 7); err != nil {
		t.Fatal(err)
	}
	if err := service.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	result := waitForBatch(t, service, 1, batch.ID, 10*time.Second)
	if result.Status != "canceled" || result.SkippedCount != 1 {
		t.Fatalf("canceled batch remained stuck waiting for the target: %#v", result.ScrapeBatchRun)
	}
}

func TestBatchScrapeRetriesRateLimitsThenTripsFuse(t *testing.T) {
	previewer := &stubBatchPreviewer{}
	previewer.preview = readyPreview()
	previewer.searchResults = map[uint][]tmdb.SearchResult{
		1: {{ID: 1, Title: "One"}},
		2: {{ID: 2, Title: "Two"}},
		3: {{ID: 3, Title: "Three"}},
		4: {{ID: 4, Title: "Four"}},
		5: {{ID: 5, Title: "Five"}},
		6: {{ID: 6, Title: "Six"}},
	}
	limited := &Error{Code: "tmdb.rate_limited", Message: "TMDB rate limit was exceeded"}
	previewer.searchErr = limited
	service, _ := newBatchTestService(t, previewer, &stubBatchJobSubmitter{}, []model.MediaCandidate{
		batchCandidate("/movies/One/One.mkv", "One"),
		batchCandidate("/movies/Two/Two.mkv", "Two"),
		batchCandidate("/movies/Three/Three.mkv", "Three"),
		batchCandidate("/movies/Four/Four.mkv", "Four"),
		batchCandidate("/movies/Five/Five.mkv", "Five"),
		batchCandidate("/movies/Six/Six.mkv", "Six"),
	})
	originalDelay := batchRateLimitDelay
	batchRateLimitDelay = time.Millisecond
	t.Cleanup(func() { batchRateLimitDelay = originalDelay })
	batch, err := service.StartBatch(1, 7, BatchScrapeCommand{})
	if err != nil {
		t.Fatal(err)
	}
	result := waitForBatch(t, service, 1, batch.ID, 10*time.Second)
	if result.Status != "failed" || result.ErrorCode != "batch.rate_limited" {
		t.Fatalf("expected rate-limit fuse, got: %#v", result.ScrapeBatchRun)
	}
	if result.FailedCount != batchRateLimitFuse {
		t.Fatalf("unexpected failed count: %d", result.FailedCount)
	}
}

func TestBatchScrapeCancelMarksPendingItems(t *testing.T) {
	previewer := &stubBatchPreviewer{
		searchResults: map[uint][]tmdb.SearchResult{
			1: {{ID: 1, Title: "One"}}, 2: {{ID: 2, Title: "Two"}}, 3: {{ID: 3, Title: "Three"}},
			4: {{ID: 4, Title: "Four"}}, 5: {{ID: 5, Title: "Five"}},
		},
		preview:     readyPreview(),
		createDelay: 30 * time.Millisecond,
	}
	submitter := &stubBatchJobSubmitter{}
	service, _ := newBatchTestService(t, previewer, submitter, []model.MediaCandidate{
		batchCandidate("/movies/One/One.mkv", "One"),
		batchCandidate("/movies/Two/Two.mkv", "Two"),
		batchCandidate("/movies/Three/Three.mkv", "Three"),
		batchCandidate("/movies/Four/Four.mkv", "Four"),
		batchCandidate("/movies/Five/Five.mkv", "Five"),
	})
	batch, err := service.StartBatch(1, 7, BatchScrapeCommand{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := service.Cancel(1, batch.ID, 7); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		result, err := service.GetBatch(1, batch.ID)
		if err != nil {
			t.Fatal(err)
		}
		pending := 0
		for _, item := range result.Items {
			if item.Status == "pending" {
				pending++
			}
		}
		if result.Status == "canceled" && pending == 0 {
			if result.SkippedCount+result.SubmittedCount != result.TotalCount {
				t.Fatalf("canceled counters inconsistent: %#v", result.ScrapeBatchRun)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("cancel did not settle items: %#v", result)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(submitter.jobs) > 1 {
		t.Fatalf("cancel must stop new submissions quickly: %d", len(submitter.jobs))
	}
}

func TestBatchScrapeRejectsWhenNoCandidatesOrMissingScan(t *testing.T) {
	previewer := &stubBatchPreviewer{
		preview:       readyPreview(),
		searchResults: map[uint][]tmdb.SearchResult{1: {{ID: 1, Title: "One"}}},
	}
	service, _ := newBatchTestService(t, previewer, &stubBatchJobSubmitter{}, nil)
	if _, err := service.StartBatch(1, 7, BatchScrapeCommand{}); err == nil {
		t.Fatal("expected failure without any scan")
	}
	scraped := batchCandidate("/movies/One/One.mkv", "One")
	scraped.Scraped = true
	service, db := newBatchTestService(t, previewer, &stubBatchJobSubmitter{}, []model.MediaCandidate{scraped})
	if _, err := service.StartBatch(1, 7, BatchScrapeCommand{}); err == nil {
		t.Fatal("expected failure when every candidate is already scraped")
	}
	var scan model.ScanRun
	if err := db.First(&scan).Error; err != nil {
		t.Fatal(err)
	}
	batch, err := service.StartBatch(1, 7, BatchScrapeCommand{ScanID: scan.ID, IncludeScraped: true})
	if err != nil {
		t.Fatal(err)
	}
	if batch.TotalCount != 1 {
		t.Fatalf("unexpected batch size: %d", batch.TotalCount)
	}
	result := waitForBatch(t, service, 1, batch.ID, 10*time.Second)
	if result.Status != "succeeded" || result.SubmittedCount != 1 {
		t.Fatalf("unexpected include-scraped batch result: %#v", result.ScrapeBatchRun)
	}
}

func TestBatchScrapeStalePreviewIsSkipped(t *testing.T) {
	previewer := &stubBatchPreviewer{
		searchResults: map[uint][]tmdb.SearchResult{1: {{ID: 1, Title: "One"}}},
		createErr:     &Error{Code: "preview.stale", Message: "Media directory changed after the scan"},
	}
	service, _ := newBatchTestService(t, previewer, &stubBatchJobSubmitter{}, []model.MediaCandidate{
		batchCandidate("/movies/One/One.mkv", "One"),
	})
	batch, err := service.StartBatch(1, 7, BatchScrapeCommand{})
	if err != nil {
		t.Fatal(err)
	}
	result := waitForBatch(t, service, 1, batch.ID, 10*time.Second)
	if result.Status != "succeeded" || result.SkippedCount != 1 {
		t.Fatalf("unexpected batch result: %#v", result.ScrapeBatchRun)
	}
	if result.Items[0].SkipReason != "stale" {
		t.Fatalf("stale preview was not mapped to a skip: %#v", result.Items[0])
	}
}

func TestBatchScrapeRequiresTMDBKey(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:batch-nokey-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ScrapeTarget{}, &model.ScanRun{}, &model.MediaCandidate{}, &model.ScrapeBatchRun{}, &model.ScrapeBatchItem{}, &model.SystemSetting{}, &model.AdminAuditLog{}); err != nil {
		t.Fatal(err)
	}
	cipher, _ := cryptoutil.New("0123456789abcdef0123456789abcdef")
	target := model.ScrapeTarget{Name: "Movies", RootPath: "/movies", LibraryType: "movie", Enabled: true}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ScanRun{TargetID: target.ID, Status: "succeeded"}).Error; err != nil {
		t.Fatal(err)
	}
	settings := NewSettingService(db, cipher, stubTMDBTester{})
	service := NewBatchScrapeService(db, settings, &stubBatchPreviewer{}, &stubBatchJobSubmitter{}, 1, 5)
	_, err = service.StartBatch(target.ID, 7, BatchScrapeCommand{})
	var serviceErr *Error
	if !errors.As(err, &serviceErr) || serviceErr.Code != "tmdb.not_configured" {
		t.Fatalf("expected tmdb.not_configured, got: %#v", err)
	}
}
