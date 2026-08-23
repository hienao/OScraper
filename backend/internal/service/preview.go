package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"oscraper/internal/model"
	"oscraper/internal/openlist"
	"oscraper/internal/provider/tmdb"
	"oscraper/internal/repository"

	"gorm.io/gorm"
)

const previewLifetime = 24 * time.Hour

type TMDBCatalog interface {
	Search(ctx context.Context, config tmdb.Config, mediaType, query string, year int) ([]tmdb.SearchResult, error)
	Detail(ctx context.Context, config tmdb.Config, mediaType string, id int) (*tmdb.Detail, error)
}

type TMDBSeasonCatalog interface {
	Season(ctx context.Context, config tmdb.Config, tvID, season int) ([]tmdb.Episode, error)
}

type PreviewService struct {
	targets   *repository.TargetRepository
	catalog   *repository.CatalogRepository
	previews  *repository.PreviewRepository
	audit     *repository.AuditRepository
	settings  *SettingService
	provider  TMDBCatalog
	inspector CandidateInspector
}

type CandidateInspector interface {
	InspectCandidate(ctx context.Context, targetID, candidateID uint, refresh bool) (*CandidateInspection, error)
}

type SearchPreviewCommand struct {
	CandidateID uint
	Title       string
	Year        int
}

type CreatePreviewCommand struct {
	CandidateID uint
	TMDBID      int
	Title       string
	Year        int
}

type RenameItem struct {
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
	AssetType  string `json:"asset_type"`
}

type PreviewArtifact struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	SourceURL string `json:"source_url,omitempty"`
	Content   string `json:"content,omitempty"`
}

type EpisodeFilePlan struct {
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
	Season     int    `json:"season"`
	Episode    int    `json:"episode"`
}

type PreviewPlan struct {
	ReadOnly                 bool              `json:"read_only"`
	Ready                    bool              `json:"ready"`
	RenameAllowed            bool              `json:"rename_allowed"`
	OrganizeFlatMovie        bool              `json:"organize_flat_movie"`
	SourcePath               string            `json:"source_path"`
	ProposedDirectoryName    string            `json:"proposed_directory_name"`
	ProposedDirectoryPath    string            `json:"proposed_directory_path"`
	ProposedDirectoryCreates []string          `json:"proposed_directory_creates"`
	ProposedDirectoryRenames []RenameItem      `json:"proposed_directory_renames"`
	ProposedFileRenames      []RenameItem      `json:"proposed_file_renames"`
	GeneratedFiles           []string          `json:"generated_files"`
	Artifacts                []PreviewArtifact `json:"artifacts"`
	EpisodeFiles             []EpisodeFilePlan `json:"episode_files"`
	Warnings                 []string          `json:"warnings"`
	Conflicts                []PlanConflict    `json:"conflicts"`
}

type PlanConflict struct {
	Code       string `json:"code"`
	SourcePath string `json:"source_path,omitempty"`
	TargetPath string `json:"target_path,omitempty"`
}

type PreviewResponse struct {
	ID          uint        `json:"id"`
	TargetID    uint        `json:"target_id"`
	CandidateID uint        `json:"candidate_id"`
	Fingerprint string      `json:"fingerprint"`
	Match       tmdb.Detail `json:"match"`
	Plan        PreviewPlan `json:"plan"`
	ExpiresAt   time.Time   `json:"expires_at"`
	CreatedAt   time.Time   `json:"created_at"`
}

func NewPreviewService(db *gorm.DB, settings *SettingService, provider TMDBCatalog, inspectors ...CandidateInspector) *PreviewService {
	previewService := &PreviewService{
		targets: repository.NewTargetRepository(db), catalog: repository.NewCatalogRepository(db),
		previews: repository.NewPreviewRepository(db), audit: repository.NewAuditRepository(db), settings: settings, provider: provider,
	}
	if len(inspectors) > 0 {
		previewService.inspector = inspectors[0]
	}
	return previewService
}

