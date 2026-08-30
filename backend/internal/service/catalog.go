package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"oscraper/internal/logging"
	"oscraper/internal/media"
	"oscraper/internal/model"
	"oscraper/internal/openlist"
	"oscraper/internal/repository"
	"oscraper/pkg/cryptoutil"

	"gorm.io/gorm"
)

const (
	maxScanDepth   = 12
	maxScanEntries = 50000
)

type CatalogService struct {
	targets     *repository.TargetRepository
	connections *repository.ConnectionRepository
	catalog     *repository.CatalogRepository
	audit       *repository.AuditRepository
	cipher      *cryptoutil.Cipher
	client      DirectoryBrowser
	activeMu    sync.Mutex
	active      map[uint]struct{}
	quota       *ConnectionQuota
	local       *localStorage
	recognizer  MediaRecognizer
	rootCtx     context.Context
	cancel      context.CancelFunc
	slots       chan struct{}
	workers     chan struct{}
	wait        sync.WaitGroup
	startOnce   sync.Once
	startErr    error
}

type MediaRecognizer interface {
	Recognize(ctx context.Context, fileName, relativePath, libraryType string) (media.Info, bool, error)
}

type catalogSource struct {
	connection *model.OpenListConnection
	token      string
	local      *localStorage
}

type ScanResponse struct {
	model.ScanRun
	Candidates []model.MediaCandidate `json:"candidates,omitempty"`
}

type scanState struct {
	entries int
	videos  int
}

type candidateFiles struct {
	entries        []openlist.DirectoryEntry
	representative string
	videos         int
}

type CandidateInspection struct {
	Candidate   *model.MediaCandidate
	Entries     []openlist.DirectoryEntry
	Siblings    []openlist.DirectoryEntry
	Fingerprint string
	Stale       bool
}

func NewCatalogService(db *gorm.DB, cipher *cryptoutil.Cipher, client DirectoryBrowser, quotas ...*ConnectionQuota) *CatalogService {
	var quota *ConnectionQuota
	if len(quotas) > 0 {
		quota = quotas[0]
	}
	return NewCatalogServiceWithLocalRoot(db, cipher, client, quota, defaultLocalMediaRoot)
}

func NewCatalogServiceWithLocalRoot(db *gorm.DB, cipher *cryptoutil.Cipher, client DirectoryBrowser, quota *ConnectionQuota, localRoot string) *CatalogService {
	return NewCatalogServiceWithRuntime(db, cipher, client, quota, localRoot, 1, 20)
}

func NewCatalogServiceWithRuntime(db *gorm.DB, cipher *cryptoutil.Cipher, client DirectoryBrowser, quota *ConnectionQuota, localRoot string, workers, queueSize int, recognizers ...MediaRecognizer) *CatalogService {
	if workers < 1 {
		workers = 1
	}
	if queueSize < 1 {
		queueSize = 20
	}
	rootCtx, cancel := context.WithCancel(context.Background())
	service := &CatalogService{
		targets: repository.NewTargetRepository(db), connections: repository.NewConnectionRepository(db),
		catalog: repository.NewCatalogRepository(db), audit: repository.NewAuditRepository(db), cipher: cipher, client: client,
		active: make(map[uint]struct{}), quota: NewConnectionQuota(), local: newLocalStorage(localRoot),
		rootCtx: rootCtx, cancel: cancel, slots: make(chan struct{}, workers+queueSize), workers: make(chan struct{}, workers),
	}
	if quota != nil {
		service.quota = quota
	}
	if len(recognizers) > 0 {
		service.recognizer = recognizers[0]
	}
	return service
}

type ScanRuntimeStats struct {
	Active   int `json:"active"`
	Queued   int `json:"queued"`
	Running  int `json:"running"`
	Capacity int `json:"capacity"`
}

func (s *CatalogService) Start() error {
	s.startOnce.Do(func() {
		var scans []model.ScanRun
		scans, s.startErr = s.catalog.RecoverInterruptedScans()
		if s.startErr != nil {
			return
		}
		for index := range scans {
			scan := scans[index]
			if !s.beginScan(scan.TargetID) {
				s.failQueuedScan(&scan, "scan.duplicate_recovery", "A newer scan is already active for this target")
				continue
			}
			select {
			case s.slots <- struct{}{}:
				s.dispatchScan(scan.ID, scan.TargetID)
			default:
				s.endScan(scan.TargetID)
				s.failQueuedScan(&scan, "scan.queue_recovery_full", "The recovered scan queue exceeds its configured capacity")
			}
		}
		if len(scans) > 0 {
			logging.Warn("scan", "interrupted scans recovered", logging.Fields{"count": len(scans)})
		}
	})
	return s.startErr
}

