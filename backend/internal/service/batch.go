package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"oscraper/internal/logging"
	"oscraper/internal/model"
	"oscraper/internal/provider/tmdb"
	"oscraper/internal/repository"

	"gorm.io/gorm"
)

const (
	batchRateLimitRetry = 2
	batchRateLimitFuse  = 5
)

// Batches run strictly sequentially: the per-item delay plus bounded retries
// keep the loop polite to TMDB and the OpenList read quota. Variables so tests
// can shrink the waits.
var (
	batchItemDelay       = 500 * time.Millisecond
	batchRateLimitDelay  = 2 * time.Second
	batchQueueWait       = 60 * time.Second
	batchQueuePollPeriod = 2 * time.Second
)

type BatchPreviewer interface {
	Search(ctx context.Context, targetID uint, request SearchPreviewCommand) ([]tmdb.SearchResult, error)
	Create(ctx context.Context, targetID, actorID uint, request CreatePreviewCommand) (*PreviewResponse, error)
}

type BatchJobSubmitter interface {
	Submit(targetID, actorID uint, request SubmitJobCommand, idempotencyKey string) (*model.ScrapeJob, error)
}

type BatchScrapeCommand struct {
	ScanID         uint
	IncludeScraped bool
}

type BatchScrapeResponse struct {
	model.ScrapeBatchRun
	Items []model.ScrapeBatchItem `json:"items"`
}

type BatchRuntimeStats struct {
	Queued   int `json:"queued"`
	Running  int `json:"running"`
	Capacity int `json:"capacity"`
}

type BatchScrapeService struct {
	batches   *repository.BatchRepository
	catalog   *repository.CatalogRepository
	targets   *repository.TargetRepository
	audit     *repository.AuditRepository
	settings  *SettingService
	previews  BatchPreviewer
	jobs      BatchJobSubmitter
	rootCtx   context.Context
	cancel    context.CancelFunc
	slots     chan struct{}
	workers   chan struct{}
	wait      sync.WaitGroup
	submitMu  sync.Mutex
	startOnce sync.Once
	startErr  error
}

func NewBatchScrapeService(db *gorm.DB, settings *SettingService, previews BatchPreviewer, jobs BatchJobSubmitter, workers, queueSize int) *BatchScrapeService {
	if workers < 1 {
		workers = 1
	}
	if queueSize < 1 {
		queueSize = 5
	}
	rootCtx, cancel := context.WithCancel(context.Background())
	return &BatchScrapeService{
		batches: repository.NewBatchRepository(db), catalog: repository.NewCatalogRepository(db),
		targets: repository.NewTargetRepository(db), audit: repository.NewAuditRepository(db),
		settings: settings, previews: previews, jobs: jobs,
		rootCtx: rootCtx, cancel: cancel, slots: make(chan struct{}, workers+queueSize), workers: make(chan struct{}, workers),
	}
}

func (s *BatchScrapeService) Start() error {
	s.startOnce.Do(func() {
		var batches []model.ScrapeBatchRun
		batches, s.startErr = s.batches.RecoverInterruptedBatches()
		if s.startErr != nil {
			return
		}
		for index := range batches {
			batch := batches[index]
			select {
			case s.slots <- struct{}{}:
				s.dispatchBatch(batch.ID, batch.TargetID)
			default:
				_ = s.batches.CompleteBatch(batch.ID, "failed", "batch.queue_recovery_full", "The recovered batch queue exceeds its configured capacity")
			}
		}
		if len(batches) > 0 {
			logging.Warn("batch", "interrupted batches recovered", logging.Fields{"count": len(batches)})
		}
	})
	return s.startErr
}

