package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"oscraper/config"
	"oscraper/internal/logging"
	"oscraper/internal/model"
	"oscraper/internal/openlist"
	"oscraper/internal/provider/tmdb"
	"oscraper/internal/repository"
	"oscraper/pkg/cryptoutil"

	"gorm.io/gorm"
)

type OpenListMutator interface {
	DirectoryBrowser
	CreateDirectory(ctx context.Context, baseURL, token, remotePath string) error
	RenameEntry(ctx context.Context, baseURL, token, sourcePath, newName string) error
	MoveEntries(ctx context.Context, baseURL, token, sourceDirectory, targetDirectory string, names []string) error
	Upload(ctx context.Context, baseURL, token, remotePath, contentType string, size int64, content io.Reader) error
}

var errOperationSkipped = errors.New("operation already applied")

const (
	imageDownloadAttempts = 3
	imageRetryBaseDelay   = 100 * time.Millisecond
)

type SubmitJobCommand struct {
	PreviewID                   uint
	RenameMedia                 bool
	ConfirmDirectoryFingerprint string
}

type JobPage struct {
	Items []model.ScrapeJob `json:"items"`
	Total int64             `json:"total"`
	Page  int               `json:"page"`
	Size  int               `json:"size"`
}

type JobService struct {
	jobs     *repository.JobRepository
	previews *repository.PreviewRepository
	targets  *repository.TargetRepository
	audit    *repository.AuditRepository
	local    *localStorage
	executor *JobExecutor
	rootCtx  context.Context
	cancel   context.CancelFunc
	slots    chan struct{}
	workers  chan struct{}
	wait     sync.WaitGroup
	submitMu sync.Mutex
}

// JobExecutor owns storage mutation and artifact execution. JobService only
// coordinates application use cases and the bounded persistent queue.
type JobExecutor struct {
	jobs        *repository.JobRepository
	previews    *repository.PreviewRepository
	targets     *repository.TargetRepository
	connections *repository.ConnectionRepository
	catalog     CandidateInspector
	audit       *repository.AuditRepository
	cipher      *cryptoutil.Cipher
	client      OpenListMutator
	quota       *ConnectionQuota
	workDir     string
	maxImage    int64
	locks       sync.Map
	imageClient *http.Client
	settings    *SettingService
	local       *localStorage
}

type jobSource struct {
	connection *model.OpenListConnection
	token      string
	local      *localStorage
}

func NewJobService(db *gorm.DB, cfg *config.Config, cipher *cryptoutil.Cipher, client OpenListMutator, catalog CandidateInspector, quota *ConnectionQuota, settings ...*SettingService) (*JobService, error) {
	workers, queueSize, maxImage := cfg.ScrapeWorkers, cfg.ScrapeQueueSize, cfg.MaxImageBytes
	if workers <= 0 {
		workers = 2
	}
	if queueSize <= 0 {
		queueSize = 100
	}
	if maxImage <= 0 {
		maxImage = 20 << 20
	}
	workDir := strings.TrimSpace(cfg.JobWorkDir)
	if workDir == "" {
		workDir = filepath.Join(filepath.Dir(cfg.SQLitePath), "work", "jobs")
	}
	if err := os.MkdirAll(workDir, 0o750); err != nil {
		return nil, err
	}
	cleanupExpiredWorkspaces(workDir, cfg.JobRetentionDays)
	rootCtx, cancel := context.WithCancel(context.Background())
	if quota == nil {
		quota = NewConnectionQuota()
	}
	jobs := repository.NewJobRepository(db)
	previews := repository.NewPreviewRepository(db)
	targets := repository.NewTargetRepository(db)
	audit := repository.NewAuditRepository(db)
	local := newLocalStorage(cfg.LocalMediaRoot)
	executor := &JobExecutor{
		jobs: jobs, previews: previews, targets: targets, connections: repository.NewConnectionRepository(db), catalog: catalog,
		audit: audit, cipher: cipher, client: client, quota: quota, workDir: workDir, maxImage: maxImage, local: local,
	}
	if len(settings) > 0 {
		executor.settings = settings[0]
	}
	executor.imageClient = &http.Client{
		Timeout: time.Duration(cfg.HTTPTimeoutSeconds) * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return openlist.ValidateEndpoint(request.Context(), request.URL)
		},
	}
	service := &JobService{
		jobs: jobs, previews: previews, targets: targets, audit: audit, local: local, executor: executor,
		rootCtx: rootCtx, cancel: cancel, slots: make(chan struct{}, workers+queueSize), workers: make(chan struct{}, workers),
	}
	if recovered, err := service.jobs.RecoverInterrupted(); err != nil {
		cancel()
		return nil, err
	} else if recovered > 0 {
		logging.Warn("job", "interrupted jobs marked retryable", logging.Fields{"count": recovered})
	}
	return service, nil
}

