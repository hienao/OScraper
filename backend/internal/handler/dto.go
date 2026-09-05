package handler

import "oscraper/internal/service"

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (r loginRequest) command() service.LoginCommand {
	return service.LoginCommand{Username: r.Username, Password: r.Password}
}

type setupAdminRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

func (r setupAdminRequest) command() service.SetupAdminCommand {
	return service.SetupAdminCommand{Username: r.Username, Password: r.Password}
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
}

func (r changePasswordRequest) command() service.ChangePasswordCommand {
	return service.ChangePasswordCommand{OldPassword: r.OldPassword, NewPassword: r.NewPassword}
}

type createConnectionRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=100"`
	BaseURL  string `json:"base_url" binding:"required,max=500"`
	Token    string `json:"token" binding:"required"`
	QPSLimit int    `json:"qps_limit" binding:"min=0,max=1000"`
	QPMLimit int    `json:"qpm_limit" binding:"min=0,max=60000"`
}

func (r createConnectionRequest) command() service.CreateConnectionCommand {
	return service.CreateConnectionCommand{Name: r.Name, BaseURL: r.BaseURL, Token: r.Token, QPSLimit: r.QPSLimit, QPMLimit: r.QPMLimit}
}

type updateConnectionRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=100"`
	BaseURL  string `json:"base_url" binding:"required,max=500"`
	QPSLimit int    `json:"qps_limit" binding:"min=0,max=1000"`
	QPMLimit int    `json:"qpm_limit" binding:"min=0,max=60000"`
	Enabled  bool   `json:"enabled"`
}

func (r updateConnectionRequest) command() service.UpdateConnectionCommand {
	return service.UpdateConnectionCommand{Name: r.Name, BaseURL: r.BaseURL, QPSLimit: r.QPSLimit, QPMLimit: r.QPMLimit, Enabled: r.Enabled}
}

type rotateTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

func (r rotateTokenRequest) command() service.RotateTokenCommand {
	return service.RotateTokenCommand{Token: r.Token}
}

type testConnectionRequest struct {
	BaseURL string `json:"base_url" binding:"required,max=500"`
	Token   string `json:"token" binding:"required"`
}

func (r testConnectionRequest) command() service.TestConnectionCommand {
	return service.TestConnectionCommand{BaseURL: r.BaseURL, Token: r.Token}
}

type targetRequest struct {
	SourceType    string `json:"source_type" binding:"omitempty,oneof=openlist local"`
	ConnectionID  uint   `json:"connection_id"`
	Name          string `json:"name" binding:"required,min=1,max=100"`
	RootPath      string `json:"root_path" binding:"required,max=1000"`
	LibraryType   string `json:"library_type" binding:"required,oneof=movie tv anime"`
	RenameEnabled bool   `json:"rename_enabled"`
	Enabled       bool   `json:"enabled"`
}

func (r targetRequest) command() service.SaveTargetCommand {
	return service.SaveTargetCommand{
		SourceType: r.SourceType, ConnectionID: r.ConnectionID, Name: r.Name, RootPath: r.RootPath,
		LibraryType: r.LibraryType, RenameEnabled: r.RenameEnabled, Enabled: r.Enabled,
	}
}

type previewSearchRequest struct {
	CandidateID uint   `json:"candidate_id" binding:"required"`
	Title       string `json:"title" binding:"max=500"`
	Year        int    `json:"year" binding:"omitempty,min=1870,max=2200"`
}

func (r previewSearchRequest) command() service.SearchPreviewCommand {
	return service.SearchPreviewCommand{CandidateID: r.CandidateID, Title: r.Title, Year: r.Year}
}

type createPreviewRequest struct {
	CandidateID        uint              `json:"candidate_id" binding:"required"`
	TMDBID             int               `json:"tmdb_id" binding:"omitempty,min=1"`
	Title              string            `json:"title" binding:"max=500"`
	Year               int               `json:"year" binding:"omitempty,min=1870,max=2200"`
	MovieVersionLabels map[string]string `json:"movie_version_labels" binding:"omitempty,max=8"`
}

func (r createPreviewRequest) command() service.CreatePreviewCommand {
	return service.CreatePreviewCommand{CandidateID: r.CandidateID, TMDBID: r.TMDBID, Title: r.Title, Year: r.Year, MovieVersionLabels: r.MovieVersionLabels}
}

type submitJobRequest struct {
	PreviewID                   uint   `json:"preview_id" binding:"required"`
	RenameMedia                 bool   `json:"rename_media"`
	ConfirmDirectoryFingerprint string `json:"confirm_directory_fingerprint" binding:"required,max=80"`
}

func (r submitJobRequest) command() service.SubmitJobCommand {
	return service.SubmitJobCommand{
		PreviewID: r.PreviewID, RenameMedia: r.RenameMedia,
		ConfirmDirectoryFingerprint: r.ConfirmDirectoryFingerprint,
	}
}

type createBatchRequest struct {
	ScanID         uint `json:"scan_id"`
	IncludeScraped bool `json:"include_scraped"`
}

func (r createBatchRequest) command() service.BatchScrapeCommand {
	return service.BatchScrapeCommand{ScanID: r.ScanID, IncludeScraped: r.IncludeScraped}
}

type scrapingSettingsRequest struct {
	APIKey       string `json:"api_key" binding:"max=500"`
	BaseURL      string `json:"base_url" binding:"required,max=500"`
	ImageBaseURL string `json:"image_base_url" binding:"required,max=500"`
	Language     string `json:"language" binding:"required,max=20"`
	Region       string `json:"region" binding:"max=2"`
	PosterSize   string `json:"poster_size" binding:"required,max=20"`
	BackdropSize string `json:"backdrop_size" binding:"required,max=20"`
	Timeout      int    `json:"timeout_seconds" binding:"required,min=1,max=120"`
	ProxyHost    string `json:"proxy_host" binding:"max=255"`
	ProxyPort    int    `json:"proxy_port" binding:"min=0,max=65535"`
	AIEnabled    bool   `json:"ai_enabled"`
	AIBaseURL    string `json:"ai_base_url" binding:"max=500"`
	AIAPIKey     string `json:"ai_api_key" binding:"max=500"`
	AIModel      string `json:"ai_model" binding:"max=200"`
	AIQPMLimit   int    `json:"ai_qpm_limit" binding:"min=0,max=1000"`
	AITimeout    int    `json:"ai_timeout_seconds" binding:"min=0,max=120"`
}

func (r scrapingSettingsRequest) command() service.SaveScrapingSettingsCommand {
	return service.SaveScrapingSettingsCommand{
		APIKey: r.APIKey, BaseURL: r.BaseURL, ImageBaseURL: r.ImageBaseURL, Language: r.Language,
		Region: r.Region, PosterSize: r.PosterSize, BackdropSize: r.BackdropSize, Timeout: r.Timeout,
		ProxyHost: r.ProxyHost, ProxyPort: r.ProxyPort, AIEnabled: r.AIEnabled, AIBaseURL: r.AIBaseURL,
		AIAPIKey: r.AIAPIKey, AIModel: r.AIModel, AIQPMLimit: r.AIQPMLimit, AITimeout: r.AITimeout,
	}
}