func (s *CatalogService) Shutdown(ctx context.Context) error {
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

func (s *CatalogService) Metrics() ScanRuntimeStats {
	s.activeMu.Lock()
	active := len(s.active)
	s.activeMu.Unlock()
	running := len(s.workers)
	return ScanRuntimeStats{Active: active, Queued: max(0, len(s.slots)-running), Running: running, Capacity: cap(s.slots)}
}

func (s *CatalogService) StartScan(targetID, actorID uint, refresh bool) (*ScanResponse, error) {
	if err := s.Start(); err != nil {
		return nil, Internal("scan.runtime_failed", "Failed to start directory scan runtime", err)
	}
	if _, _, err := s.scanSource(targetID); err != nil {
		return nil, err
	}
	active, err := s.catalog.ActiveScanCount(targetID)
	if err != nil {
		return nil, Internal("scan.active_check_failed", "Failed to check active directory scans", err)
	}
	if active > 0 || !s.beginScan(targetID) {
		return nil, Conflict("scan.already_running", "A directory scan is already running for this target")
	}
	select {
	case s.slots <- struct{}{}:
	default:
		s.endScan(targetID)
		return nil, TooManyRequests("scan.queue_full", "The directory scan queue is full")
	}
	scan := &model.ScanRun{TargetID: targetID, ActorID: actorID, Refresh: refresh, Status: "pending"}
	if err := s.catalog.CreateScan(scan); err != nil {
		<-s.slots
		s.endScan(targetID)
		return nil, Internal("scan.create_failed", "Failed to create directory scan", err)
	}
	s.dispatchScan(scan.ID, targetID)
	return &ScanResponse{ScanRun: *scan}, nil
}

func (s *CatalogService) dispatchScan(scanID, targetID uint) {
	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
		defer func() { <-s.slots; s.endScan(targetID) }()
		select {
		case s.workers <- struct{}{}:
			defer func() { <-s.workers }()
		case <-s.rootCtx.Done():
			return
		}
		claimed, err := s.catalog.ClaimScan(scanID)
		if err != nil || !claimed {
			if err != nil {
				logging.Error("scan", "failed to claim scan", logging.Fields{"scan_id": scanID, "error": err})
			}
			return
		}
		scan, err := s.catalog.FindScan(scanID, targetID)
		if err != nil {
			logging.Error("scan", "failed to load claimed scan", logging.Fields{"scan_id": scanID, "error": err})
			return
		}
		_, runErr := s.runScan(s.rootCtx, scan)
		if runErr != nil {
			logging.Error("scan", "directory scan failed", logging.Fields{"scan_id": scanID, "target_id": targetID, "error": runErr})
		}
	}()
}

func (s *CatalogService) failQueuedScan(scan *model.ScanRun, code, message string) {
	completed := time.Now().UTC()
	scan.Status, scan.ErrorCode, scan.ErrorMessage, scan.CompletedAt = "failed", code, message, &completed
	_ = s.catalog.SaveScan(scan)
}

func (s *CatalogService) Scan(ctx context.Context, targetID, actorID uint, refresh bool) (*ScanResponse, error) {
	if !s.beginScan(targetID) {
		return nil, Conflict("scan.already_running", "A directory scan is already running for this target")
	}
	defer s.endScan(targetID)
	startedAt := time.Now().UTC()
	scan := &model.ScanRun{TargetID: targetID, ActorID: actorID, Refresh: refresh, Status: "running", StartedAt: &startedAt}
	if err := s.catalog.CreateScan(scan); err != nil {
		return nil, Internal("scan.create_failed", "Failed to create directory scan", err)
	}
	return s.runScan(ctx, scan)
}

