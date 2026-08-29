package service

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"oscraper/internal/model"
	"oscraper/internal/provider/ai"
	"oscraper/internal/provider/tmdb"
	"oscraper/internal/repository"
	"oscraper/pkg/cryptoutil"

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
	settingTMDBProxyHost    = "tmdb.proxy_host"
	settingTMDBProxyPort    = "tmdb.proxy_port"
	settingAIEnabled        = "ai.enabled"
	settingAIBaseURL        = "ai.base_url"
	settingAIAPIKey         = "ai.api_key"
	settingAIModel          = "ai.model"
	settingAIQPMLimit       = "ai.qpm_limit"
	settingAITimeout        = "ai.timeout_seconds"
)

var (
	languagePattern  = regexp.MustCompile(`^[a-z]{2}(?:-[A-Z]{2})?$`)
	regionPattern    = regexp.MustCompile(`^(?:[A-Z]{2})?$`)
	imageSizePattern = regexp.MustCompile(`^(?:original|[wh]\d{2,4})$`)
	proxyHostPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)
)

type TMDBProvider interface {
	Test(ctx context.Context, config tmdb.Config) error
}

type AIProvider interface {
	Test(ctx context.Context, config ai.Config) error
}

type SettingService struct {
	repo     *repository.SettingRepository
	audit    *repository.AuditRepository
	cipher   *cryptoutil.Cipher
	provider TMDBProvider
	ai       AIProvider
}

type SaveScrapingSettingsCommand struct {
	APIKey       string
	BaseURL      string
	ImageBaseURL string
	Language     string
	Region       string
	PosterSize   string
	BackdropSize string
	Timeout      int
	ProxyHost    string
	ProxyPort    int
	AIEnabled    bool
	AIBaseURL    string
	AIAPIKey     string
	AIModel      string
	AIQPMLimit   int
	AITimeout    int
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
	ProxyHost    string `json:"proxy_host"`
	ProxyPort    int    `json:"proxy_port"`
	AIEnabled    bool   `json:"ai_enabled"`
	AIHasAPIKey  bool   `json:"ai_has_api_key"`
	AIAPIKeyMask string `json:"ai_api_key_mask,omitempty"`
	AIBaseURL    string `json:"ai_base_url"`
	AIModel      string `json:"ai_model"`
	AIQPMLimit   int    `json:"ai_qpm_limit"`
	AITimeout    int    `json:"ai_timeout_seconds"`
}

func NewSettingService(db *gorm.DB, cipher *cryptoutil.Cipher, provider TMDBProvider, aiProviders ...AIProvider) *SettingService {
	service := &SettingService{repo: repository.NewSettingRepository(db), audit: repository.NewAuditRepository(db), cipher: cipher, provider: provider}
	if len(aiProviders) > 0 {
		service.ai = aiProviders[0]
	}
	return service
}

func (s *SettingService) GetScraping() (*ScrapingSettingsResponse, error) {
	config, hasKey, err := s.TMDBConfig()
	if err != nil {
		return nil, err
	}
	aiConfig, aiHasKey, err := s.AIConfig()
	if err != nil {
		return nil, err
	}
	response := settingsResponse(config, hasKey, aiConfig, aiHasKey)
	return &response, nil
}