func (s *JobService) Shutdown(ctx context.Context) error {
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

type JobRuntimeStats struct {
	Queued   int `json:"queued"`
	Running  int `json:"running"`
	Capacity int `json:"capacity"`
}

func (s *JobService) Metrics() JobRuntimeStats {
	running := len(s.workers)
	return JobRuntimeStats{Queued: max(0, len(s.slots)-running), Running: running, Capacity: cap(s.slots)}
}

func (s *JobService) Submit(targetID, actorID uint, request SubmitJobCommand, idempotencyKey string) (*model.ScrapeJob, error) {
	key := strings.TrimSpace(idempotencyKey)
	if len(key) > 100 {
		return nil, BadRequest("job.invalid_idempotency_key", "Idempotency-Key is too long")
	}
	s.submitMu.Lock()
	defer s.submitMu.Unlock()
	if key != "" {
		if existing, err := s.jobs.FindIdempotent(actorID, request.PreviewID, key); err == nil {
			return existing, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, Internal("job.lookup_failed", "Failed to check idempotent job", err)
		}
	}
	preview, err := s.previews.Find(request.PreviewID, targetID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("preview.not_found", "Scrape preview not found")
	}
	if err != nil {
		return nil, Internal("preview.lookup_failed", "Failed to load scrape preview", err)
	}
	if time.Now().UTC().After(preview.ExpiresAt) {
		return nil, Conflict("preview.expired", "Scrape preview has expired")
	}
	if request.ConfirmDirectoryFingerprint != preview.Fingerprint {
		return nil, Conflict("job.fingerprint_mismatch", "Confirmed directory fingerprint does not match the preview")
	}
	var plan PreviewPlan
	if err := json.Unmarshal([]byte(preview.PlanJSON), &plan); err != nil {
		return nil, Internal("preview.invalid_snapshot", "Stored scrape plan is invalid", err)
	}
	if !plan.Ready || len(plan.Conflicts) > 0 {
		return nil, Conflict("job.preview_blocked", "Scrape preview contains blocking conflicts")
	}
	hasRename := len(plan.ProposedDirectoryCreates)+len(plan.ProposedDirectoryRenames)+len(plan.ProposedFileRenames) > 0
	if hasRename && !request.RenameMedia {
		return nil, Conflict("job.rename_confirmation_required", "Media rename must be explicitly confirmed")
	}
	target, err := s.targets.Find(targetID)
	if err != nil {
		return nil, NotFound("target.not_found", "Scrape target not found")
	}
	operations, err := buildJobOperations(plan)
	if err != nil {
		return nil, Internal("job.plan_failed", "Failed to build scrape operations", err)
	}
	for _, operation := range operations {
		if operation.SourcePath != "" && !openlist.IsWithinPath(target.RootPath, operation.SourcePath) {
			return nil, Forbidden("job.path_outside_root", "A source operation is outside the scrape target root")
		}
		if !openlist.IsWithinPath(target.RootPath, operation.TargetPath) {
			return nil, Forbidden("job.path_outside_root", "A target operation is outside the scrape target root")
		}
		if sourceType(target) == "local" {
			if operation.SourcePath != "" {
				if _, normalizeErr := s.local.Normalize(operation.SourcePath); normalizeErr != nil {
					return nil, normalizeErr
				}
			}
			if _, normalizeErr := s.local.Normalize(operation.TargetPath); normalizeErr != nil {
				return nil, normalizeErr
			}
		}
	}
	active, err := s.jobs.ActiveCount(targetID, preview.CandidateID)
	if err != nil {
		return nil, Internal("job.active_check_failed", "Failed to check active scrape jobs", err)
	}
	if active > 0 {
		return nil, Conflict("job.already_active", "The target or media candidate already has an active job")
	}
	select {
	case s.slots <- struct{}{}:
	default:
		return nil, TooManyRequests("job.queue_full", "The scrape job queue is full")
	}
	connectionID := uint(0)
	if target.ConnectionID != nil {
		connectionID = *target.ConnectionID
	}
	job := &model.ScrapeJob{
		TargetID: targetID, PreviewID: preview.ID, CandidateID: preview.CandidateID,
		SourceType: sourceType(target), SourceRoot: target.RootPath, ConnectionID: connectionID,
		ActorID: actorID, IdempotencyKey: key, Status: "pending", Stage: "preparing", Message: "Queued", Attempts: 1,
	}
	if err := s.jobs.Create(job, operations); err != nil {
		<-s.slots
		return nil, Internal("job.create_failed", "Failed to create scrape job", err)
	}
	s.recordAudit(actorID, "job.submit", job)
	s.dispatch(job.ID)
	return job, nil
}

func (s *JobService) Get(id uint) (*model.ScrapeJob, error) {
	job, err := s.jobs.Find(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("job.not_found", "Scrape job not found")
	}
	if err != nil {
		return nil, Internal("job.lookup_failed", "Failed to load scrape job", err)
	}
	return job, nil
}

func (s *JobService) List(status string, targetID uint, page, size int) (*JobPage, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" && status != "pending" && status != "running" && status != "succeeded" && status != "failed" && status != "canceled" {
		return nil, BadRequest("job.invalid_status", "Scrape job status is invalid")
	}
	items, total, err := s.jobs.List(status, targetID, page, size)
	if err != nil {
		return nil, Internal("job.list_failed", "Failed to list scrape jobs", err)
	}
	return &JobPage{Items: items, Total: total, Page: page, Size: size}, nil
}