func (s *PreviewService) Search(ctx context.Context, targetID uint, request SearchPreviewCommand) ([]tmdb.SearchResult, error) {
	candidate, _, err := s.requireCandidate(targetID, request.CandidateID)
	if err != nil {
		return nil, err
	}
	query := strings.TrimSpace(request.Title)
	if query == "" {
		query = candidate.ParsedTitle
	}
	year := request.Year
	if year == 0 && candidate.Year != nil {
		year = *candidate.Year
	}
	config, hasKey, err := s.settings.TMDBConfig()
	if err != nil {
		return nil, err
	}
	if !hasKey {
		return nil, Conflict("tmdb.not_configured", "TMDB API key is not configured")
	}
	results, err := s.provider.Search(ctx, config, candidate.Kind, query, year)
	if err != nil {
		return nil, mapTMDBError(err)
	}
	sort.SliceStable(results, func(left, right int) bool {
		leftYear := year > 0 && results[left].Year == year
		rightYear := year > 0 && results[right].Year == year
		if leftYear != rightYear {
			return leftYear
		}
		if results[left].VoteAverage != results[right].VoteAverage {
			return results[left].VoteAverage > results[right].VoteAverage
		}
		return results[left].Popularity > results[right].Popularity
	})
	if len(results) > 20 {
		results = results[:20]
	}
	return results, nil
}

func (s *PreviewService) Create(ctx context.Context, targetID, actorID uint, request CreatePreviewCommand) (*PreviewResponse, error) {
	candidate, target, err := s.requireCandidate(targetID, request.CandidateID)
	if err != nil {
		return nil, err
	}
	var entries []openlist.DirectoryEntry
	var siblings []openlist.DirectoryEntry
	if s.inspector != nil {
		inspection, inspectErr := s.inspector.InspectCandidate(ctx, targetID, candidate.ID, true)
		if inspectErr != nil {
			return nil, inspectErr
		}
		if inspection.Stale {
			return nil, Conflict("preview.stale", "Media directory changed after the scan; run a new scan before previewing")
		}
		entries = inspection.Entries
		siblings = inspection.Siblings
	} else if candidate.ManifestJSON != "" {
		_ = json.Unmarshal([]byte(candidate.ManifestJSON), &entries)
	}
	config, hasKey, err := s.settings.TMDBConfig()
	if err != nil {
		return nil, err
	}
	if !hasKey {
		return nil, Conflict("tmdb.not_configured", "TMDB API key is not configured")
	}
	tmdbID := request.TMDBID
	if tmdbID == 0 && candidate.TMDBID != nil {
		tmdbID = *candidate.TMDBID
	}
	if tmdbID == 0 {
		results, searchErr := s.Search(ctx, targetID, SearchPreviewCommand{CandidateID: candidate.ID, Title: request.Title, Year: request.Year})
		if searchErr != nil {
			return nil, searchErr
		}
		if len(results) == 0 {
			return nil, NotFound("tmdb.no_results", "TMDB did not find a matching media item")
		}
		tmdbID = results[0].ID
	}
	detail, err := s.provider.Detail(ctx, config, candidate.Kind, tmdbID)
	if err != nil {
		return nil, mapTMDBError(err)
	}
	plan := buildFullPreviewPlan(target, candidate, detail, entries, siblings)
	if candidate.Kind != "movie" {
		if seasonProvider, ok := s.provider.(TMDBSeasonCatalog); ok {
			if err := expandEpisodeArtifacts(ctx, seasonProvider, config, detail, &plan); err != nil {
				return nil, mapTMDBError(err)
			}
		}
	}
	matchJSON, err := json.Marshal(detail)
	if err != nil {
		return nil, Internal("preview.encode_failed", "Failed to encode TMDB preview", err)
	}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return nil, Internal("preview.encode_failed", "Failed to encode scrape plan", err)
	}
	preview := &model.ScrapePreview{
		TargetID: target.ID, CandidateID: candidate.ID, ActorID: actorID, TMDBID: detail.ID,
		MediaType: detail.MediaType, Fingerprint: candidate.Fingerprint, MatchJSON: string(matchJSON), PlanJSON: string(planJSON),
		ExpiresAt: time.Now().UTC().Add(previewLifetime),
	}
	if err := s.previews.Create(preview); err != nil {
		return nil, Internal("preview.create_failed", "Failed to save scrape preview", err)
	}
	auditDetail, _ := json.Marshal(map[string]any{"preview_id": preview.ID, "candidate_id": candidate.ID, "tmdb_id": detail.ID, "media_type": detail.MediaType})
	_ = s.audit.Record(actorID, "preview.create", "scrape_candidate:"+candidate.Path, string(auditDetail))
	return previewResponse(preview, *detail, plan), nil
}

