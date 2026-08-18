package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"openlistscraper/internal/model"
	"openlistscraper/internal/provider/tmdb"
	"openlistscraper/internal/repository"
	"openlistscraper/pkg/cryptoutil"

	"gorm.io/gorm"
)

const (
	settingTMDBAPIKey       = "tmdb.api_key"
	settingTMDBBaseURL      = "tmdb.base_url"
	settingTMDBImageBaseURL = "tmdb.image_base_url"
	settingTMDBLanguage     = "tmdb.language"
	settingTMDBRegion       = "tmdb.region"
	settingTMDBPosterSize   = "tmdb.poster_size"
	settingTMDBBackdropSize = "tmdb.backdrop_size"
	settingTMDBTimeout      = "tmdb.timeout_seconds"
)

var (
	languagePattern  = regexp.MustCompile(`^[a-z]{2}(?:-[A-Z]{2})?$`)
	regionPattern    = regexp.MustCompile(`^(?:[A-Z]{2})?$`)
	imageSizePattern = regexp.MustCompile(`^(?:original|[wh]\d{2,4})$`)
)

type TMDBProvider interface {
	Test(ctx context.Context, config tmdb.Config) error
}

type SettingService struct {
	repo     *repository.SettingRepository
	audit    *repository.AuditRepository
	cipher   *cryptoutil.Cipher
	provider TMDBProvider
}

type ScrapingSettingsRequest struct {
	APIKey       string `json:"api_key" binding:"max=500"`
	BaseURL      string `json:"base_url" binding:"required,max=500"`
	ImageBaseURL string `json:"image_base_url" binding:"required,max=500"`
	Language     string `json:"language" binding:"required,max=20"`
	Region       string `json:"region" binding:"max=2"`
	PosterSize   string `json:"poster_size" binding:"required,max=20"`
	BackdropSize string `json:"backdrop_size" binding:"required,max=20"`
	Timeout      int    `json:"timeout_seconds" binding:"required,min=1,max=120"`
}

type ScrapingSettingsResponse struct {
	HasAPIKey    bool   `json:"has_api_key"`
	APIKeyMask   string `json:"api_key_mask,omitempty"`
	BaseURL      string `json:"base_url"`
	ImageBaseURL string `json:"image_base_url"`
	Language     string `json:"language"`
	Region       string `json:"region"`
	PosterSize   string `json:"poster_size"`
	BackdropSize string `json:"backdrop_size"`
	Timeout      int    `json:"timeout_seconds"`
}

func NewSettingService(db *gorm.DB, cipher *cryptoutil.Cipher, provider TMDBProvider) *SettingService {
	return &SettingService{repo: repository.NewSettingRepository(db), audit: repository.NewAuditRepository(db), cipher: cipher, provider: provider}
}

func (s *SettingService) GetScraping() (*ScrapingSettingsResponse, error) {
	config, hasKey, err := s.TMDBConfig()
	if err != nil {
		return nil, err
	}
	response := settingsResponse(config, hasKey)
	return &response, nil
}

func (s *SettingService) SaveScraping(actorID uint, request ScrapingSettingsRequest) (*ScrapingSettingsResponse, error) {
	config := tmdb.Config{
		APIKey: strings.TrimSpace(request.APIKey), BaseURL: strings.TrimRight(strings.TrimSpace(request.BaseURL), "/"),
		ImageBaseURL: strings.TrimRight(strings.TrimSpace(request.ImageBaseURL), "/"), Language: strings.TrimSpace(request.Language),
		Region: strings.ToUpper(strings.TrimSpace(request.Region)), PosterSize: strings.TrimSpace(request.PosterSize),
		BackdropSize: strings.TrimSpace(request.BackdropSize), Timeout: time.Duration(request.Timeout) * time.Second,
	}
	if err := validateTMDBConfig(config); err != nil {
		return nil, err
	}
	hasKey := config.APIKey != ""
	if !hasKey {
		existing, existingHasKey, err := s.TMDBConfig()
		if err != nil {
			return nil, err
		}
		config.APIKey = existing.APIKey
		hasKey = existingHasKey
	}
	settings := []model.SystemSetting{
		{Key: settingTMDBBaseURL, Value: config.BaseURL},
		{Key: settingTMDBImageBaseURL, Value: config.ImageBaseURL},
		{Key: settingTMDBLanguage, Value: config.Language},
		{Key: settingTMDBRegion, Value: config.Region},
		{Key: settingTMDBPosterSize, Value: config.PosterSize},
		{Key: settingTMDBBackdropSize, Value: config.BackdropSize},
		{Key: settingTMDBTimeout, Value: strconv.Itoa(request.Timeout)},
	}
	if strings.TrimSpace(request.APIKey) != "" {
		encrypted, err := s.cipher.Encrypt(strings.TrimSpace(request.APIKey))
		if err != nil {
			return nil, Internal("setting.encryption_failed", "Failed to secure TMDB API key", err)
		}
		settings = append(settings, model.SystemSetting{Key: settingTMDBAPIKey, Value: encrypted, IsSecret: true})
	}
	if err := s.repo.Upsert(settings); err != nil {
		return nil, Internal("setting.save_failed", "Failed to save scraping settings", err)
	}
	detail, _ := json.Marshal(map[string]any{"base_url": config.BaseURL, "language": config.Language, "region": config.Region, "api_key_updated": strings.TrimSpace(request.APIKey) != ""})
	_ = s.audit.Record(actorID, "setting.tmdb.update", "system_settings:tmdb", string(detail))
	response := settingsResponse(config, hasKey)
	return &response, nil
}