func (s *JobService) Operations(id uint) ([]model.ScrapeJobOperation, error) {
	if _, err := s.Get(id); err != nil {
		return nil, err
	}
	operations, err := s.jobs.Operations(id)
	if err != nil {
		return nil, Internal("job.operation_list_failed", "Failed to list scrape operations", err)
	}
	return operations, nil
}

func (s *JobService) Retry(id, actorID uint) (*model.ScrapeJob, error) {
	s.submitMu.Lock()
	defer s.submitMu.Unlock()
	job, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	active, err := s.jobs.ActiveCount(job.TargetID, job.CandidateID)
	if err != nil {
		return nil, Internal("job.active_check_failed", "Failed to check active scrape jobs", err)
	}
	if active > 0 {
		return nil, Conflict("job.already_active", "The target or media candidate already has an active job")
	}
	select {
	case s.slots <- struct{}{}:
	default:
		return nil, TooManyRequests("job.queue_full", "The scrape job queue is full")
	}
	reset, err := s.jobs.ResetFailed(id)
	if err != nil || !reset {
		<-s.slots
		if err != nil {
			return nil, Internal("job.retry_failed", "Failed to reset scrape job", err)
		}
		return nil, Conflict("job.not_retryable", "Only failed scrape jobs can be retried")
	}
	s.recordAudit(actorID, "job.retry", job)
	s.dispatch(id)
	return s.Get(id)
}

func (s *JobService) Cancel(id, actorID uint) (*model.ScrapeJob, error) {
	canceled, err := s.jobs.CancelPending(id)
	if err != nil {
		return nil, Internal("job.cancel_failed", "Failed to cancel scrape job", err)
	}
	if !canceled {
		if _, lookupErr := s.Get(id); lookupErr != nil {
			return nil, lookupErr
		}
		return nil, Conflict("job.not_cancelable", "Only pending scrape jobs can be canceled")
	}
	job, _ := s.Get(id)
	s.recordAudit(actorID, "job.cancel", job)
	return job, nil
}

func (s *JobService) dispatch(jobID uint) {
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
		claimed, err := s.jobs.Claim(jobID)
		if err != nil || !claimed {
			return
		}
		s.executor.Execute(s.rootCtx, jobID)
	}()
}