func (s *CatalogService) runScan(ctx context.Context, scan *model.ScanRun) (*ScanResponse, error) {
	target, source, err := s.scanSource(scan.TargetID)
	if err != nil {
		s.failScan(scan, err)
		return nil, err
	}

	candidates, scanErr := s.discover(ctx, target, source, scan.Refresh)
	completed := time.Now().UTC()
	scan.CompletedAt = &completed
	if scanErr != nil {
		s.failScan(scan, scanErr)
		return nil, scanErr
	}

	scan.Status = "succeeded"
	scan.CandidateCount = len(candidates)
	for index := range candidates {
		scan.VideoCount += candidates[index].VideoCount
		if candidates[index].Scraped {
			scan.ScrapedCount++
		}
		candidates[index].ScanID = scan.ID
		candidates[index].TargetID = target.ID
	}
	if err := s.catalog.CompleteScan(scan, candidates); err != nil {
		return nil, Internal("scan.save_failed", "Failed to save directory scan", err)
	}
	s.recordScanAudit(scan.ActorID, target, scan)
	return &ScanResponse{ScanRun: *scan, Candidates: candidates}, nil
}

func (s *CatalogService) failScan(scan *model.ScanRun, scanErr error) {
	completed := time.Now().UTC()
	scan.Status, scan.CompletedAt = "failed", &completed
	if serviceError, ok := scanErr.(*Error); ok {
		scan.ErrorCode, scan.ErrorMessage = serviceError.Code, serviceError.Message
	} else {
		scan.ErrorCode, scan.ErrorMessage = "scan.failed", "Directory scan failed"
	}
	_ = s.catalog.SaveScan(scan)
}

func (s *CatalogService) beginScan(targetID uint) bool {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	if _, exists := s.active[targetID]; exists {
		return false
	}
	s.active[targetID] = struct{}{}
	return true
}

func (s *CatalogService) endScan(targetID uint) {
	s.activeMu.Lock()
	delete(s.active, targetID)
	s.activeMu.Unlock()
}

func (s *CatalogService) GetScan(targetID, scanID uint) (*ScanResponse, error) {
	if _, err := s.requireTarget(targetID); err != nil {
		return nil, err
	}
	scan, err := s.catalog.FindScan(scanID, targetID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("scan.not_found", "Directory scan not found")
	}
	if err != nil {
		return nil, Internal("scan.lookup_failed", "Failed to load directory scan", err)
	}
	candidates, err := s.catalog.Candidates(targetID, scan.ID)
	if err != nil {
		return nil, Internal("candidate.list_failed", "Failed to list media candidates", err)
	}
	return &ScanResponse{ScanRun: *scan, Candidates: candidates}, nil
}

func (s *CatalogService) Candidates(targetID, scanID uint) ([]model.MediaCandidate, error) {
	if _, err := s.requireTarget(targetID); err != nil {
		return nil, err
	}
	if scanID == 0 {
		scan, err := s.catalog.LatestScan(targetID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []model.MediaCandidate{}, nil
		}
		if err != nil {
			return nil, Internal("scan.lookup_failed", "Failed to load latest directory scan", err)
		}
		scanID = scan.ID
	} else if _, err := s.catalog.FindScan(scanID, targetID); errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("scan.not_found", "Directory scan not found")
	} else if err != nil {
		return nil, Internal("scan.lookup_failed", "Failed to load directory scan", err)
	}
	candidates, err := s.catalog.Candidates(targetID, scanID)
	if err != nil {
		return nil, Internal("candidate.list_failed", "Failed to list media candidates", err)
	}
	return candidates, nil
}

func (s *CatalogService) InspectCandidate(ctx context.Context, targetID, candidateID uint, refresh bool) (*CandidateInspection, error) {
	target, source, err := s.scanSource(targetID)
	if err != nil {
		return nil, err
	}
	candidate, err := s.catalog.FindCandidate(candidateID, targetID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("candidate.not_found", "Media candidate not found")
	}
	if err != nil {
		return nil, Internal("candidate.lookup_failed", "Failed to load media candidate", err)
	}
	if !openlist.IsWithinPath(target.RootPath, candidate.Path) {
		return nil, Forbidden("candidate.path_outside_root", "Media candidate is outside the scrape target root")
	}
	parentPath := path.Dir(candidate.Path)
	siblings, err := s.listDirectory(ctx, source, parentPath, refresh)
	if err != nil {
		return nil, err
	}
	var entries []openlist.DirectoryEntry
	if media.IsVideoFile(path.Base(candidate.Path)) {
		found := false
		for _, entry := range siblings {
			if entry.Path == candidate.Path && !entry.IsDir {
				found = true
				break
			}
		}
		if !found {
			return nil, Conflict("candidate.source_missing", "Media candidate no longer exists")
		}
		entries = relatedFlatAssets(candidate.Path, siblings)
	} else {
		state := &scanState{}
		files, walkErr := s.walk(ctx, source, candidate.Path, candidate.Path, 1, refresh, state)
		if walkErr != nil {
			return nil, walkErr
		}
		entries = files.entries
	}
	currentFingerprint := fingerprint(entries)
	return &CandidateInspection{Candidate: candidate, Entries: entries, Siblings: siblings, Fingerprint: currentFingerprint, Stale: currentFingerprint != candidate.Fingerprint}, nil
}

