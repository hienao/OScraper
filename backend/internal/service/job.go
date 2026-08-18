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

	"openlistscraper/config"
	"openlistscraper/internal/logging"
	"openlistscraper/internal/model"
	"openlistscraper/internal/openlist"
	"openlistscraper/internal/repository"
	"openlistscraper/pkg/cryptoutil"

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

type SubmitJobRequest struct {
	PreviewID                   uint   `json:"preview_id" binding:"required"`
	RenameMedia                 bool   `json:"rename_media"`
	ConfirmDirectoryFingerprint string `json:"confirm_directory_fingerprint" binding:"required,max=80"`
}

type JobPage struct {
	Items []model.ScrapeJob `json:"items"`
	Total int64             `json:"total"`
	Page  int               `json:"page"`
	Size  int               `json:"size"`
}

type JobService struct {
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
	rootCtx     context.Context
	cancel      context.CancelFunc
	slots       chan struct{}
	workers     chan struct{}
	wait        sync.WaitGroup
	submitMu    sync.Mutex
	locks       sync.Map
	imageClient *http.Client
}

func NewJobService(db *gorm.DB, cfg *config.Config, cipher *cryptoutil.Cipher, client OpenListMutator, catalog CandidateInspector, quota *ConnectionQuota) (*JobService, error) {
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
	service := &JobService{
		jobs: repository.NewJobRepository(db), previews: repository.NewPreviewRepository(db), targets: repository.NewTargetRepository(db),
		connections: repository.NewConnectionRepository(db), catalog: catalog, audit: repository.NewAuditRepository(db), cipher: cipher,
		client: client, quota: quota, workDir: workDir, maxImage: maxImage, rootCtx: rootCtx, cancel: cancel,
		slots: make(chan struct{}, workers+queueSize), workers: make(chan struct{}, workers),
	}
	service.imageClient = &http.Client{
		Timeout: time.Duration(cfg.HTTPTimeoutSeconds) * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return openlist.ValidateEndpoint(request.Context(), request.URL)
		},
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

func (s *JobService) Submit(targetID, actorID uint, request SubmitJobRequest, idempotencyKey string) (*model.ScrapeJob, error) {
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
	job := &model.ScrapeJob{
		TargetID: targetID, PreviewID: preview.ID, CandidateID: preview.CandidateID, ConnectionID: target.ConnectionID,
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
		s.run(s.rootCtx, jobID)
	}()
}

func (s *JobService) run(ctx context.Context, jobID uint) {
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
	_, err = s.targets.Find(job.TargetID)
	if err != nil {
		s.fail(job, "target.not_found", "Scrape target no longer exists", err)
		return
	}
	connection, err := s.connections.Find(job.ConnectionID)
	if err != nil || !connection.Enabled {
		s.fail(job, "target.connection_disabled", "OpenList connection is unavailable", err)
		return
	}
	token, err := s.cipher.Decrypt(connection.EncryptedToken)
	if err != nil {
		s.fail(job, "connection.decryption_failed", "Stored OpenList token cannot be decrypted", err)
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
	lock := s.connectionLock(connection.ID)
	lock.Lock()
	defer lock.Unlock()

	s.updateStage(job, "renaming", 10, "Applying media rename plan")
	for index := range operations {
		if operations[index].Type == "upload" || operations[index].Status == "succeeded" || operations[index].Status == "skipped" {
			continue
		}
		if err := s.executeMutation(ctx, connection, token, &operations[index]); err != nil {
			s.failOperation(job, &operations[index], err)
			return
		}
		s.checkpoint(job, &operations[index], 10, 55, len(operations))
	}

	s.updateStage(job, "generating", 60, "Preparing immutable metadata artifacts")
	if err := s.prepareArtifacts(ctx, job, plan, operations); err != nil {
		s.failFromError(job, err)
		return
	}
	operations, _ = s.jobs.Operations(job.ID)
	s.updateStage(job, "uploading", 70, "Uploading metadata artifacts")
	for index := range operations {
		if operations[index].Type != "upload" || operations[index].Status == "succeeded" || operations[index].Status == "skipped" {
			continue
		}
		if err := s.executeUpload(ctx, connection, token, &operations[index]); err != nil {
			s.failOperation(job, &operations[index], err)
			return
		}
		s.checkpoint(job, &operations[index], 70, 90, len(operations))
	}

	s.updateStage(job, "verifying", 95, "Verifying final OpenList paths")
	if err := s.verify(ctx, connection, token, plan); err != nil {
		s.failFromError(job, err)
		return
	}
	now := time.Now().UTC()
	job.Status, job.Stage, job.Progress, job.Message, job.CompletedAt = "succeeded", "completed", 100, "Scrape completed", &now
	job.ErrorCode, job.ErrorMessage = "", ""
	_ = s.jobs.Save(job)
	_ = os.RemoveAll(s.jobWorkspace(job.ID))
	logging.Info("job", "scrape job completed", logging.Fields{"job_id": job.ID, "target_id": job.TargetID})
}

func (s *JobService) executeMutation(ctx context.Context, connection *model.OpenListConnection, token string, operation *model.ScrapeJobOperation) error {
	now := time.Now().UTC()
	operation.Status, operation.Attempts, operation.StartedAt = "running", operation.Attempts+1, &now
	if err := s.jobs.SaveOperation(operation); err != nil {
		return Internal("job.checkpoint_failed", "Failed to save operation checkpoint", err)
	}
	var err error
	switch operation.Type {
	case "mkdir":
		exists, isDir, checkErr := s.entryState(ctx, connection, token, operation.TargetPath)
		if checkErr != nil {
			err = checkErr
		} else if exists && isDir {
			return s.completeOperation(operation, "skipped")
		} else if exists {
			err = Conflict("job.target_exists", "A file occupies the planned directory path")
		} else if waitErr := s.quota.Wait(ctx, connection); waitErr != nil {
			err = waitErr
		} else {
			err = s.client.CreateDirectory(ctx, connection.BaseURL, token, operation.TargetPath)
		}
	case "rename":
		err = s.ensureMutationState(ctx, connection, token, operation, func() error {
			if waitErr := s.quota.Wait(ctx, connection); waitErr != nil {
				return waitErr
			}
			return s.client.RenameEntry(ctx, connection.BaseURL, token, operation.SourcePath, path.Base(operation.TargetPath))
		})
	case "move":
		err = s.ensureMutationState(ctx, connection, token, operation, func() error {
			if waitErr := s.quota.Wait(ctx, connection); waitErr != nil {
				return waitErr
			}
			return s.client.MoveEntries(ctx, connection.BaseURL, token, path.Dir(operation.SourcePath), path.Dir(operation.TargetPath), []string{path.Base(operation.SourcePath)})
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

func (s *JobService) ensureMutationState(ctx context.Context, connection *model.OpenListConnection, token string, operation *model.ScrapeJobOperation, mutate func() error) error {
	sourceExists, _, err := s.entryState(ctx, connection, token, operation.SourcePath)
	if err != nil {
		return err
	}
	targetExists, _, err := s.entryState(ctx, connection, token, operation.TargetPath)
	if err != nil {
		return err
	}
	if targetExists && !sourceExists {
		if err := s.completeOperation(operation, "skipped"); err != nil {
			return err
		}
		return errOperationSkipped
	}
	if targetExists {
		return Conflict("job.target_exists", "OpenList destination appeared after preview")
	}
	if !sourceExists {
		return Conflict("job.source_missing", "OpenList source is missing")
	}
	return mutate()
}

func (s *JobService) completeOperation(operation *model.ScrapeJobOperation, status string) error {
	now := time.Now().UTC()
	operation.Status, operation.CompletedAt, operation.LastError = status, &now, ""
	return s.jobs.SaveOperation(operation)
}

func (s *JobService) entryState(ctx context.Context, connection *model.OpenListConnection, token, remotePath string) (bool, bool, error) {
	if err := s.quota.Wait(ctx, connection); err != nil {
		return false, false, err
	}
	entries, err := s.client.ListDirectory(ctx, connection.BaseURL, token, path.Dir(remotePath), true)
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

func (s *JobService) prepareArtifacts(ctx context.Context, job *model.ScrapeJob, plan PreviewPlan, operations []model.ScrapeJobOperation) error {
	workspace := s.jobWorkspace(job.ID)
	if err := os.MkdirAll(workspace, 0o750); err != nil {
		return Internal("job.workspace_failed", "Failed to prepare job workspace", err)
	}
	for index := range operations {
		operation := &operations[index]
		if operation.Type != "upload" || operation.Status == "succeeded" || operation.Status == "skipped" {
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
				return err
			}
		}
		operation.LocalPath, operation.ContentType = localPath, contentType
		if err := s.jobs.SaveOperation(operation); err != nil {
			return Internal("job.checkpoint_failed", "Failed to save artifact checkpoint", err)
		}
	}
	return nil
}

func (s *JobService) downloadImage(ctx context.Context, rawURL, destination string) (string, error) {
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
	response, err := s.imageClient.Do(request)
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

func (s *JobService) executeUpload(ctx context.Context, connection *model.OpenListConnection, token string, operation *model.ScrapeJobOperation) error {
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
	if err := s.quota.Wait(ctx, connection); err != nil {
		return err
	}
	if err := s.client.Upload(ctx, connection.BaseURL, token, operation.TargetPath, operation.ContentType, info.Size(), file); err != nil {
		return mapOpenListError(err)
	}
	return s.completeOperation(operation, "succeeded")
}

func (s *JobService) verify(ctx context.Context, connection *model.OpenListConnection, token string, plan PreviewPlan) error {
	expected := make([]string, 0, len(plan.ProposedFileRenames)+len(plan.Artifacts)+1)
	if plan.ProposedDirectoryPath != "" && plan.ProposedDirectoryPath != path.Dir(plan.SourcePath) {
		expected = append(expected, plan.ProposedDirectoryPath)
	}
	for _, rename := range plan.ProposedFileRenames {
		expected = append(expected, rename.TargetPath)
	}
	for _, artifact := range plan.Artifacts {
		expected = append(expected, artifact.Path)
	}
	for _, remotePath := range expected {
		exists, _, err := s.entryState(ctx, connection, token, remotePath)
		if err != nil {
			return err
		}
		if !exists {
			return Conflict("job.verification_failed", "An expected OpenList path is missing after execution")
		}
	}
	return nil
}

func (s *JobService) updateStage(job *model.ScrapeJob, stage string, progress int, message string) {
	job.Stage, job.Progress, job.Message = stage, progress, message
	_ = s.jobs.Save(job)
	logging.Info("job", "scrape job stage", logging.Fields{"job_id": job.ID, "target_id": job.TargetID, "stage": stage, "progress": progress})
}

func (s *JobService) checkpoint(job *model.ScrapeJob, operation *model.ScrapeJobOperation, start, end, total int) {
	job.Checkpoint = operation.Sequence
	if total > 0 {
		job.Progress = start + (end-start)*operation.Sequence/total
	}
	job.Message = "Operation " + strconv.Itoa(operation.Sequence) + " completed"
	_ = s.jobs.Save(job)
}

func (s *JobService) failOperation(job *model.ScrapeJob, operation *model.ScrapeJobOperation, err error) {
	operation.Status, operation.LastError = "failed", safeErrorMessage(err)
	now := time.Now().UTC()
	operation.CompletedAt = &now
	_ = s.jobs.SaveOperation(operation)
	s.failFromError(job, err)
}

func (s *JobService) failFromError(job *model.ScrapeJob, err error) {
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

func (s *JobService) fail(job *model.ScrapeJob, code, message string, cause error) {
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

func (s *JobService) connectionLock(connectionID uint) *sync.Mutex {
	value, _ := s.locks.LoadOrStore(connectionID, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func (s *JobService) jobWorkspace(jobID uint) string {
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
