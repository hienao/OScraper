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
	service := &CatalogService{
		targets: repository.NewTargetRepository(db), connections: repository.NewConnectionRepository(db),
		catalog: repository.NewCatalogRepository(db), audit: repository.NewAuditRepository(db), cipher: cipher, client: client,
		active: make(map[uint]struct{}), quota: NewConnectionQuota(),
	}
	if len(quotas) > 0 && quotas[0] != nil {
		service.quota = quotas[0]
	}
	return service
}

func (s *CatalogService) Scan(ctx context.Context, targetID, actorID uint, refresh bool) (*ScanResponse, error) {
	if !s.beginScan(targetID) {
		return nil, Conflict("scan.already_running", "A directory scan is already running for this target")
	}
	defer s.endScan(targetID)
	target, connection, token, err := s.scanCredentials(targetID)
	if err != nil {
		return nil, err
	}
	scan := &model.ScanRun{TargetID: target.ID, Status: "running", StartedAt: time.Now().UTC()}
	if err := s.catalog.CreateScan(scan); err != nil {
		return nil, Internal("scan.create_failed", "Failed to create directory scan", err)
	}

	candidates, scanErr := s.discover(ctx, target, connection, token, refresh)
	completed := time.Now().UTC()
	scan.CompletedAt = &completed
	if scanErr != nil {
		scan.Status = "failed"
		if serviceError, ok := scanErr.(*Error); ok {
			scan.ErrorCode = serviceError.Code
			scan.ErrorMessage = serviceError.Message
		} else {
			scan.ErrorCode = "scan.failed"
			scan.ErrorMessage = "Directory scan failed"
		}
		_ = s.catalog.SaveScan(scan)
		return nil, scanErr
	}

	scan.Status = "succeeded"
	scan.CandidateCount = len(candidates)
	for index := range candidates {
		scan.VideoCount += candidates[index].VideoCount
		candidates[index].ScanID = scan.ID
		candidates[index].TargetID = target.ID
	}
	if err := s.catalog.CompleteScan(scan, candidates); err != nil {
		return nil, Internal("scan.save_failed", "Failed to save directory scan", err)
	}
	s.recordScanAudit(actorID, target, scan)
	return &ScanResponse{ScanRun: *scan, Candidates: candidates}, nil
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
	target, connection, token, err := s.scanCredentials(targetID)
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
	siblings, err := s.listDirectory(ctx, connection, token, parentPath, refresh)
	if err != nil {
		return nil, mapOpenListError(err)
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
			return nil, Conflict("candidate.source_missing", "Media candidate no longer exists in OpenList")
		}
		entries = relatedFlatAssets(candidate.Path, siblings)
	} else {
		state := &scanState{}
		files, walkErr := s.walk(ctx, connection, token, candidate.Path, candidate.Path, 1, refresh, state)
		if walkErr != nil {
			return nil, walkErr
		}
		entries = files.entries
	}
	currentFingerprint := fingerprint(entries)
	return &CandidateInspection{Candidate: candidate, Entries: entries, Siblings: siblings, Fingerprint: currentFingerprint, Stale: currentFingerprint != candidate.Fingerprint}, nil
}

func (s *CatalogService) discover(ctx context.Context, target *model.ScrapeTarget, connection *model.OpenListConnection, token string, refresh bool) ([]model.MediaCandidate, error) {
	rootEntries, err := s.listDirectory(ctx, connection, token, target.RootPath, refresh)
	if err != nil {
		return nil, mapOpenListError(err)
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
			candidates = append(candidates, makeCandidate(entry.Path, target.LibraryType, info, entry.Name, 1, relatedFlatAssets(entry.Path, rootEntries)))
			continue
		}
		if !entry.IsDir {
			continue
		}
		files, walkErr := s.walk(ctx, connection, token, entry.Path, entry.Path, 1, refresh, state)
		if walkErr != nil {
			return nil, walkErr
		}
		if files.videos == 0 {
			continue
		}
		info := media.ParseCandidate(entry.Name, files.representative, target.LibraryType)
		candidates = append(candidates, makeCandidate(entry.Path, target.LibraryType, info, files.representative, files.videos, files.entries))
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Path < candidates[j].Path })
	return candidates, nil
}