func (s *PreviewService) Get(targetID, previewID uint) (*PreviewResponse, error) {
	preview, err := s.previews.Find(previewID, targetID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("preview.not_found", "Scrape preview not found")
	}
	if err != nil {
		return nil, Internal("preview.lookup_failed", "Failed to load scrape preview", err)
	}
	var match tmdb.Detail
	var plan PreviewPlan
	if err := json.Unmarshal([]byte(preview.MatchJSON), &match); err != nil {
		return nil, Internal("preview.invalid_snapshot", "Stored scrape preview is invalid", err)
	}
	if err := json.Unmarshal([]byte(preview.PlanJSON), &plan); err != nil {
		return nil, Internal("preview.invalid_snapshot", "Stored scrape plan is invalid", err)
	}
	return previewResponse(preview, match, plan), nil
}

func (s *PreviewService) requireCandidate(targetID, candidateID uint) (*model.MediaCandidate, *model.ScrapeTarget, error) {
	target, err := s.targets.Find(targetID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, NotFound("target.not_found", "Scrape target not found")
	}
	if err != nil {
		return nil, nil, Internal("target.lookup_failed", "Failed to load scrape target", err)
	}
	candidate, err := s.catalog.FindCandidate(candidateID, targetID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, NotFound("candidate.not_found", "Media candidate not found")
	}
	if err != nil {
		return nil, nil, Internal("candidate.lookup_failed", "Failed to load media candidate", err)
	}
	return candidate, target, nil
}

func safeMediaName(value string) string {
	value = strings.NewReplacer("/", "／", "\\", "＼").Replace(strings.TrimSpace(value))
	value = strings.Map(func(character rune) rune {
		if character < 32 || character == 127 {
			return -1
		}
		return character
	}, value)
	value = strings.Trim(value, " .")
	if value == "" || value == "." || value == ".." {
		value = "Untitled"
	}
	for utf8.RuneCountInString(value) > 180 {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}

func previewResponse(preview *model.ScrapePreview, match tmdb.Detail, plan PreviewPlan) *PreviewResponse {
	if plan.ProposedDirectoryCreates == nil {
		plan.ProposedDirectoryCreates = []string{}
	}
	if plan.ProposedDirectoryRenames == nil {
		plan.ProposedDirectoryRenames = []RenameItem{}
	}
	if plan.ProposedFileRenames == nil {
		plan.ProposedFileRenames = []RenameItem{}
	}
	if plan.GeneratedFiles == nil {
		plan.GeneratedFiles = []string{}
	}
	if plan.Artifacts == nil {
		plan.Artifacts = []PreviewArtifact{}
	}
	if plan.EpisodeFiles == nil {
		plan.EpisodeFiles = []EpisodeFilePlan{}
	}
	if plan.Warnings == nil {
		plan.Warnings = []string{}
	}
	if plan.Conflicts == nil {
		plan.Conflicts = []PlanConflict{}
	}
	return &PreviewResponse{
		ID: preview.ID, TargetID: preview.TargetID, CandidateID: preview.CandidateID,
		Fingerprint: preview.Fingerprint, Match: match, Plan: plan, ExpiresAt: preview.ExpiresAt, CreatedAt: preview.CreatedAt,
	}
}