func (s *BatchScrapeService) Shutdown(ctx context.Context) error {
	s.cancel()
	done := make(chan struct{})
	go func() { s.wait.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *BatchScrapeService) Metrics() BatchRuntimeStats {
	running := len(s.workers)
	return BatchRuntimeStats{Queued: max(0, len(s.slots)-running), Running: running, Capacity: cap(s.slots)}
}

func (s *BatchScrapeService) StartBatch(targetID, actorID uint, command BatchScrapeCommand) (*BatchScrapeResponse, error) {
	if err := s.Start(); err != nil {
		return nil, Internal("batch.runtime_failed", "Failed to start scrape batch runtime", err)
	}
	target, err := s.targets.Find(targetID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("target.not_found", "Scrape target not found")
	}
	if err != nil {
		return nil, Internal("target.lookup_failed", "Failed to load scrape target", err)
	}
	if !target.Enabled {
		return nil, Conflict("target.disabled", "The scrape target is disabled")
	}
	s.submitMu.Lock()
	defer s.submitMu.Unlock()
	active, err := s.batches.ActiveBatchCount(targetID)
	if err != nil {
		return nil, Internal("batch.active_check_failed", "Failed to check active scrape batches", err)
	}
	if active > 0 {
		return nil, Conflict("batch.already_running", "A scrape batch is already running for this target")
	}
	if _, hasKey, err := s.settings.TMDBConfig(); err != nil {
		return nil, Internal("settings.tmdb_failed", "Failed to load TMDB settings", err)
	} else if !hasKey {
		return nil, Conflict("tmdb.not_configured", "TMDB API key is not configured")
	}
	scan, candidates, err := s.resolveCandidates(targetID, command.ScanID, command.IncludeScraped)
	if err != nil {
		return nil, err
	}
	items := make([]model.ScrapeBatchItem, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, model.ScrapeBatchItem{CandidateID: candidate.ID, Path: candidate.Path, Status: "pending"})
	}
	batch := &model.ScrapeBatchRun{
		TargetID: targetID, ActorID: actorID, ScanID: scan.ID, Status: "pending",
		TotalCount: len(items), IncludeScraped: command.IncludeScraped,
	}
	select {
	case s.slots <- struct{}{}:
	default:
		return nil, TooManyRequests("batch.queue_full", "The scrape batch queue is full")
	}
	if err := s.batches.CreateBatch(batch, items); err != nil {
		<-s.slots
		return nil, Internal("batch.create_failed", "Failed to create scrape batch", err)
	}
	auditDetail, _ := json.Marshal(map[string]any{"batch_id": batch.ID, "scan_id": scan.ID, "total": batch.TotalCount, "include_scraped": batch.IncludeScraped})
	_ = s.audit.Record(actorID, "batch.create", fmt.Sprintf("scrape_target:%d", targetID), string(auditDetail))
	s.dispatchBatch(batch.ID, targetID)
	return s.batchResponse(batch.ID, targetID)
}

func (s *BatchScrapeService) GetBatch(targetID, batchID uint) (*BatchScrapeResponse, error) {
	if _, err := s.batches.FindBatch(batchID, targetID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NotFound("batch.not_found", "Scrape batch not found")
		}
		return nil, Internal("batch.lookup_failed", "Failed to load scrape batch", err)
	}
	return s.batchResponse(batchID, targetID)
}

func (s *BatchScrapeService) Cancel(targetID, batchID, actorID uint) (*BatchScrapeResponse, error) {
	s.submitMu.Lock()
	defer s.submitMu.Unlock()
	batch, err := s.batches.FindBatch(batchID, targetID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("batch.not_found", "Scrape batch not found")
	}
	if err != nil {
		return nil, Internal("batch.lookup_failed", "Failed to load scrape batch", err)
	}
	if batch.Status != "pending" && batch.Status != "running" {
		return nil, Conflict("batch.not_cancelable", "Only pending or running scrape batches can be canceled")
	}
	canceled, err := s.batches.CancelBatch(batchID)
	if err != nil {
		return nil, Internal("batch.cancel_failed", "Failed to cancel scrape batch", err)
	}
	if !canceled {
		return nil, Conflict("batch.not_cancelable", "Only pending or running scrape batches can be canceled")
	}
	_, _ = s.batches.SkipPendingItems(batchID, "canceled", "The batch was canceled")
	s.recountProgress(batchID)
	auditDetail, _ := json.Marshal(map[string]any{"batch_id": batchID})
	_ = s.audit.Record(actorID, "batch.cancel", fmt.Sprintf("scrape_target:%d", targetID), string(auditDetail))
	return s.batchResponse(batchID, targetID)
}

