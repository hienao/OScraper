package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"oscraper/internal/model"
	"oscraper/internal/openlist"
	"oscraper/internal/repository"
	"oscraper/pkg/cryptoutil"

	"gorm.io/gorm"
)

type DirectoryBrowser interface {
	ListDirectory(ctx context.Context, baseURL, token, path string, refresh bool) ([]openlist.DirectoryEntry, error)
}

type warningDirectoryBrowser interface {
	ListDirectoryWithWarnings(ctx context.Context, baseURL, token, path string, refresh bool) (openlist.DirectoryListing, error)
}

type TargetService struct {
	targets     *repository.TargetRepository
	connections *repository.ConnectionRepository
	audit       *repository.AuditRepository
	jobs        *repository.JobRepository
	cipher      *cryptoutil.Cipher
	client      DirectoryBrowser
	local       *localStorage
}

type SaveTargetCommand struct {
	SourceType    string
	ConnectionID  uint
	Name          string
	RootPath      string
	LibraryType   string
	RenameEnabled bool
	Enabled       bool
}

type TargetResponse struct {
	ID             uint      `json:"id"`
	SourceType     string    `json:"source_type"`
	ConnectionID   *uint     `json:"connection_id,omitempty"`
	ConnectionName string    `json:"connection_name"`
	Name           string    `json:"name"`
	RootPath       string    `json:"root_path"`
	LibraryType    string    `json:"library_type"`
	RenameEnabled  bool      `json:"rename_enabled"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type DirectoryNode struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	Modified string `json:"modified,omitempty"`
}

type DirectoryLevel struct {
	TargetID uint                        `json:"target_id"`
	RootPath string                      `json:"root_path"`
	Path     string                      `json:"path"`
	Entries  []DirectoryNode             `json:"entries"`
	Warnings []openlist.DirectoryWarning `json:"warnings,omitempty"`
}

func NewTargetService(db *gorm.DB, cipher *cryptoutil.Cipher, client DirectoryBrowser, localRoots ...string) *TargetService {
	localRoot := defaultLocalMediaRoot
	if len(localRoots) > 0 {
		localRoot = localRoots[0]
	}
	return &TargetService{
		targets: repository.NewTargetRepository(db), connections: repository.NewConnectionRepository(db),
		audit: repository.NewAuditRepository(db), jobs: repository.NewJobRepository(db), cipher: cipher, client: client,
		local: newLocalStorage(localRoot),
	}
}

func (s *TargetService) Get(id uint) (*TargetResponse, error) {
	target, err := s.require(id)
	if err != nil {
		return nil, err
	}
	connectionName := "Local media"
	if sourceType(target) == "openlist" {
		connection, connectionErr := s.requireTargetConnection(target)
		if connectionErr != nil {
			return nil, connectionErr
		}
		connectionName = connection.Name
	}
	response := targetResponse(target, connectionName)
	return &response, nil
}

func (s *TargetService) List() ([]TargetResponse, error) {
	targets, err := s.targets.List()
	if err != nil {
		return nil, Internal("target.list_failed", "Failed to list scrape targets", err)
	}
	responses := make([]TargetResponse, 0, len(targets))
	for index := range targets {
		connectionName := "Local media"
		if sourceType(&targets[index]) == "openlist" {
			connection, connectionErr := s.requireTargetConnection(&targets[index])
			if connectionErr != nil {
				return nil, connectionErr
			}
			connectionName = connection.Name
		}
		responses = append(responses, targetResponse(&targets[index], connectionName))
	}
	return responses, nil
}

func (s *TargetService) Create(ctx context.Context, actorID uint, request SaveTargetCommand) (*TargetResponse, error) {
	source, connectionID, connectionName, rootPath, err := s.validate(ctx, request, 0)
	if err != nil {
		return nil, err
	}
	target := &model.ScrapeTarget{
		SourceType: source, ConnectionID: connectionID, Name: strings.TrimSpace(request.Name), RootPath: rootPath,
		LibraryType: strings.ToLower(request.LibraryType), RenameEnabled: request.RenameEnabled, Enabled: request.Enabled,
	}
	if err := s.targets.Create(target); err != nil {
		return nil, Internal("target.create_failed", "Failed to create scrape target", err)
	}
	s.recordAudit(actorID, "target.create", target)
	response := targetResponse(target, connectionName)
	return &response, nil
}

func (s *TargetService) Update(ctx context.Context, id, actorID uint, request SaveTargetCommand) (*TargetResponse, error) {
	target, err := s.require(id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIdle(id); err != nil {
		return nil, err
	}
	source, connectionID, connectionName, rootPath, err := s.validate(ctx, request, id)
	if err != nil {
		return nil, err
	}
	target.SourceType = source
	target.ConnectionID = connectionID
	target.Name = strings.TrimSpace(request.Name)
	target.RootPath = rootPath
	target.LibraryType = strings.ToLower(request.LibraryType)
	target.RenameEnabled = request.RenameEnabled
	target.Enabled = request.Enabled
	if err := s.targets.Update(target); err != nil {
		return nil, Internal("target.update_failed", "Failed to update scrape target", err)
	}
	s.recordAudit(actorID, "target.update", target)
	response := targetResponse(target, connectionName)
	return &response, nil
}

func (s *TargetService) Delete(id, actorID uint) error {
	target, err := s.require(id)
	if err != nil {
		return err
	}
	if err := s.requireIdle(id); err != nil {
		return err
	}
	if err := s.targets.DeleteWithCatalog(target); err != nil {
		return Internal("target.delete_failed", "Failed to delete scrape target", err)
	}
	s.recordAudit(actorID, "target.delete", target)
	return nil
}

func (s *TargetService) requireIdle(id uint) error {
	count, err := s.jobs.ActiveCount(id, 0)
	if err != nil {
		return Internal("target.job_check_failed", "Failed to check active scrape jobs", err)
	}
	if count > 0 {
		return Conflict("target.job_active", "Wait for active scrape jobs before changing this target")
	}
	return nil
}

func (s *TargetService) Browse(ctx context.Context, id uint, requestedPath string, refresh bool) (*DirectoryLevel, error) {
	target, err := s.require(id)
	if err != nil {
		return nil, err
	}
	path := strings.TrimSpace(requestedPath)
	if path == "" {
		path = target.RootPath
	}
	normalized, normalizeErr := s.normalizeTargetPath(target, path)
	if normalizeErr != nil || !openlist.IsWithinPath(target.RootPath, normalized) {
		return nil, Forbidden("target.path_outside_root", "Requested path is outside the scrape target root")
	}
	listing, err := s.listTargetDirectoryForBrowse(ctx, target, normalized, refresh)
	if err != nil {
		return nil, err
	}
	nodes := make([]DirectoryNode, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		nodes = append(nodes, DirectoryNode{Name: entry.Name, Path: entry.Path, IsDir: entry.IsDir, Size: entry.Size, Modified: entry.Modified})
	}
	return &DirectoryLevel{TargetID: target.ID, RootPath: target.RootPath, Path: normalized, Entries: nodes, Warnings: listing.Warnings}, nil
}

func (s *TargetService) BrowseConnection(ctx context.Context, id uint, requestedPath string, refresh bool) (*DirectoryLevel, error) {
	connection, err := s.connections.Find(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("connection.not_found", "OpenList connection not found")
	}
	if err != nil {
		return nil, Internal("target.connection_failed", "Failed to load OpenList connection", err)
	}
	if !connection.Enabled {
		return nil, Conflict("target.connection_disabled", "OpenList connection is disabled")
	}
	accountRoot, rootErr := openlist.NormalizeRemotePath(connection.BasePath)
	if rootErr != nil {
		return nil, Internal("target.connection_failed", "OpenList connection has an invalid account root", rootErr)
	}
	requestedPath = strings.TrimSpace(requestedPath)
	if requestedPath == "" {
		requestedPath = accountRoot
	}
	normalized, normalizeErr := openlist.NormalizeRemotePath(requestedPath)
	if normalizeErr != nil || !openlist.IsWithinPath(accountRoot, normalized) {
		return nil, Forbidden("target.path_outside_account", "Requested path is outside the OpenList account root")
	}
	token, err := s.cipher.Decrypt(connection.EncryptedToken)
	if err != nil {
		return nil, Internal("connection.decryption_failed", "Stored OpenList token cannot be decrypted", err)
	}
	listing, err := s.listOpenListDirectoryForBrowse(ctx, connection.BaseURL, token, normalized, refresh)
	if err != nil {
		return nil, mapOpenListError(err)
	}
	nodes := make([]DirectoryNode, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		nodes = append(nodes, DirectoryNode{Name: entry.Name, Path: entry.Path, IsDir: entry.IsDir, Size: entry.Size, Modified: entry.Modified})
	}
	return &DirectoryLevel{RootPath: accountRoot, Path: normalized, Entries: nodes, Warnings: listing.Warnings}, nil
}

func (s *TargetService) validate(ctx context.Context, request SaveTargetCommand, excludeID uint) (string, *uint, string, string, error) {
	libraryType := strings.ToLower(strings.TrimSpace(request.LibraryType))
	if libraryType != "movie" && libraryType != "tv" && libraryType != "anime" {
		return "", nil, "", "", BadRequest("target.invalid_library_type", "Scrape target media type is invalid")
	}
	source := strings.ToLower(strings.TrimSpace(request.SourceType))
	if source == "" {
		source = "openlist"
	}
	if source == "local" {
		rootPath, normalizeErr := s.local.Normalize(request.RootPath)
		if normalizeErr != nil {
			return "", nil, "", "", normalizeErr
		}
		if _, err := s.local.ListDirectory(ctx, rootPath, false); err != nil {
			return "", nil, "", "", err
		}
		if request.Enabled {
			targets, err := s.targets.List()
			if err != nil {
				return "", nil, "", "", Internal("target.list_failed", "Failed to validate local target overlap", err)
			}
			for index := range targets {
				existing := &targets[index]
				if existing.ID == excludeID || !existing.Enabled || sourceType(existing) != "local" {
					continue
				}
				if openlist.IsWithinPath(existing.RootPath, rootPath) || openlist.IsWithinPath(rootPath, existing.RootPath) {
					return "", nil, "", "", Conflict("target.local_root_overlap", "Local scrape target overlaps another enabled target")
				}
			}
		}
		return source, nil, "Local media", rootPath, nil
	}
	if source != "openlist" {
		return "", nil, "", "", BadRequest("target.invalid_source_type", "Scrape target source type is invalid")
	}
	if request.ConnectionID == 0 {
		return "", nil, "", "", BadRequest("target.connection_required", "OpenList connection is required")
	}
	connection, err := s.connections.Find(request.ConnectionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil, "", "", NotFound("connection.not_found", "OpenList connection not found")
	}
	if err != nil {
		return "", nil, "", "", Internal("target.connection_failed", "Failed to load target connection", err)
	}
	if !connection.Enabled {
		return "", nil, "", "", Conflict("target.connection_disabled", "OpenList connection is disabled")
	}
	rootPath, normalizeErr := openlist.NormalizeRemotePath(request.RootPath)
	if normalizeErr != nil {
		return "", nil, "", "", BadRequest("target.invalid_path", "Scrape target path is invalid")
	}
	if !openlist.IsWithinPath(connection.BasePath, rootPath) {
		return "", nil, "", "", Forbidden("target.path_outside_account", "Scrape target path is outside the OpenList account root")
	}
	token, err := s.cipher.Decrypt(connection.EncryptedToken)
	if err != nil {
		return "", nil, "", "", Internal("connection.decryption_failed", "Stored OpenList token cannot be decrypted", err)
	}
	if _, err := s.client.ListDirectory(ctx, connection.BaseURL, token, rootPath, false); err != nil {
		return "", nil, "", "", mapOpenListError(err)
	}
	connectionID := connection.ID
	return source, &connectionID, connection.Name, rootPath, nil
}

func (s *TargetService) LocalStatus() LocalStorageStatus { return s.local.Status() }

func (s *TargetService) BrowseLocal(ctx context.Context, requestedPath string) (*DirectoryLevel, error) {
	path := strings.TrimSpace(requestedPath)
	if path == "" {
		path = s.local.root
	}
	normalized, err := s.local.Normalize(path)
	if err != nil {
		return nil, err
	}
	entries, err := s.local.ListDirectory(ctx, normalized, false)
	if err != nil {
		return nil, err
	}
	nodes := make([]DirectoryNode, 0, len(entries))
	for _, entry := range entries {
		nodes = append(nodes, DirectoryNode{Name: entry.Name, Path: entry.Path, IsDir: entry.IsDir, Size: entry.Size, Modified: entry.Modified})
	}
	return &DirectoryLevel{RootPath: s.local.root, Path: normalized, Entries: nodes}, nil
}

func (s *TargetService) normalizeTargetPath(target *model.ScrapeTarget, value string) (string, error) {
	if sourceType(target) == "local" {
		return s.local.Normalize(value)
	}
	return openlist.NormalizeRemotePath(value)
}

func (s *TargetService) listTargetDirectoryForBrowse(ctx context.Context, target *model.ScrapeTarget, path string, refresh bool) (openlist.DirectoryListing, error) {
	if sourceType(target) == "local" {
		entries, err := s.local.ListDirectory(ctx, path, false)
		return openlist.DirectoryListing{Entries: entries}, err
	}
	connection, err := s.requireTargetConnection(target)
	if err != nil {
		return openlist.DirectoryListing{}, err
	}
	token, err := s.cipher.Decrypt(connection.EncryptedToken)
	if err != nil {
		return openlist.DirectoryListing{}, Internal("connection.decryption_failed", "Stored OpenList token cannot be decrypted", err)
	}
	listing, err := s.listOpenListDirectoryForBrowse(ctx, connection.BaseURL, token, path, refresh)
	if err != nil {
		return openlist.DirectoryListing{}, mapOpenListError(err)
	}
	return listing, nil
}

func (s *TargetService) listOpenListDirectoryForBrowse(ctx context.Context, baseURL, token, path string, refresh bool) (openlist.DirectoryListing, error) {
	if client, ok := s.client.(warningDirectoryBrowser); ok {
		return client.ListDirectoryWithWarnings(ctx, baseURL, token, path, refresh)
	}
	entries, err := s.client.ListDirectory(ctx, baseURL, token, path, refresh)
	return openlist.DirectoryListing{Entries: entries}, err
}

func (s *TargetService) requireTargetConnection(target *model.ScrapeTarget) (*model.OpenListConnection, error) {
	if target.ConnectionID == nil || *target.ConnectionID == 0 {
		return nil, Internal("target.connection_failed", "OpenList target has no connection", nil)
	}
	connection, err := s.connections.Find(*target.ConnectionID)
	if err != nil {
		return nil, Internal("target.connection_failed", "Failed to load target connection", err)
	}
	return connection, nil
}

func sourceType(target *model.ScrapeTarget) string {
	if strings.ToLower(strings.TrimSpace(target.SourceType)) == "local" {
		return "local"
	}
	return "openlist"
}

func (s *TargetService) require(id uint) (*model.ScrapeTarget, error) {
	target, err := s.targets.Find(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("target.not_found", "Scrape target not found")
	}
	if err != nil {
		return nil, Internal("target.lookup_failed", "Failed to load scrape target", err)
	}
	return target, nil
}

func (s *TargetService) recordAudit(actorID uint, action string, target *model.ScrapeTarget) {
	detail, _ := json.Marshal(map[string]interface{}{
		"source_type": sourceType(target), "connection_id": target.ConnectionID, "root_path": target.RootPath,
		"library_type": target.LibraryType, "rename_enabled": target.RenameEnabled,
	})
	_ = s.audit.Record(actorID, action, "scrape_target:"+target.Name, string(detail))
}

func targetResponse(target *model.ScrapeTarget, connectionName string) TargetResponse {
	return TargetResponse{
		ID: target.ID, SourceType: sourceType(target), ConnectionID: target.ConnectionID, ConnectionName: connectionName,
		Name: target.Name, RootPath: target.RootPath, LibraryType: target.LibraryType,
		RenameEnabled: target.RenameEnabled, Enabled: target.Enabled, CreatedAt: target.CreatedAt, UpdatedAt: target.UpdatedAt,
	}
}