func (s *SettingService) TestTMDB(ctx context.Context) error {
	config, hasKey, err := s.TMDBConfig()
	if err != nil {
		return err
	}
	if !hasKey {
		return Conflict("tmdb.not_configured", "TMDB API key is not configured")
	}
	if err := s.provider.Test(ctx, config); err != nil {
		return mapTMDBError(err)
	}
	return nil
}

func (s *SettingService) TMDBConfig() (tmdb.Config, bool, error) {
	config := tmdb.Config{
		BaseURL: "https://api.themoviedb.org", ImageBaseURL: "https://image.tmdb.org",
		Language: "zh-CN", PosterSize: "w500", BackdropSize: "w1280", Timeout: 20 * time.Second,
	}
	values := map[string]*string{
		settingTMDBBaseURL: &config.BaseURL, settingTMDBImageBaseURL: &config.ImageBaseURL,
		settingTMDBLanguage: &config.Language, settingTMDBRegion: &config.Region,
		settingTMDBPosterSize: &config.PosterSize, settingTMDBBackdropSize: &config.BackdropSize,
	}
	for key, destination := range values {
		setting, err := s.repo.Get(key)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return tmdb.Config{}, false, Internal("setting.load_failed", "Failed to load scraping settings", err)
		}
		*destination = setting.Value
	}
	if setting, err := s.repo.Get(settingTMDBTimeout); err == nil {
		if seconds, parseErr := strconv.Atoi(setting.Value); parseErr == nil && seconds >= 1 && seconds <= 120 {
			config.Timeout = time.Duration(seconds) * time.Second
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return tmdb.Config{}, false, Internal("setting.load_failed", "Failed to load scraping settings", err)
	}
	setting, err := s.repo.Get(settingTMDBAPIKey)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return config, false, nil
	}
	if err != nil {
		return tmdb.Config{}, false, Internal("setting.load_failed", "Failed to load scraping settings", err)
	}
	apiKey, err := s.cipher.Decrypt(setting.Value)
	if err != nil {
		return tmdb.Config{}, false, Internal("setting.decryption_failed", "Stored TMDB API key cannot be decrypted", err)
	}
	config.APIKey = apiKey
	return config, strings.TrimSpace(apiKey) != "", nil
}

func validateTMDBConfig(config tmdb.Config) error {
	for _, item := range []struct {
		value string
		code  string
	}{
		{config.BaseURL, "tmdb.invalid_url"}, {config.ImageBaseURL, "tmdb.invalid_image_url"},
	} {
		parsed, err := url.Parse(item.value)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return BadRequest(item.code, "TMDB URL is invalid")
		}
	}
	if !languagePattern.MatchString(config.Language) {
		return BadRequest("tmdb.invalid_language", "TMDB language must use a code such as zh-CN")
	}
	if !regionPattern.MatchString(config.Region) {
		return BadRequest("tmdb.invalid_region", "TMDB region must be an ISO 3166-1 alpha-2 code")
	}
	if !imageSizePattern.MatchString(config.PosterSize) || !imageSizePattern.MatchString(config.BackdropSize) {
		return BadRequest("tmdb.invalid_image_size", "TMDB image size is invalid")
	}
	return nil
}

func settingsResponse(config tmdb.Config, hasKey bool) ScrapingSettingsResponse {
	response := ScrapingSettingsResponse{
		HasAPIKey: hasKey, BaseURL: config.BaseURL, ImageBaseURL: config.ImageBaseURL,
		Language: config.Language, Region: config.Region, PosterSize: config.PosterSize,
		BackdropSize: config.BackdropSize, Timeout: int(config.Timeout / time.Second),
	}
	if hasKey {
		response.APIKeyMask = "••••••••"
	}
	return response
}

func mapTMDBError(err error) error {
	var providerError *tmdb.Error
	if errors.As(err, &providerError) {
		status := 400
		if providerError.Code == "tmdb.rate_limited" {
			status = 429
		}
		return &Error{Status: status, Code: providerError.Code, Message: providerError.Message, Cause: providerError.Cause}
	}
	return Internal("tmdb.unknown_error", "TMDB request failed", err)
}