func (s *BatchScrapeService) resolveCandidates(targetID, scanID uint, includeScraped bool) (*model.ScanRun, []model.MediaCandidate, error) {
	var scan *model.ScanRun
	var err error
	if scanID > 0 {
		scan, err = s.catalog.FindScan(scanID, targetID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, NotFound("scan.not_found", "Scan run not found")
		}
		if err != nil {
			return nil, nil, Internal("scan.lookup_failed", "Failed to load scan run", err)
		}
	} else {
		scan, err = s.catalog.LatestSucceededScan(targetID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, Conflict("batch.scan_missing", "Run a media scan before starting a batch scrape")
		}
		if err != nil {
			return nil, nil, Internal("scan.lookup_failed", "Failed to load scan run", err)
		}
	}
	if scan.Status != "succeeded" {
		return nil, nil, Conflict("batch.scan_not_succeeded", "The scan run has not completed successfully")
	}
	candidates, err := s.catalog.Candidates(targetID, scan.ID)
	if err != nil {
		return nil, nil, Internal("candidate.list_failed", "Failed to list media candidates", err)
	}
	filtered := make([]model.MediaCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if includeScraped || !candidate.Scraped {
			filtered = append(filtered, candidate)
		}
	}
	if len(filtered) == 0 {
		return nil, nil, Conflict("batch.no_candidates", "No media candidates to scrape")
	}
	return scan, filtered, nil
}

func (s *BatchScrapeService) dispatchBatch(batchID, targetID uint) {
	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
		defer func() { <-s.slots }()
		select {
		case s.workers <- struct{}{}:
			defer func() { <-s.workers }()
		case <-s.rootCtx.Done():
			return
		}
		claimed, err := s.batches.ClaimBatch(batchID)
		if err != nil || !claimed {
			if err != nil {
				logging.Error("batch", "failed to claim batch", logging.Fields{"batch_id": batchID, "error": err})
			}
			return
		}
		s.runBatch(batchID, targetID)
	}()
}

type batchItemResult struct {
	status      string
	skipReason  string
	detail      string
	tmdbID      *int
	jobID       *uint
	rateLimited bool
}

func (s *BatchScrapeService) runBatch(batchID, targetID uint) {
	ctx := s.rootCtx
	batch, err := s.batches.FindBatch(batchID, targetID)
	if err != nil {
		_ = s.batches.CompleteBatch(batchID, "failed", "batch.load_failed", "Failed to load the scrape batch for execution")
		return
	}
	items, err := s.batches.Items(batchID)
	if err != nil {
		_ = s.batches.CompleteBatch(batchID, "failed", "batch.load_failed", "Failed to load the scrape batch items")
		return
	}
	var submitted, skipped, failed int
	rateLimitedInARow := 0
	for index := range items {
		if current, lookupErr := s.batches.FindBatch(batchID, targetID); lookupErr == nil && current.Status != "running" && current.Status != "pending" {
			_, _ = s.batches.SkipPendingItems(batchID, "canceled", "The batch was canceled")
			s.recountProgress(batchID)
			return
		}
		if ctx.Err() != nil {
			_, _ = s.batches.SkipPendingItems(batchID, "canceled", "The service is shutting down")
			_ = s.batches.CompleteBatch(batchID, "failed", "batch.interrupted", "The scrape batch was interrupted by a service shutdown")
			return
		}
		if index > 0 && !sleepContext(ctx, batchItemDelay) {
			_, _ = s.batches.SkipPendingItems(batchID, "canceled", "The service is shutting down")
			_ = s.batches.CompleteBatch(batchID, "failed", "batch.interrupted", "The scrape batch was interrupted by a service shutdown")
			return
		}
		item := items[index]
		result := s.processItem(ctx, batch, &item)
		item.Status, item.SkipReason, item.Detail, item.TMDBID, item.JobID = result.status, result.skipReason, result.detail, result.tmdbID, result.jobID
		if err := s.batches.SaveItem(&item); err != nil {
			logging.Error("batch", "failed to save batch item", logging.Fields{"batch_id": batchID, "item_id": item.ID, "error": err})
		}
		switch result.status {
		case "submitted":
			submitted++
			rateLimitedInARow = 0
		case "skipped":
			skipped++
			rateLimitedInARow = 0
		default:
			failed++
			if result.rateLimited {
				rateLimitedInARow++
			} else {
				rateLimitedInARow = 0
			}
		}
		_ = s.batches.UpdateBatchProgress(batchID, submitted, skipped, failed)
		if rateLimitedInARow >= batchRateLimitFuse {
			_, _ = s.batches.SkipPendingItems(batchID, "canceled", "The batch stopped after repeated TMDB rate limits")
			_ = s.batches.CompleteBatch(batchID, "failed", "batch.rate_limited", "The batch stopped after repeated TMDB rate limits")
			return
		}
	}
	// The status guard inside CompleteBatch keeps a concurrently canceled batch canceled.
	_ = s.batches.CompleteBatch(batchID, "succeeded", "", "")
}