func (s *JobExecutor) Execute(ctx context.Context, jobID uint) {
	job, err := s.jobs.Find(jobID)
	if err != nil {
		return
	}
	preview, err := s.previews.Find(job.PreviewID, job.TargetID)
	if err != nil {
		s.fail(job, "job.preview_missing", "Scrape preview no longer exists", err)
		return
	}
	var plan PreviewPlan
	if err := json.Unmarshal([]byte(preview.PlanJSON), &plan); err != nil {
		s.fail(job, "preview.invalid_snapshot", "Stored scrape plan is invalid", err)
		return
	}
	target, err := s.targets.Find(job.TargetID)
	if err != nil {
		s.fail(job, "target.not_found", "Scrape target no longer exists", err)
		return
	}
	if jobSourceType(job) == "openlist" && (job.SourceRoot == "" || job.SourceRoot == "/") {
		job.SourceRoot = target.RootPath
		_ = s.jobs.Save(job)
	}
	if sourceType(target) != jobSourceType(job) || target.RootPath != job.SourceRoot {
		s.fail(job, "job.source_changed", "Scrape target source changed after job submission", nil)
		return
	}
	source, err := s.resolveJobSource(job)
	if err != nil {
		s.failFromError(job, err)
		return
	}
	operations, err := s.jobs.Operations(job.ID)
	if err != nil {
		s.fail(job, "job.operation_list_failed", "Failed to load scrape operations", err)
		return
	}
	if job.Checkpoint == 0 {
		s.updateStage(job, "preparing", 5, "Checking directory fingerprint")
		inspection, inspectErr := s.catalog.InspectCandidate(ctx, job.TargetID, job.CandidateID, true)
		if inspectErr != nil {
			s.failFromError(job, inspectErr)
			return
		}
		if inspection.Stale || inspection.Fingerprint != preview.Fingerprint {
			s.fail(job, "preview.stale", "Media directory changed after preview", nil)
			return
		}
	}
	lock := s.storageLock(job)
	lock.Lock()
	defer lock.Unlock()

	s.updateStage(job, "renaming", 10, "Applying media rename plan")
	for index := range operations {
		if operations[index].Type == "upload" || operations[index].Type == "marker" || operations[index].Status == "succeeded" || operations[index].Status == "skipped" {
			continue
		}
		if err := s.executeMutation(ctx, source, &operations[index]); err != nil {
			s.failOperation(job, &operations[index], err)
			return
		}
		s.checkpoint(job, &operations[index], 10, 55, len(operations))
	}

	s.updateStage(job, "generating", 60, "Preparing immutable metadata artifacts")
	if err := s.prepareArtifacts(ctx, job, source, plan, operations); err != nil {
		s.failFromError(job, err)
		return
	}
	operations, err = s.jobs.Operations(job.ID)
	if err != nil {
		s.fail(job, "job.operation_list_failed", "Failed to load scrape operations", err)
		return
	}
	s.updateStage(job, "uploading", 70, "Uploading metadata artifacts")
	for index := range operations {
		if operations[index].Type != "upload" || operations[index].Status == "succeeded" || operations[index].Status == "skipped" {
			continue
		}
		if err := s.executeUpload(ctx, source, &operations[index]); err != nil {
			s.failOperation(job, &operations[index], err)
			return
		}
		s.checkpoint(job, &operations[index], 70, 90, len(operations))
	}

	s.updateStage(job, "verifying", 95, "Verifying final storage paths")
	operations, err = s.jobs.Operations(job.ID)
	if err != nil {
		s.fail(job, "job.operation_list_failed", "Failed to load scrape operations", err)
		return
	}
	if err := s.verify(ctx, source, plan, operations); err != nil {
		s.failFromError(job, err)
		return
	}

	s.updateStage(job, "marking", 98, "Writing scrape marker")
	for index := range operations {
		if operations[index].Type != "marker" || operations[index].Status == "succeeded" {
			continue
		}
		if err := s.executeMarker(ctx, source, &operations[index]); err != nil {
			s.failOperation(job, &operations[index], err)
			return
		}
		if err := s.verifyMarker(ctx, source, &operations[index]); err != nil {
			s.failOperation(job, &operations[index], err)
			return
		}
		s.checkpoint(job, &operations[index], 98, 99, len(operations))
	}
	now := time.Now().UTC()
	job.Status, job.Stage, job.Progress, job.Message, job.CompletedAt = "succeeded", "completed", 100, "Scrape completed", &now
	job.ErrorCode, job.ErrorMessage = "", ""
	_ = s.jobs.Save(job)
	_ = os.RemoveAll(s.jobWorkspace(job.ID))
	logging.Info("job", "scrape job completed", logging.Fields{"job_id": job.ID, "target_id": job.TargetID})
}