func (s *SettingService) SaveScraping(actorID uint, request SaveScrapingSettingsCommand) (*ScrapingSettingsResponse, error) {
	if strings.TrimSpace(request.AIBaseURL) == "" {
		request.AIBaseURL = "https://api.openai.com/v1"
	}
	if strings.TrimSpace(request.AIModel) == "" {
		request.AIModel = "gpt-4o-mini"
	}
	if request.AIQPMLimit == 0 {
		request.AIQPMLimit = 60
	}
	if request.AITimeout == 0 {
		request.AITimeout = 30
	}
	config := tmdb.Config{
		APIKey: strings.TrimSpace(request.APIKey), BaseURL: strings.TrimRight(strings.TrimSpace(request.BaseURL), "/"),
		ImageBaseURL: strings.TrimRight(strings.TrimSpace(request.ImageBaseURL), "/"), Language: strings.TrimSpace(request.Language),
		Region: strings.ToUpper(strings.TrimSpace(request.Region)), PosterSize: strings.TrimSpace(request.PosterSize),
		BackdropSize: strings.TrimSpace(request.BackdropSize), ProxyHost: strings.TrimSpace(request.ProxyHost), ProxyPort: request.ProxyPort,
		Timeout: time.Duration(request.Timeout) * time.Second,
	}
	aiConfig := ai.Config{
		Enabled: request.AIEnabled, BaseURL: strings.TrimRight(strings.TrimSpace(request.AIBaseURL), "/"),
		APIKey: strings.TrimSpace(request.AIAPIKey), Model: strings.TrimSpace(request.AIModel),
		QPMLimit: request.AIQPMLimit, Timeout: time.Duration(request.AITimeout) * time.Second,
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
	aiHasKey := aiConfig.APIKey != ""
	if !aiHasKey {
		existing, existingHasKey, err := s.AIConfig()
		if err != nil {
			return nil, err
		}
		aiConfig.APIKey = existing.APIKey
		aiHasKey = existingHasKey
	}
	if err := validateTMDBConfig(config); err != nil {
		return nil, err
	}
	if err := validateAIConfig(aiConfig, aiHasKey); err != nil {
		return nil, err
	}
	settings := []model.SystemSetting{
		{Key: settingTMDBBaseURL, Value: config.BaseURL},
		{Key: settingTMDBImageBaseURL, Value: config.ImageBaseURL},
		{Key: settingTMDBLanguage, Value: config.Language},
		{Key: settingTMDBRegion, Value: config.Region},
		{Key: settingTMDBPosterSize, Value: config.PosterSize},
		{Key: settingTMDBBackdropSize, Value: config.BackdropSize},
		{Key: settingTMDBTimeout, Value: strconv.Itoa(request.Timeout)},
		{Key: settingTMDBProxyHost, Value: config.ProxyHost},
		{Key: settingTMDBProxyPort, Value: strconv.Itoa(config.ProxyPort)},
		{Key: settingAIEnabled, Value: strconv.FormatBool(aiConfig.Enabled)},
		{Key: settingAIBaseURL, Value: aiConfig.BaseURL},
		{Key: settingAIModel, Value: aiConfig.Model},
		{Key: settingAIQPMLimit, Value: strconv.Itoa(aiConfig.QPMLimit)},
		{Key: settingAITimeout, Value: strconv.Itoa(int(aiConfig.Timeout / time.Second))},
	}
	if strings.TrimSpace(request.APIKey) != "" {
		encrypted, err := s.cipher.Encrypt(strings.TrimSpace(request.APIKey))
		if err != nil {
			return nil, Internal("setting.encryption_failed", "Failed to secure TMDB API key", err)
		}
		settings = append(settings, model.SystemSetting{Key: settingTMDBAPIKey, Value: encrypted, IsSecret: true})
	}
	if strings.TrimSpace(request.AIAPIKey) != "" {
		encrypted, err := s.cipher.Encrypt(strings.TrimSpace(request.AIAPIKey))
		if err != nil {
			return nil, Internal("setting.encryption_failed", "Failed to secure AI API key", err)
		}
		settings = append(settings, model.SystemSetting{Key: settingAIAPIKey, Value: encrypted, IsSecret: true})
	}
	if err := s.repo.Upsert(settings); err != nil {
		return nil, Internal("setting.save_failed", "Failed to save scraping settings", err)
	}
	detail, _ := json.Marshal(map[string]any{
		"base_url": config.BaseURL, "language": config.Language, "region": config.Region,
		"proxy_enabled": config.ProxyHost != "", "api_key_updated": strings.TrimSpace(request.APIKey) != "",
		"ai_enabled": aiConfig.Enabled, "ai_model": aiConfig.Model, "ai_api_key_updated": strings.TrimSpace(request.AIAPIKey) != "",
	})
	_ = s.audit.Record(actorID, "setting.scraping.update", "system_settings:scraping", string(detail))
	response := settingsResponse(config, hasKey, aiConfig, aiHasKey)
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

func (s *SettingService) TestAI(ctx context.Context) error {
	config, hasKey, err := s.AIConfig()
	if err != nil {
		return err
	}
	if !config.Enabled || !hasKey {
		return Conflict("ai.not_configured", "AI media recognition is not enabled and configured")
	}
	if s.ai == nil {
		return Internal("ai.unavailable", "AI media recognition provider is unavailable", nil)
	}
	if err := s.ai.Test(ctx, config); err != nil {
		return mapAIError(err)
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
		settingTMDBProxyHost: &config.ProxyHost,
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
	if setting, err := s.repo.Get(settingTMDBProxyPort); err == nil {
		if port, parseErr := strconv.Atoi(setting.Value); parseErr == nil && port >= 0 && port <= 65535 {
			config.ProxyPort = port
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

func (s *SettingService) AIConfig() (ai.Config, bool, error) {
	config := ai.Config{BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini", QPMLimit: 60, Timeout: 30 * time.Second}
	stringValues := map[string]*string{settingAIBaseURL: &config.BaseURL, settingAIModel: &config.Model}
	for key, destination := range stringValues {
		setting, err := s.repo.Get(key)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return ai.Config{}, false, Internal("setting.load_failed", "Failed to load AI settings", err)
		}
		*destination = setting.Value
	}
	if setting, err := s.repo.Get(settingAIEnabled); err == nil {
		config.Enabled, _ = strconv.ParseBool(setting.Value)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ai.Config{}, false, Internal("setting.load_failed", "Failed to load AI settings", err)
	}
	if setting, err := s.repo.Get(settingAIQPMLimit); err == nil {
		if limit, parseErr := strconv.Atoi(setting.Value); parseErr == nil && limit >= 1 && limit <= 1000 {
			config.QPMLimit = limit
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ai.Config{}, false, Internal("setting.load_failed", "Failed to load AI settings", err)
	}
	if setting, err := s.repo.Get(settingAITimeout); err == nil {
		if seconds, parseErr := strconv.Atoi(setting.Value); parseErr == nil && seconds >= 1 && seconds <= 120 {
			config.Timeout = time.Duration(seconds) * time.Second
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ai.Config{}, false, Internal("setting.load_failed", "Failed to load AI settings", err)
	}
	setting, err := s.repo.Get(settingAIAPIKey)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return config, false, nil
	}
	if err != nil {
		return ai.Config{}, false, Internal("setting.load_failed", "Failed to load AI settings", err)
	}
	apiKey, err := s.cipher.Decrypt(setting.Value)
	if err != nil {
		return ai.Config{}, false, Internal("setting.decryption_failed", "Stored AI API key cannot be decrypted", err)
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
	if config.ProxyHost == "" && config.ProxyPort != 0 || config.ProxyHost != "" && (config.ProxyPort < 1 || config.ProxyPort > 65535) {
		return BadRequest("tmdb.invalid_proxy", "TMDB proxy host and port must be configured together")
	}
	if config.ProxyHost != "" && net.ParseIP(config.ProxyHost) == nil && !proxyHostPattern.MatchString(config.ProxyHost) {
		return BadRequest("tmdb.invalid_proxy", "TMDB proxy host is invalid")
	}
	return nil
}

func validateAIConfig(config ai.Config, hasKey bool) error {
	parsed, err := url.Parse(config.BaseURL)
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return BadRequest("ai.invalid_url", "AI base URL is invalid")
	}
	if strings.TrimSpace(config.Model) == "" || len(config.Model) > 200 {
		return BadRequest("ai.invalid_model", "AI model is invalid")
	}
	if config.QPMLimit < 1 || config.QPMLimit > 1000 {
		return BadRequest("ai.invalid_qpm", "AI QPM limit must be between 1 and 1000")
	}
	if config.Timeout < time.Second || config.Timeout > 120*time.Second {
		return BadRequest("ai.invalid_timeout", "AI timeout must be between 1 and 120 seconds")
	}
	if config.Enabled && !hasKey {
		return Conflict("ai.not_configured", "Configure an AI API key before enabling media recognition")
	}
	return nil
}

func settingsResponse(config tmdb.Config, hasKey bool, aiConfig ai.Config, aiHasKey bool) ScrapingSettingsResponse {
	response := ScrapingSettingsResponse{
		HasAPIKey: hasKey, BaseURL: config.BaseURL, ImageBaseURL: config.ImageBaseURL,
		Language: config.Language, Region: config.Region, PosterSize: config.PosterSize,
		BackdropSize: config.BackdropSize, Timeout: int(config.Timeout / time.Second), ProxyHost: config.ProxyHost, ProxyPort: config.ProxyPort,
		AIEnabled: aiConfig.Enabled, AIHasAPIKey: aiHasKey, AIBaseURL: aiConfig.BaseURL, AIModel: aiConfig.Model,
		AIQPMLimit: aiConfig.QPMLimit, AITimeout: int(aiConfig.Timeout / time.Second),
	}
	if hasKey {
		response.APIKeyMask = "••••••••"
	}
	if aiHasKey {
		response.AIAPIKeyMask = "••••••••"
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

func mapAIError(err error) error {
	var providerError *ai.Error
	if errors.As(err, &providerError) {
		status := 400
		if providerError.Code == "ai.rate_limited" {
			status = 429
		}
		return &Error{Status: status, Code: providerError.Code, Message: providerError.Message, Cause: providerError.Cause}
	}
	return Internal("ai.unknown_error", "AI media recognition request failed", err)
}