func (s *BatchScrapeService) processItem(ctx context.Context, batch *model.ScrapeBatchRun, item *model.ScrapeBatchItem) batchItemResult {
	candidate, err := s.catalog.FindCandidate(item.CandidateID, batch.TargetID)
	if err != nil {
		return batchItemResult{status: "failed", detail: "Media candidate no longer exists"}
	}
	tmdbID := 0
	if candidate.TMDBID != nil {
		tmdbID = *candidate.TMDBID
	} else {
		results, err := s.searchWithRetry(ctx, batch, candidate)
		if err != nil {
			return batchItemResult{status: "failed", detail: errorDetail(err), rateLimited: isRateLimited(err)}
		}
		switch {
		case len(results) == 0:
			return batchItemResult{status: "skipped", skipReason: "no_match", detail: "TMDB did not return any results"}
		case len(results) > 1:
			return batchItemResult{status: "skipped", skipReason: "multiple_matches", detail: fmt.Sprintf("TMDB returned %d results", len(results))}
		}
		tmdbID = results[0].ID
	}
	preview, err := s.createPreviewWithRetry(ctx, batch, item, tmdbID)
	if err != nil {
		if code, serviceErr := errorCodeOf(err); serviceErr && code == "preview.stale" {
			return batchItemResult{status: "skipped", skipReason: "stale", detail: "Media directory changed after the scan", tmdbID: intPointer(tmdbID)}
		}
		return batchItemResult{status: "failed", detail: errorDetail(err), tmdbID: intPointer(tmdbID), rateLimited: isRateLimited(err)}
	}
	if !preview.Plan.Ready || len(preview.Plan.Conflicts) > 0 {
		return batchItemResult{status: "skipped", skipReason: "plan_conflicts", detail: "The scrape plan has blocking conflicts", tmdbID: &preview.Match.ID}
	}
	plan := preview.Plan
	renameMedia := len(plan.ProposedDirectoryCreates)+len(plan.ProposedDirectoryRenames)+len(plan.ProposedFileRenames) > 0
	job, err := s.submitWithRetry(batch, item, preview, renameMedia)
	if err != nil {
		if code, serviceErr := errorCodeOf(err); serviceErr && code == "job.already_active" {
			return batchItemResult{status: "skipped", skipReason: "already_active", detail: "The media already has an active scrape job", tmdbID: &preview.Match.ID}
		}
		return batchItemResult{status: "failed", detail: errorDetail(err), tmdbID: &preview.Match.ID, rateLimited: isRateLimited(err)}
	}
	jobID := job.ID
	return batchItemResult{status: "submitted", tmdbID: &preview.Match.ID, jobID: &jobID}
}