func (s *JobExecutor) executeMutation(ctx context.Context, source *jobSource, operation *model.ScrapeJobOperation) error {
	now := time.Now().UTC()
	operation.Status, operation.Attempts, operation.StartedAt = "running", operation.Attempts+1, &now
	if err := s.jobs.SaveOperation(operation); err != nil {
		return Internal("job.checkpoint_failed", "Failed to save operation checkpoint", err)
	}
	var err error
	switch operation.Type {
	case "mkdir":
		exists, isDir, checkErr := s.entryState(ctx, source, operation.TargetPath)
		if checkErr != nil {
			err = checkErr
		} else if exists && isDir {
			return s.completeOperation(operation, "skipped")
		} else if exists {
			err = Conflict("job.target_exists", "A file occupies the planned directory path")
		} else if source.local != nil {
			err = source.local.CreateDirectory(operation.TargetPath)
		} else if waitErr := s.quota.Wait(ctx, source.connection); waitErr != nil {
			err = waitErr
		} else {
			err = s.client.CreateDirectory(ctx, source.connection.BaseURL, source.token, operation.TargetPath)
		}
	case "rename":
		err = s.ensureMutationState(ctx, source, operation, func() error {
			if source.local != nil {
				return source.local.MoveNoReplace(operation.SourcePath, operation.TargetPath)
			}
			if waitErr := s.quota.Wait(ctx, source.connection); waitErr != nil {
				return waitErr
			}
			return s.client.RenameEntry(ctx, source.connection.BaseURL, source.token, operation.SourcePath, path.Base(operation.TargetPath))
		})
	case "move":
		err = s.ensureMutationState(ctx, source, operation, func() error {
			if source.local != nil {
				return source.local.MoveNoReplace(operation.SourcePath, operation.TargetPath)
			}
			if waitErr := s.quota.Wait(ctx, source.connection); waitErr != nil {
				return waitErr
			}
			return s.client.MoveEntries(ctx, source.connection.BaseURL, source.token, path.Dir(operation.SourcePath), path.Dir(operation.TargetPath), []string{path.Base(operation.SourcePath)})
		})
	default:
		err = BadRequest("job.invalid_operation", "Scrape operation type is invalid")
	}
	if errors.Is(err, errOperationSkipped) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.completeOperation(operation, "succeeded")
}

func (s *JobExecutor) ensureMutationState(ctx context.Context, source *jobSource, operation *model.ScrapeJobOperation, mutate func() error) error {
	sourceExists, _, err := s.entryState(ctx, source, operation.SourcePath)
	if err != nil {
		return err
	}
	targetExists, _, err := s.entryState(ctx, source, operation.TargetPath)
	if err != nil {
		return err
	}
	if targetExists && !sourceExists {
		if err := s.completeOperation(operation, "skipped"); err != nil {
			return err
		}
		return errOperationSkipped
	}
	if targetExists && !(source.local != nil && sourceExists && strings.EqualFold(operation.SourcePath, operation.TargetPath)) {
		return Conflict("job.target_exists", "Storage destination appeared after preview")
	}
	if !sourceExists {
		return Conflict("job.source_missing", "Storage source is missing")
	}
	return mutate()
}

func (s *JobExecutor) completeOperation(operation *model.ScrapeJobOperation, status string) error {
	now := time.Now().UTC()
	operation.Status, operation.CompletedAt, operation.LastError = status, &now, ""
	return s.jobs.SaveOperation(operation)
}

func (s *JobExecutor) skipOperation(operation *model.ScrapeJobOperation, reason string) error {
	now := time.Now().UTC()
	operation.Status, operation.CompletedAt, operation.LastError = "skipped", &now, reason
	return s.jobs.SaveOperation(operation)
}

func (s *JobExecutor) entryState(ctx context.Context, source *jobSource, remotePath string) (bool, bool, error) {
	if source.local != nil {
		return source.local.EntryState(remotePath)
	}
	if err := s.quota.Wait(ctx, source.connection); err != nil {
		return false, false, err
	}
	entries, err := s.client.ListDirectory(ctx, source.connection.BaseURL, source.token, path.Dir(remotePath), true)
	if err != nil {
		return false, false, mapOpenListError(err)
	}
	for _, entry := range entries {
		if entry.Path == remotePath {
			return true, entry.IsDir, nil
		}
	}
	return false, false, nil
}

func (s *JobExecutor) prepareArtifacts(ctx context.Context, job *model.ScrapeJob, source *jobSource, plan PreviewPlan, operations []model.ScrapeJobOperation) error {
	workspace := s.jobWorkspace(job.ID)
	if err := os.MkdirAll(workspace, 0o750); err != nil {
		return Internal("job.workspace_failed", "Failed to prepare job workspace", err)
	}
	for index := range operations {
		operation := &operations[index]
		if operation.Type != "upload" || operation.Status == "succeeded" || operation.Status == "skipped" {
			continue
		}
		skipped, err := s.skipExistingUpload(ctx, source, operation)
		if err != nil {
			return err
		}
		if skipped {
			continue
		}
		artifactIndex := operation.Artifact - 1
		if artifactIndex < 0 || artifactIndex >= len(plan.Artifacts) {
			return Internal("job.invalid_artifact", "Scrape artifact snapshot is invalid", nil)
		}
		artifact := plan.Artifacts[artifactIndex]
		localPath := filepath.Join(workspace, fmt.Sprintf("%03d-%s", operation.Sequence, safeLocalName(path.Base(artifact.Path))))
		contentType := "application/xml; charset=utf-8"
		if isNFOArtifact(artifact.Kind) {
			if err := os.WriteFile(localPath, []byte(artifact.Content), 0o640); err != nil {
				return Internal("job.artifact_failed", "Failed to generate NFO artifact", err)
			}
		} else {
			var err error
			contentType, err = s.downloadImage(ctx, artifact.SourceURL, localPath)
			if err != nil {
				if !shouldSkipImageArtifact(err) {
					return err
				}
				reason := safeErrorMessage(err)
				if saveErr := s.skipOperation(operation, reason); saveErr != nil {
					return Internal("job.checkpoint_failed", "Failed to save skipped image checkpoint", saveErr)
				}
				logging.Warn("job", "TMDB image skipped after download failure", logging.Fields{
					"job_id": job.ID, "target_id": job.TargetID, "artifact_kind": artifact.Kind, "error": err,
				})
				continue
			}
		}
		operation.LocalPath, operation.ContentType = localPath, contentType
		if err := s.jobs.SaveOperation(operation); err != nil {
			return Internal("job.checkpoint_failed", "Failed to save artifact checkpoint", err)
		}
	}
	return nil
}