func (s *CatalogService) walk(ctx context.Context, connection *model.OpenListConnection, token, candidateRoot, currentPath string, depth int, refresh bool, state *scanState) (candidateFiles, error) {
	if depth > maxScanDepth {
		return candidateFiles{}, Conflict("scan.too_deep", "Directory scan exceeds the safe depth limit")
	}
	entries, err := s.listDirectory(ctx, connection, token, currentPath, refresh)
	if err != nil {
		return candidateFiles{}, mapOpenListError(err)
	}
	state.entries += len(entries)
	if state.entries > maxScanEntries {
		return candidateFiles{}, Conflict("scan.too_large", "Directory scan exceeds the safe entry limit")
	}
	result := candidateFiles{entries: append([]openlist.DirectoryEntry(nil), entries...)}
	for _, entry := range entries {
		if !openlist.IsWithinPath(candidateRoot, entry.Path) {
			return candidateFiles{}, Forbidden("scan.path_outside_candidate", "OpenList returned an entry outside the candidate root")
		}
		if entry.IsDir {
			nested, nestedErr := s.walk(ctx, connection, token, candidateRoot, entry.Path, depth+1, refresh, state)
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

func (s *CatalogService) listDirectory(ctx context.Context, connection *model.OpenListConnection, token, remotePath string, refresh bool) ([]openlist.DirectoryEntry, error) {
	if err := s.waitForReadQuota(ctx, connection); err != nil {
		return nil, err
	}
	return s.client.ListDirectory(ctx, connection.BaseURL, token, remotePath, refresh)
}

// waitForReadQuota enforces each connection's QPS and QPM as sliding windows.
// The registry is shared by all scans in this process, so concurrent targets
// using the same OpenList connection consume the same allowance.
func (s *CatalogService) waitForReadQuota(ctx context.Context, connection *model.OpenListConnection) error {
	return s.quota.Wait(ctx, connection)
}

func makeCandidate(candidatePath, kind string, info media.Info, representative string, videoCount int, entries []openlist.DirectoryEntry) model.MediaCandidate {
	status := "ready"
	if info.Confidence < 70 {
		status = "needs_review"
	}
	manifest, _ := json.Marshal(entries)
	return model.MediaCandidate{
		Path: candidatePath, Kind: kind, Fingerprint: fingerprint(entries), RepresentativeFile: representative,
		ManifestJSON: string(manifest),
		ParsedTitle:  info.Title, Year: info.Year, Season: info.Season, Episode: info.Episode, TMDBID: info.TMDBID,
		Confidence: info.Confidence, VideoCount: videoCount, Status: status,
	}
}

func fingerprint(entries []openlist.DirectoryEntry) string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, fmt.Sprintf("%s\x00%t\x00%d\x00%s", entry.Path, entry.IsDir, entry.Size, entry.Modified))
	}
	sort.Strings(lines)
	hash := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return "sha256:" + hex.EncodeToString(hash[:])
}

func (s *CatalogService) scanCredentials(targetID uint) (*model.ScrapeTarget, *model.OpenListConnection, string, error) {
	target, err := s.requireTarget(targetID)
	if err != nil {
		return nil, nil, "", err
	}
	if !target.Enabled {
		return nil, nil, "", Conflict("scan.target_disabled", "Scrape target is disabled")
	}
	connection, err := s.connections.Find(target.ConnectionID)
	if err != nil {
		return nil, nil, "", Internal("target.connection_failed", "Failed to load target connection", err)
	}
	if !connection.Enabled {
		return nil, nil, "", Conflict("target.connection_disabled", "OpenList connection is disabled")
	}
	token, err := s.cipher.Decrypt(connection.EncryptedToken)
	if err != nil {
		return nil, nil, "", Internal("connection.decryption_failed", "Stored OpenList token cannot be decrypted", err)
	}
	return target, connection, token, nil
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
	detail := fmt.Sprintf(`{"scan_id":%d,"candidate_count":%d,"video_count":%d}`, scan.ID, scan.CandidateCount, scan.VideoCount)
	_ = s.audit.Record(actorID, "target.scan", "scrape_target:"+target.Name, detail)
}
