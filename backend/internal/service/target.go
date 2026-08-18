package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"openlistscraper/internal/model"
	"openlistscraper/internal/openlist"
	"openlistscraper/internal/repository"
	"openlistscraper/pkg/cryptoutil"

	"gorm.io/gorm"
)

type DirectoryBrowser interface {
	ListDirectory(ctx context.Context, baseURL, token, path string, refresh bool) ([]openlist.DirectoryEntry, error)
}

type TargetService struct {
	targets     *repository.TargetRepository
	connections *repository.ConnectionRepository
	audit       *repository.AuditRepository
	jobs        *repository.JobRepository
	cipher      *cryptoutil.Cipher
	client      DirectoryBrowser
}

type TargetRequest struct {
	ConnectionID  uint   `json:"connection_id" binding:"required"`
	Name          string `json:"name" binding:"required,min=1,max=100"`
	RootPath      string `json:"root_path" binding:"required,max=1000"`
	LibraryType   string `json:"library_type" binding:"required,oneof=movie tv anime"`
	RenameEnabled bool   `json:"rename_enabled"`
	Enabled       bool   `json:"enabled"`
}

type TargetResponse struct {
	ID             uint      `json:"id"`
	ConnectionID   uint      `json:"connection_id"`
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
	TargetID uint            `json:"target_id"`
	RootPath string          `json:"root_path"`
	Path     string          `json:"path"`
	Entries  []DirectoryNode `json:"entries"`
}

func NewTargetService(db *gorm.DB, cipher *cryptoutil.Cipher, client DirectoryBrowser) *TargetService {
	return &TargetService{
		targets: repository.NewTargetRepository(db), connections: repository.NewConnectionRepository(db),
		audit: repository.NewAuditRepository(db), jobs: repository.NewJobRepository(db), cipher: cipher, client: client,
	}
}

func (s *TargetService) Get(id uint) (*TargetResponse, error) {
	target, err := s.require(id)
	if err != nil {
		return nil, err
	}
	connection, err := s.connections.Find(target.ConnectionID)
	if err != nil {
		return nil, Internal("target.connection_failed", "Failed to load target connection", err)
	}
	response := targetResponse(target, connection.Name)
	return &response, nil
}

func (s *TargetService) List() ([]TargetResponse, error) {
	targets, err := s.targets.List()
	if err != nil {
		return nil, Internal("target.list_failed", "Failed to list scrape targets", err)
	}
	responses := make([]TargetResponse, 0, len(targets))
	for index := range targets {
		connection, connectionErr := s.connections.Find(targets[index].ConnectionID)
		if connectionErr != nil {
			return nil, Internal("target.connection_failed", "Failed to load target connection", connectionErr)
		}
		responses = append(responses, targetResponse(&targets[index], connection.Name))
	}
	return responses, nil
}

func (s *TargetService) Create(ctx context.Context, actorID uint, request TargetRequest) (*TargetResponse, error) {
	connection, rootPath, err := s.validate(ctx, request)
	if err != nil {
		return nil, err
	}
	target := &model.ScrapeTarget{
		ConnectionID: request.ConnectionID, Name: strings.TrimSpace(request.Name), RootPath: rootPath,
		LibraryType: strings.ToLower(request.LibraryType), RenameEnabled: request.RenameEnabled, Enabled: request.Enabled,
	}
	if err := s.targets.Create(target); err != nil {
		return nil, Internal("target.create_failed", "Failed to create scrape target", err)
	}
	s.recordAudit(actorID, "target.create", target)
	response := targetResponse(target, connection.Name)
	return &response, nil
}