func (s *JobExecutor) downloadImage(ctx context.Context, rawURL, destination string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= imageDownloadAttempts; attempt++ {
		contentType, err := s.downloadImageOnce(ctx, rawURL, destination)
		if err == nil {
			return contentType, nil
		}
		lastErr = err
		if !isRetryableImageDownloadError(err) || attempt == imageDownloadAttempts {
			return "", err
		}
		timer := time.NewTimer(time.Duration(attempt) * imageRetryBaseDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return "", ctx.Err()
		case <-timer.C:
		}
	}
	return "", lastErr
}

func (s *JobExecutor) downloadImageOnce(ctx context.Context, rawURL, destination string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", BadRequest("job.invalid_image_url", "TMDB image URL is invalid")
	}
	if err := openlist.ValidateEndpoint(ctx, parsed); err != nil {
		return "", BadRequest("job.invalid_image_url", "TMDB image URL is not allowed")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", Internal("job.image_download_failed", "Failed to create image request", err)
	}
	request.Header.Set("Accept", "image/jpeg,image/png,image/webp")
	imageClient := s.imageClient
	if s.settings != nil {
		config, _, configErr := s.settings.TMDBConfig()
		if configErr != nil {
			return "", configErr
		}
		imageClient, configErr = tmdb.HTTPClient(config, s.imageClient.Timeout)
		if configErr != nil {
			return "", mapTMDBError(configErr)
		}
		imageClient.CheckRedirect = s.imageClient.CheckRedirect
	}
	response, err := imageClient.Do(request)
	if err != nil {
		return "", Internal("job.image_download_failed", "Failed to download TMDB image", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", Conflict("job.image_download_failed", fmt.Sprintf("TMDB image returned HTTP %d", response.StatusCode))
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		return "", Conflict("job.invalid_image_type", "TMDB image response has an unsupported content type")
	}
	if response.ContentLength > s.maxImage {
		return "", Conflict("job.image_too_large", "TMDB image exceeds the configured size limit")
	}
	temporary := destination + ".part"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		return "", Internal("job.artifact_failed", "Failed to create image artifact", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, s.maxImage+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || written > s.maxImage {
		_ = os.Remove(temporary)
		if written > s.maxImage {
			return "", Conflict("job.image_too_large", "TMDB image exceeds the configured size limit")
		}
		return "", Internal("job.image_download_failed", "Failed to stream TMDB image", errors.Join(copyErr, closeErr))
	}
	if err := os.Rename(temporary, destination); err != nil {
		_ = os.Remove(temporary)
		return "", Internal("job.artifact_failed", "Failed to finalize image artifact", err)
	}
	return contentType, nil
}

func isRetryableImageDownloadError(err error) bool {
	var serviceError *Error
	return errors.As(err, &serviceError) && serviceError.Code == "job.image_download_failed"
}

func shouldSkipImageArtifact(err error) bool {
	var serviceError *Error
	if !errors.As(err, &serviceError) {
		return false
	}
	switch serviceError.Code {
	case "job.image_download_failed", "job.invalid_image_url", "job.invalid_image_type", "job.image_too_large":
		return true
	default:
		return false
	}
}

func (s *JobExecutor) skipExistingUpload(ctx context.Context, source *jobSource, operation *model.ScrapeJobOperation) (bool, error) {
	exists, isDir, err := s.entryState(ctx, source, operation.TargetPath)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	if isDir {
		return false, Conflict("job.target_exists", "A directory occupies the metadata path")
	}
	if err := s.completeOperation(operation, "skipped"); err != nil {
		return false, Internal("job.checkpoint_failed", "Failed to save skipped upload checkpoint", err)
	}
	return true, nil
}

func (s *JobExecutor) executeUpload(ctx context.Context, source *jobSource, operation *model.ScrapeJobOperation) error {
	skipped, err := s.skipExistingUpload(ctx, source, operation)
	if err != nil {
		return err
	}
	if skipped {
		return nil
	}
	now := time.Now().UTC()
	operation.Status, operation.Attempts, operation.StartedAt = "running", operation.Attempts+1, &now
	if err := s.jobs.SaveOperation(operation); err != nil {
		return Internal("job.checkpoint_failed", "Failed to save upload checkpoint", err)
	}
	file, err := os.Open(operation.LocalPath)
	if err != nil {
		return Internal("job.artifact_missing", "Prepared metadata artifact is missing", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Internal("job.artifact_missing", "Prepared metadata artifact cannot be read", err)
	}
	if source.local != nil {
		if err := source.local.PutMetadata(operation.TargetPath, info.Size(), file); err != nil {
			return err
		}
	} else {
		if err := s.quota.Wait(ctx, source.connection); err != nil {
			return err
		}
		if err := s.client.Upload(ctx, source.connection.BaseURL, source.token, operation.TargetPath, operation.ContentType, info.Size(), file); err != nil {
			return mapOpenListError(err)
		}
	}
	return s.completeOperation(operation, "succeeded")
}

func (s *JobExecutor) executeMarker(ctx context.Context, source *jobSource, operation *model.ScrapeJobOperation) error {
	exists, isDir, err := s.entryState(ctx, source, operation.TargetPath)
	if err != nil {
		return err
	}
	if exists && isDir {
		return Conflict("job.target_exists", "A directory occupies the scrape marker path")
	}
	now := time.Now().UTC()
	operation.Status, operation.Attempts, operation.StartedAt = "running", operation.Attempts+1, &now
	operation.ContentType = scrapeMarkerContentType
	if err := s.jobs.SaveOperation(operation); err != nil {
		return Internal("job.checkpoint_failed", "Failed to save scrape marker checkpoint", err)
	}
	reader := strings.NewReader(scrapeMarkerContent)
	if source.local != nil {
		if err := source.local.PutMetadata(operation.TargetPath, int64(len(scrapeMarkerContent)), reader); err != nil {
			return err
		}
	} else {
		if err := s.quota.Wait(ctx, source.connection); err != nil {
			return err
		}
		if err := s.client.Upload(ctx, source.connection.BaseURL, source.token, operation.TargetPath, scrapeMarkerContentType, int64(len(scrapeMarkerContent)), reader); err != nil {
			return mapOpenListError(err)
		}
	}
	return s.completeOperation(operation, "succeeded")
}

func (s *JobExecutor) verifyMarker(ctx context.Context, source *jobSource, operation *model.ScrapeJobOperation) error {
	exists, isDir, err := s.entryState(ctx, source, operation.TargetPath)
	if err != nil {
		return err
	}
	if !exists || isDir {
		return Conflict("job.marker_verification_failed", "Scrape marker is missing after execution")
	}
	return nil
}

func (s *JobExecutor) verify(ctx context.Context, source *jobSource, plan PreviewPlan, operations []model.ScrapeJobOperation) error {
	expected := make([]string, 0, len(plan.ProposedFileRenames)+len(plan.Artifacts)+1)
	skippedImages := make(map[string]struct{})
	for index := range operations {
		operation := operations[index]
		if operation.Type == "upload" && operation.Status == "skipped" && operation.LastError != "" {
			skippedImages[operation.TargetPath] = struct{}{}
		}
	}
	if plan.ProposedDirectoryPath != "" && plan.ProposedDirectoryPath != path.Dir(plan.SourcePath) {
		expected = append(expected, plan.ProposedDirectoryPath)
	}
	for _, rename := range plan.ProposedFileRenames {
		expected = append(expected, rename.TargetPath)
	}
	for _, artifact := range plan.Artifacts {
		if _, skipped := skippedImages[artifact.Path]; skipped {
			continue
		}
		expected = append(expected, artifact.Path)
	}
	for _, remotePath := range expected {
		exists, _, err := s.entryState(ctx, source, remotePath)
		if err != nil {
			return err
		}
		if !exists {
			return Conflict("job.verification_failed", "An expected storage path is missing after execution")
		}
	}
	return nil
}

func (s *JobExecutor) updateStage(job *model.ScrapeJob, stage string, progress int, message string) {
	job.Stage, job.Progress, job.Message = stage, progress, message
	_ = s.jobs.Save(job)
	logging.Info("job", "scrape job stage", logging.Fields{"job_id": job.ID, "target_id": job.TargetID, "stage": stage, "progress": progress})
}

func (s *JobExecutor) checkpoint(job *model.ScrapeJob, operation *model.ScrapeJobOperation, start, end, total int) {
	job.Checkpoint = operation.Sequence
	if total > 0 {
		job.Progress = start + (end-start)*operation.Sequence/total
	}
	job.Message = "Operation " + strconv.Itoa(operation.Sequence) + " completed"
	_ = s.jobs.Save(job)
}

func (s *JobExecutor) failOperation(job *model.ScrapeJob, operation *model.ScrapeJobOperation, err error) {
	operation.Status, operation.LastError = "failed", safeErrorMessage(err)
	now := time.Now().UTC()
	operation.CompletedAt = &now
	_ = s.jobs.SaveOperation(operation)
	s.failFromError(job, err)
}

func (s *JobExecutor) failFromError(job *model.ScrapeJob, err error) {
	var serviceError *Error
	if errors.As(err, &serviceError) {
		s.fail(job, serviceError.Code, serviceError.Message, serviceError.Cause)
		return
	}
	var openListError *openlist.APIError
	if errors.As(err, &openListError) {
		s.fail(job, openListError.Code, openListError.Message, openListError.Cause)
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		s.fail(job, "job.interrupted", "Scrape job was interrupted", err)
		return
	}
	s.fail(job, "job.execution_failed", "Scrape job execution failed", err)
}

func (s *JobExecutor) fail(job *model.ScrapeJob, code, message string, cause error) {
	now := time.Now().UTC()
	job.Status, job.ErrorCode, job.ErrorMessage, job.Message, job.CompletedAt = "failed", code, message, message, &now
	_ = s.jobs.Save(job)
	fields := logging.Fields{"job_id": job.ID, "target_id": job.TargetID, "error_code": code}
	if cause != nil {
		fields["cause"] = safeErrorMessage(cause)
	}
	logging.Error("job", "scrape job failed", fields)
}

func (s *JobService) recordAudit(actorID uint, action string, job *model.ScrapeJob) {
	detail, _ := json.Marshal(map[string]any{"job_id": job.ID, "preview_id": job.PreviewID, "target_id": job.TargetID})
	_ = s.audit.Record(actorID, action, "scrape_job:"+strconv.Itoa(int(job.ID)), string(detail))
}

func (s *JobExecutor) storageLock(job *model.ScrapeJob) *sync.Mutex {
	key := "local:" + s.local.root
	if jobSourceType(job) == "openlist" {
		key = "openlist:" + strconv.FormatUint(uint64(job.ConnectionID), 10)
	}
	value, _ := s.locks.LoadOrStore(key, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func jobSourceType(job *model.ScrapeJob) string {
	if strings.ToLower(strings.TrimSpace(job.SourceType)) == "local" {
		return "local"
	}
	return "openlist"
}

func (s *JobExecutor) resolveJobSource(job *model.ScrapeJob) (*jobSource, error) {
	if jobSourceType(job) == "local" {
		if _, err := s.local.Normalize(job.SourceRoot); err != nil {
			return nil, err
		}
		return &jobSource{local: s.local}, nil
	}
	connection, err := s.connections.Find(job.ConnectionID)
	if err != nil || !connection.Enabled {
		return nil, Conflict("target.connection_disabled", "OpenList connection is unavailable")
	}
	token, err := s.cipher.Decrypt(connection.EncryptedToken)
	if err != nil {
		return nil, Internal("connection.decryption_failed", "Stored OpenList token cannot be decrypted", err)
	}
	return &jobSource{connection: connection, token: token}, nil
}

func (s *JobExecutor) jobWorkspace(jobID uint) string {
	return filepath.Join(s.workDir, strconv.FormatUint(uint64(jobID), 10))
}

func safeLocalName(value string) string {
	value = strings.Map(func(character rune) rune {
		if character < 32 || strings.ContainsRune("/\\:", character) {
			return '_'
		}
		return character
	}, value)
	if value == "" || value == "." || value == ".." {
		return "artifact"
	}
	return value
}

func safeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 1000 {
		message = message[:1000]
	}
	return message
}

func cleanupExpiredWorkspaces(root string, retentionDays int) {
	if retentionDays <= 0 {
		retentionDays = 7
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if _, err := strconv.ParseUint(entry.Name(), 10, 64); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(root, entry.Name()))
	}
}
