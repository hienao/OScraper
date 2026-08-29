package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"oscraper/internal/model"
	"oscraper/internal/provider/ai"
	"oscraper/internal/provider/tmdb"
	"oscraper/pkg/cryptoutil"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubTMDBTester struct{ err error }

func (s stubTMDBTester) Test(context.Context, tmdb.Config) error { return s.err }

type stubAITester struct{ err error }

func (s stubAITester) Test(context.Context, ai.Config) error { return s.err }

func newSettingTestService(t *testing.T) (*SettingService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:settings-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.AdminAuditLog{}); err != nil {
		t.Fatal(err)
	}
	cipher, err := cryptoutil.New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	return NewSettingService(db, cipher, stubTMDBTester{}), db
}

func TestSaveScrapingEncryptsAndNeverReturnsAPIKey(t *testing.T) {
	settings, db := newSettingTestService(t)
	response, err := settings.SaveScraping(1, SaveScrapingSettingsCommand{
		APIKey: "plain-secret", BaseURL: "https://api.themoviedb.org", ImageBaseURL: "https://image.tmdb.org",
		Language: "zh-CN", Region: "CN", PosterSize: "w500", BackdropSize: "w1280", Timeout: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !response.HasAPIKey || response.APIKeyMask == "plain-secret" {
		t.Fatalf("secret leaked in response: %#v", response)
	}
	var stored model.SystemSetting
	if err := db.First(&stored, "key = ?", settingTMDBAPIKey).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Value == "plain-secret" || !stored.IsSecret {
		t.Fatalf("API key was not encrypted: %#v", stored)
	}
	_, err = settings.SaveScraping(1, SaveScrapingSettingsCommand{
		BaseURL: "https://api.themoviedb.org", ImageBaseURL: "https://image.tmdb.org",
		Language: "en-US", PosterSize: "w500", BackdropSize: "original", Timeout: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	config, hasKey, err := settings.TMDBConfig()
	if err != nil || !hasKey || config.APIKey != "plain-secret" || config.Language != "en-US" {
		t.Fatalf("blank update did not preserve key: %#v %t %v", config, hasKey, err)
	}
}

func TestSaveScrapingRejectsInvalidLanguage(t *testing.T) {
	settings, _ := newSettingTestService(t)
	_, err := settings.SaveScraping(1, SaveScrapingSettingsCommand{
		BaseURL: "https://api.themoviedb.org", ImageBaseURL: "https://image.tmdb.org",
		Language: "chinese", PosterSize: "w500", BackdropSize: "w1280", Timeout: 20,
	})
	if err == nil {
		t.Fatal("expected invalid language to fail")
	}
}

func TestSaveScrapingEncryptsAIKeyAndPersistsProxy(t *testing.T) {
	settings, db := newSettingTestService(t)
	settings.ai = stubAITester{}
	response, err := settings.SaveScraping(1, SaveScrapingSettingsCommand{
		APIKey: "tmdb-secret", BaseURL: "https://api.tmdb.org", ImageBaseURL: "https://image.tmdb.org",
		Language: "zh-CN", PosterSize: "w500", BackdropSize: "w1280", Timeout: 20,
		ProxyHost: "proxy.local", ProxyPort: 7890, AIEnabled: true, AIAPIKey: "ai-secret",
		AIBaseURL: "https://api.openai.com/v1", AIModel: "gpt-4o-mini", AIQPMLimit: 45, AITimeout: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !response.AIHasAPIKey || response.AIAPIKeyMask == "ai-secret" || response.ProxyHost != "proxy.local" || response.ProxyPort != 7890 {
		t.Fatalf("unexpected settings response: %#v", response)
	}
	var stored model.SystemSetting
	if err := db.First(&stored, "key = ?", settingAIAPIKey).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Value == "ai-secret" || !stored.IsSecret {
		t.Fatalf("AI API key was not encrypted: %#v", stored)
	}
	config, hasKey, err := settings.AIConfig()
	if err != nil || !hasKey || config.APIKey != "ai-secret" || config.QPMLimit != 45 || config.Timeout != 25*time.Second {
		t.Fatalf("unexpected stored AI config: %#v %t %v", config, hasKey, err)
	}
}