func (s *TargetService) Update(ctx context.Context, id, actorID uint, request TargetRequest) (*TargetResponse, error) {
	target, err := s.require(id)
	if err != nil {
		return nil, err
	}
	if err := s.requireIdle(id); err != nil {
		return nil, err
	}
	connection, rootPath, err := s.validate(ctx, request)
	if err != nil {
		return nil, err
	}
	target.ConnectionID = request.ConnectionID
	target.Name = strings.TrimSpace(request.Name)
	target.RootPath = rootPath
	target.LibraryType = strings.ToLower(request.LibraryType)
	target.RenameEnabled = request.RenameEnabled
	target.Enabled = request.Enabled
	if err := s.targets.Update(target); err != nil {
		return nil, Internal("target.update_failed", "Failed to update scrape target", err)
	}
	s.recordAudit(actorID, "target.update", target)
	response := targetResponse(target, connection.Name)
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
	connection, err := s.connections.Find(target.ConnectionID)
	if err != nil {
		return nil, Internal("target.connection_failed", "Failed to load target connection", err)
	}
	path := strings.TrimSpace(requestedPath)
	if path == "" {
		path = target.RootPath
	}
	normalized, normalizeErr := openlist.NormalizeRemotePath(path)
	if normalizeErr != nil || !openlist.IsWithinPath(target.RootPath, normalized) {
		return nil, Forbidden("target.path_outside_root", "Requested path is outside the scrape target root")
	}
	token, err := s.cipher.Decrypt(connection.EncryptedToken)
	if err != nil {
		return nil, Internal("connection.decryption_failed", "Stored OpenList token cannot be decrypted", err)
	}
	entries, err := s.client.ListDirectory(ctx, connection.BaseURL, token, normalized, refresh)
	if err != nil {
		return nil, mapOpenListError(err)
	}
	nodes := make([]DirectoryNode, 0, len(entries))
	for _, entry := range entries {
		nodes = append(nodes, DirectoryNode{Name: entry.Name, Path: entry.Path, IsDir: entry.IsDir, Size: entry.Size, Modified: entry.Modified})
	}
	return &DirectoryLevel{TargetID: target.ID, RootPath: target.RootPath, Path: normalized, Entries: nodes}, nil
}

func (s *TargetService) validate(ctx context.Context, request TargetRequest) (*model.OpenListConnection, string, error) {
	libraryType := strings.ToLower(strings.TrimSpace(request.LibraryType))
	if libraryType != "movie" && libraryType != "tv" && libraryType != "anime" {
		return nil, "", BadRequest("target.invalid_library_type", "Scrape target media type is invalid")
	}
	connection, err := s.connections.Find(request.ConnectionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", NotFound("connection.not_found", "OpenList connection not found")
	}
	if err != nil {
		return nil, "", Internal("target.connection_failed", "Failed to load target connection", err)
	}
	if !connection.Enabled {
		return nil, "", Conflict("target.connection_disabled", "OpenList connection is disabled")
	}
	rootPath, normalizeErr := openlist.NormalizeRemotePath(request.RootPath)
	if normalizeErr != nil {
		return nil, "", BadRequest("target.invalid_path", "Scrape target path is invalid")
	}
	if !openlist.IsWithinPath(connection.BasePath, rootPath) {
		return nil, "", Forbidden("target.path_outside_account", "Scrape target path is outside the OpenList account root")
	}
	token, err := s.cipher.Decrypt(connection.EncryptedToken)
	if err != nil {
		return nil, "", Internal("connection.decryption_failed", "Stored OpenList token cannot be decrypted", err)
	}
	if _, err := s.client.ListDirectory(ctx, connection.BaseURL, token, rootPath, false); err != nil {
		return nil, "", mapOpenListError(err)
	}
	return connection, rootPath, nil
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
		"connection_id": target.ConnectionID, "root_path": target.RootPath,
		"library_type": target.LibraryType, "rename_enabled": target.RenameEnabled,
	})
	_ = s.audit.Record(actorID, action, "scrape_target:"+target.Name, string(detail))
}

func targetResponse(target *model.ScrapeTarget, connectionName string) TargetResponse {
	return TargetResponse{
		ID: target.ID, ConnectionID: target.ConnectionID, ConnectionName: connectionName,
		Name: target.Name, RootPath: target.RootPath, LibraryType: target.LibraryType,
		RenameEnabled: target.RenameEnabled, Enabled: target.Enabled, CreatedAt: target.CreatedAt, UpdatedAt: target.UpdatedAt,
	}
}