func (s *CatalogService) discover(ctx context.Context, target *model.ScrapeTarget, source *catalogSource, refresh bool) ([]model.MediaCandidate, error) {
	rootEntries, err := s.listDirectory(ctx, source, target.RootPath, refresh)
	if err != nil {
		return nil, err
	}
	state := &scanState{entries: len(rootEntries)}
	if state.entries > maxScanEntries {
		return nil, Conflict("scan.too_large", "Directory scan exceeds the safe entry limit")
	}
	candidates := make([]model.MediaCandidate, 0)
	for _, entry := range rootEntries {
		if err := ctx.Err(); err != nil {
			return nil, Internal("scan.canceled", "Directory scan was canceled", err)
		}
		if target.LibraryType == "movie" && !entry.IsDir && media.IsVideoFile(entry.Name) {
			info := media.ParseCandidate(strings.TrimSuffix(entry.Name, path.Ext(entry.Name)), entry.Name, target.LibraryType)
			info = s.recognizeIfNeeded(ctx, info, entry.Name, entry.Name, target.LibraryType)
			assets := relatedFlatAssets(entry.Path, rootEntries)
			candidates = append(candidates, makeCandidate(entry.Path, target.LibraryType, info, entry.Name, 1, assets, hasFlatScrapeMarker(entry.Path, assets)))
			continue
		}
		if !entry.IsDir {
			continue
		}
		files, walkErr := s.walk(ctx, source, entry.Path, entry.Path, 1, refresh, state)
		if walkErr != nil {
			return nil, walkErr
		}
		if files.videos == 0 {
			continue
		}
		info := media.ParseCandidate(entry.Name, files.representative, target.LibraryType)
		info = s.recognizeIfNeeded(ctx, info, path.Base(files.representative), path.Join(entry.Name, files.representative), target.LibraryType)
		candidates = append(candidates, makeCandidate(entry.Path, target.LibraryType, info, files.representative, files.videos, files.entries, hasDirectoryScrapeMarker(entry.Path, files.entries)))
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Path < candidates[j].Path })
	return candidates, nil
}

func (s *CatalogService) recognizeIfNeeded(ctx context.Context, parsed media.Info, fileName, relativePath, libraryType string) media.Info {
	if s.recognizer == nil || parsed.Confidence >= 70 || parsed.TMDBID != nil {
		return parsed
	}
	recognized, ok, err := s.recognizer.Recognize(ctx, fileName, relativePath, libraryType)
	if err != nil {
		logging.Warn("scan", "AI media recognition failed; using local parser result", logging.Fields{"path": relativePath, "error": err})
		return parsed
	}
	if !ok {
		return parsed
	}
	if recognized.TMDBID == nil {
		recognized.TMDBID = parsed.TMDBID
	}
	return recognized
}

func (s *CatalogService) walk(ctx context.Context, source *catalogSource, candidateRoot, currentPath string, depth int, refresh bool, state *scanState) (candidateFiles, error) {
	if depth > maxScanDepth {
		return candidateFiles{}, Conflict("scan.too_deep", "Directory scan exceeds the safe depth limit")
	}
	entries, err := s.listDirectory(ctx, source, currentPath, refresh)
	if err != nil {
		return candidateFiles{}, err
	}
	state.entries += len(entries)
	if state.entries > maxScanEntries {
		return candidateFiles{}, Conflict("scan.too_large", "Directory scan exceeds the safe entry limit")
	}
	result := candidateFiles{entries: append([]openlist.DirectoryEntry(nil), entries...)}
	for _, entry := range entries {
		if !openlist.IsWithinPath(candidateRoot, entry.Path) {
			return candidateFiles{}, Forbidden("scan.path_outside_candidate", "Storage returned an entry outside the candidate root")
		}
		if entry.IsDir {
			nested, nestedErr := s.walk(ctx, source, candidateRoot, entry.Path, depth+1, refresh, state)
			if nestedErr != nil {
				return candidateFiles{}, nestedErr
			}
			result.entries = append(result.entries, nested.entries...)
			result.videos += nested.videos
			if result.representative == "" {
				result.representative = nested.representative
			}
			continue
		}
		if media.IsVideoFile(entry.Name) {
			result.videos++
			state.videos++
			if result.representative == "" {
				result.representative = strings.TrimPrefix(entry.Path, candidateRoot+"/")
			}
		}
	}
	return result, nil
}

