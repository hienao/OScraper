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

type OpenListTester interface {
	TestConnection(ctx context.Context, baseURL, token string) (*openlist.Identity, error)
}

type ConnectionService struct {
	repo    *repository.ConnectionRepository
	targets *repository.TargetRepository
	audit   *repository.AuditRepository
	cipher  *cryptoutil.Cipher
	client  OpenListTester
}

type ConnectionRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=100"`
	BaseURL  string `json:"base_url" binding:"required,max=500"`
	Token    string `json:"token" binding:"required"`
	QPSLimit int    `json:"qps_limit" binding:"min=0,max=1000"`
	QPMLimit int    `json:"qpm_limit" binding:"min=0,max=60000"`
}

type ConnectionUpdateRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=100"`
	BaseURL  string `json:"base_url" binding:"required,max=500"`
	QPSLimit int    `json:"qps_limit" binding:"min=0,max=1000"`
	QPMLimit int    `json:"qpm_limit" binding:"min=0,max=60000"`
	Enabled  bool   `json:"enabled"`
}

type TokenRequest struct {
	Token string `json:"token" binding:"required"`
}

type TestConnectionRequest struct {
	BaseURL string `json:"base_url" binding:"required,max=500"`
	Token   string `json:"token" binding:"required"`
}

type ConnectionResponse struct {
	ID           uint       `json:"id"`
	Name         string     `json:"name"`
	BaseURL      string     `json:"base_url"`
	Username     string     `json:"username"`
	BasePath     string     `json:"base_path"`
	QPSLimit     int        `json:"qps_limit"`
	QPMLimit     int        `json:"qpm_limit"`
	Enabled      bool       `json:"enabled"`
	HasToken     bool       `json:"has_token"`
	TokenMask    string     `json:"token_mask"`
	LastTestedAt *time.Time `json:"last_tested_at,omitempty"`
	LastTestOK   bool       `json:"last_test_ok"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ConnectionTestResponse struct {
	OK       bool   `json:"ok"`
	Username string `json:"username"`
	BasePath string `json:"base_path"`
}

func NewConnectionService(db *gorm.DB, cipher *cryptoutil.Cipher, client OpenListTester) *ConnectionService {
	return &ConnectionService{
		repo: repository.NewConnectionRepository(db), targets: repository.NewTargetRepository(db), audit: repository.NewAuditRepository(db), cipher: cipher, client: client,
	}
}

func (s *ConnectionService) List() ([]ConnectionResponse, error) {
	connections, err := s.repo.List()
	if err != nil {
		return nil, Internal("connection.list_failed", "Failed to list OpenList connections", err)
	}
	responses := make([]ConnectionResponse, 0, len(connections))
	for index := range connections {
		responses = append(responses, connectionResponse(&connections[index]))
	}
	return responses, nil
}

func (s *ConnectionService) Get(id uint) (*ConnectionResponse, error) {
	connection, err := s.require(id)
	if err != nil {
		return nil, err
	}
	response := connectionResponse(connection)
	return &response, nil
}

func (s *ConnectionService) Test(ctx context.Context, request TestConnectionRequest) (*ConnectionTestResponse, error) {
	identity, err := s.client.TestConnection(ctx, request.BaseURL, strings.TrimSpace(request.Token))
	if err != nil {
		return nil, mapOpenListError(err)
	}
	return &ConnectionTestResponse{OK: true, Username: identity.Username, BasePath: identity.BasePath}, nil
}

func (s *ConnectionService) Create(ctx context.Context, actorID uint, request ConnectionRequest) (*ConnectionResponse, error) {
	baseURL, err := openlist.NormalizeBaseURL(request.BaseURL)
	if err != nil {
		return nil, mapOpenListError(err)
	}
	token := strings.TrimSpace(request.Token)
	identity, err := s.client.TestConnection(ctx, baseURL, token)
	if err != nil {
		return nil, mapOpenListError(err)
	}
	encrypted, err := s.cipher.Encrypt(token)
	if err != nil {
		return nil, Internal("connection.encryption_failed", "Failed to secure OpenList token", err)
	}
	now := time.Now()
	connection := &model.OpenListConnection{
		Name: strings.TrimSpace(request.Name), BaseURL: baseURL, EncryptedToken: encrypted,
		Username: identity.Username, BasePath: identity.BasePath, QPSLimit: request.QPSLimit,
		QPMLimit: request.QPMLimit, Enabled: true, LastTestedAt: &now, LastTestOK: true,
	}
	if err := s.repo.Create(connection); err != nil {
		return nil, Internal("connection.create_failed", "Failed to create OpenList connection", err)
	}
	s.recordAudit(actorID, "connection.create", connection, map[string]interface{}{"base_url": baseURL})
	response := connectionResponse(connection)
	return &response, nil
}

func (s *ConnectionService) Update(id, actorID uint, request ConnectionUpdateRequest) (*ConnectionResponse, error) {
	connection, err := s.require(id)
	if err != nil {
		return nil, err
	}
	baseURL, err := openlist.NormalizeBaseURL(request.BaseURL)
	if err != nil {
		return nil, mapOpenListError(err)
	}
	connection.Name = strings.TrimSpace(request.Name)
	connection.BaseURL = baseURL
	connection.QPSLimit = request.QPSLimit
	connection.QPMLimit = request.QPMLimit
	connection.Enabled = request.Enabled
	connection.LastTestOK = false
	if err := s.repo.Update(connection); err != nil {
		return nil, Internal("connection.update_failed", "Failed to update OpenList connection", err)
	}
	s.recordAudit(actorID, "connection.update", connection, map[string]interface{}{"base_url": baseURL, "enabled": request.Enabled})
	response := connectionResponse(connection)
	return &response, nil
}

func (s *ConnectionService) TestSaved(ctx context.Context, id uint) (*ConnectionTestResponse, error) {
	connection, err := s.require(id)
	if err != nil {
		return nil, err
	}
	token, err := s.cipher.Decrypt(connection.EncryptedToken)
	if err != nil {
		return nil, Internal("connection.decryption_failed", "Stored OpenList token cannot be decrypted", err)
	}
	now := time.Now()
	identity, testErr := s.client.TestConnection(ctx, connection.BaseURL, token)
	connection.LastTestedAt = &now
	connection.LastTestOK = testErr == nil
	if testErr == nil {
		connection.Username = identity.Username
		connection.BasePath = identity.BasePath
	}
	if err := s.repo.Update(connection); err != nil {
		return nil, Internal("connection.test_save_failed", "Failed to save connection test result", err)
	}
	if testErr != nil {
		return nil, mapOpenListError(testErr)
	}
	return &ConnectionTestResponse{OK: true, Username: identity.Username, BasePath: identity.BasePath}, nil
}

func (s *ConnectionService) RotateToken(ctx context.Context, id, actorID uint, request TokenRequest) (*ConnectionResponse, error) {
	connection, err := s.require(id)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(request.Token)
	identity, err := s.client.TestConnection(ctx, connection.BaseURL, token)
	if err != nil {
		return nil, mapOpenListError(err)
	}
	encrypted, err := s.cipher.Encrypt(token)
	if err != nil {
		return nil, Internal("connection.encryption_failed", "Failed to secure OpenList token", err)
	}
	now := time.Now()
	connection.EncryptedToken = encrypted
	connection.Username = identity.Username
	connection.BasePath = identity.BasePath
	connection.LastTestedAt = &now
	connection.LastTestOK = true
	if err := s.repo.Update(connection); err != nil {
		return nil, Internal("connection.rotate_failed", "Failed to rotate OpenList token", err)
	}
	s.recordAudit(actorID, "connection.rotate_token", connection, nil)
	response := connectionResponse(connection)
	return &response, nil
}

func (s *ConnectionService) Delete(id, actorID uint) error {
	connection, err := s.require(id)
	if err != nil {
		return err
	}
	targetCount, err := s.targets.CountByConnection(id)
	if err != nil {
		return Internal("connection.target_check_failed", "Failed to check connection usage", err)
	}
	if targetCount > 0 {
		return Conflict("connection.in_use", "Delete scrape targets that use this connection first")
	}
	if err := s.repo.Delete(connection); err != nil {
		return Internal("connection.delete_failed", "Failed to delete OpenList connection", err)
	}
	s.recordAudit(actorID, "connection.delete", connection, nil)
	return nil
}

func (s *ConnectionService) require(id uint) (*model.OpenListConnection, error) {
	connection, err := s.repo.Find(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("connection.not_found", "OpenList connection not found")
	}
	if err != nil {
		return nil, Internal("connection.lookup_failed", "Failed to load OpenList connection", err)
	}
	return connection, nil
}

func (s *ConnectionService) recordAudit(actorID uint, action string, connection *model.OpenListConnection, detail interface{}) {
	encoded := "{}"
	if detail != nil {
		if value, err := json.Marshal(detail); err == nil {
			encoded = string(value)
		}
	}
	_ = s.audit.Record(actorID, action, "openlist_connection:"+connection.Name, encoded)
}

func connectionResponse(connection *model.OpenListConnection) ConnectionResponse {
	return ConnectionResponse{
		ID: connection.ID, Name: connection.Name, BaseURL: connection.BaseURL,
		Username: connection.Username, BasePath: connection.BasePath, QPSLimit: connection.QPSLimit,
		QPMLimit: connection.QPMLimit, Enabled: connection.Enabled, HasToken: connection.EncryptedToken != "",
		TokenMask: "••••••••", LastTestedAt: connection.LastTestedAt, LastTestOK: connection.LastTestOK,
		CreatedAt: connection.CreatedAt, UpdatedAt: connection.UpdatedAt,
	}
}

func mapOpenListError(err error) error {
	var apiError *openlist.APIError
	if errors.As(err, &apiError) {
		return &Error{Status: 400, Code: apiError.Code, Message: apiError.Message, Cause: apiError.Cause}
	}
	return Internal("openlist.unknown_error", "OpenList request failed", err)
}