func (s *BatchScrapeService) searchWithRetry(ctx context.Context, batch *model.ScrapeBatchRun, candidate *model.MediaCandidate) ([]tmdb.SearchResult, error) {
	command := SearchPreviewCommand{CandidateID: candidate.ID, Title: candidate.ParsedTitle, Year: candidateYear(candidate)}
	var results []tmdb.SearchResult
	var err error
	for attempt := 0; ; attempt++ {
		results, err = s.previews.Search(ctx, batch.TargetID, command)
		if !isRateLimited(err) || attempt >= batchRateLimitRetry {
			return results, err
		}
		if !sleepContext(ctx, batchRateLimitDelay*time.Duration(attempt+1)) {
			return nil, err
		}
	}
}

func (s *BatchScrapeService) createPreviewWithRetry(ctx context.Context, batch *model.ScrapeBatchRun, item *model.ScrapeBatchItem, tmdbID int) (*PreviewResponse, error) {
	command := CreatePreviewCommand{CandidateID: item.CandidateID, TMDBID: tmdbID}
	var preview *PreviewResponse
	var err error
	for attempt := 0; ; attempt++ {
		preview, err = s.previews.Create(ctx, batch.TargetID, batch.ActorID, command)
		if !isRateLimited(err) || attempt >= batchRateLimitRetry {
			return preview, err
		}
		if !sleepContext(ctx, batchRateLimitDelay*time.Duration(attempt+1)) {
			return nil, err
		}
	}
}

func (s *BatchScrapeService) submitWithRetry(batch *model.ScrapeBatchRun, item *model.ScrapeBatchItem, preview *PreviewResponse, renameMedia bool) (*model.ScrapeJob, error) {
	command := SubmitJobCommand{PreviewID: preview.ID, RenameMedia: renameMedia, ConfirmDirectoryFingerprint: preview.Fingerprint}
	idempotencyKey := fmt.Sprintf("batch-%d-%d", batch.ID, item.ID)
	deadline := time.Now().Add(batchQueueWait)
	for {
		job, err := s.jobs.Submit(batch.TargetID, batch.ActorID, command, idempotencyKey)
		var serviceErr *Error
		queueFull := errors.As(err, &serviceErr) && serviceErr.Code == "job.queue_full"
		if err == nil || !queueFull || time.Now().After(deadline) {
			return job, err
		}
		if !sleepContext(s.rootCtx, batchQueuePollPeriod) {
			return nil, err
		}
	}
}

func (s *BatchScrapeService) recountProgress(batchID uint) {
	items, err := s.batches.Items(batchID)
	if err != nil {
		return
	}
	var submitted, skipped, failed int
	for _, item := range items {
		switch item.Status {
		case "submitted":
			submitted++
		case "skipped":
			skipped++
		case "failed":
			failed++
		}
	}
	_ = s.batches.UpdateBatchProgress(batchID, submitted, skipped, failed)
}

func (s *BatchScrapeService) batchResponse(batchID, targetID uint) (*BatchScrapeResponse, error) {
	batch, err := s.batches.FindBatch(batchID, targetID)
	if err != nil {
		return nil, Internal("batch.lookup_failed", "Failed to load scrape batch", err)
	}
	items, err := s.batches.Items(batchID)
	if err != nil {
		return nil, Internal("batch.item_list_failed", "Failed to list scrape batch items", err)
	}
	return &BatchScrapeResponse{ScrapeBatchRun: *batch, Items: items}, nil
}

func candidateYear(candidate *model.MediaCandidate) int {
	if candidate.Year != nil {
		return *candidate.Year
	}
	return 0
}

func intPointer(value int) *int { return &value }

func isRateLimited(err error) bool {
	code, ok := errorCodeOf(err)
	return ok && code == "tmdb.rate_limited"
}

func errorCodeOf(err error) (string, bool) {
	var serviceErr *Error
	if errors.As(err, &serviceErr) {
		return serviceErr.Code, true
	}
	return "", false
}

func errorDetail(err error) string {
	var serviceErr *Error
	if errors.As(err, &serviceErr) {
		return serviceErr.Code + ": " + serviceErr.Message
	}
	return err.Error()
}

func sleepContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