func (s *CatalogService) listDirectory(ctx context.Context, source *catalogSource, remotePath string, refresh bool) ([]openlist.DirectoryEntry, error) {
	if source.local != nil {
		return source.local.ListDirectory(ctx, remotePath, false)
	}
	if err := s.waitForReadQuota(ctx, source.connection); err != nil {
		return nil, err
	}
	entries, err := s.client.ListDirectory(ctx, source.connection.BaseURL, source.token, remotePath, refresh)
	if err != nil {
		return nil, mapOpenListError(err)
	}
	return entries, nil
}

// waitForReadQuota enforces each connection's QPS and QPM as sliding windows.
// The registry is shared by all scans in this process, so concurrent targets
// using the same OpenList connection consume the same allowance.
func (s *CatalogService) waitForReadQuota(ctx context.Context, connection *model.OpenListConnection) error {
	return s.quota.Wait(ctx, connection)
}

func makeCandidate(candidatePath, kind string, info media.Info, representative string, videoCount int, entries []openlist.DirectoryEntry, scraped bool) model.MediaCandidate {
	status := "ready"
	if info.Confidence < 70 {
		status = "needs_review"
	}
	manifest, _ := json.Marshal(entries)
	return model.MediaCandidate{
		Path: candidatePath, Kind: kind, Fingerprint: fingerprint(entries), RepresentativeFile: representative,
		ManifestJSON: string(manifest),
		ParsedTitle:  info.Title, Year: info.Year, Season: info.Season, Episode: info.Episode, TMDBID: info.TMDBID,
		Confidence: info.Confidence, VideoCount: videoCount, Scraped: scraped, Status: status,
	}
}

func fingerprint(entries []openlist.DirectoryEntry) string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir && isScrapeMarkerName(entry.Name) {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s\x00%t\x00%d\x00%s\x00%s", entry.Path, entry.IsDir, entry.Size, entry.Modified, entry.Sign))
	}
	sort.Strings(lines)
	hash := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return "sha256:" + hex.EncodeToString(hash[:])
}

func (s *CatalogService) scanSource(targetID uint) (*model.ScrapeTarget, *catalogSource, error) {
	target, err := s.requireTarget(targetID)
	if err != nil {
		return nil, nil, err
	}
	if !target.Enabled {
		return nil, nil, Conflict("scan.target_disabled", "Scrape target is disabled")
	}
	if sourceType(target) == "local" {
		return target, &catalogSource{local: s.local}, nil
	}
	if target.ConnectionID == nil || *target.ConnectionID == 0 {
		return nil, nil, Internal("target.connection_failed", "OpenList target has no connection", nil)
	}
	connection, err := s.connections.Find(*target.ConnectionID)
	if err != nil {
		return nil, nil, Internal("target.connection_failed", "Failed to load target connection", err)
	}
	if !connection.Enabled {
		return nil, nil, Conflict("target.connection_disabled", "OpenList connection is disabled")
	}
	token, err := s.cipher.Decrypt(connection.EncryptedToken)
	if err != nil {
		return nil, nil, Internal("connection.decryption_failed", "Stored OpenList token cannot be decrypted", err)
	}
	return target, &catalogSource{connection: connection, token: token}, nil
}

func (s *CatalogService) requireTarget(id uint) (*model.ScrapeTarget, error) {
	target, err := s.targets.Find(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("target.not_found", "Scrape target not found")
	}
	if err != nil {
		return nil, Internal("target.lookup_failed", "Failed to load scrape target", err)
	}
	return target, nil
}

func (s *CatalogService) recordScanAudit(actorID uint, target *model.ScrapeTarget, scan *model.ScanRun) {
	detail := fmt.Sprintf(`{"scan_id":%d,"candidate_count":%d,"scraped_candidate_count":%d,"video_count":%d}`, scan.ID, scan.CandidateCount, scan.ScrapedCount, scan.VideoCount)
	_ = s.audit.Record(actorID, "target.scan", "scrape_target:"+target.Name, detail)
}
