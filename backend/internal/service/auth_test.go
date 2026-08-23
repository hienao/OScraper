package service

import (
	"testing"

	"oscraper/config"
	"oscraper/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestAuthService(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Load()
	return NewAuthService(cfg, db), db
}

func TestBootstrapAdminMustCompleteSetup(t *testing.T) {
	service, _ := newTestAuthService(t)
	if err := service.InitBootstrapAdmin(); err != nil {
		t.Fatal(err)
	}
	login, err := service.Login(LoginCommand{Username: "admin", Password: "admin"})
	if err != nil || login.Token == "" {
		t.Fatalf("bootstrap login failed: %v", err)
	}
	profile, err := service.Profile(1)
	if err != nil || !profile.RequiresAdminSetup {
		t.Fatalf("expected setup to be required: %#v %v", profile, err)
	}
	if _, err := service.SetupAdmin(1, SetupAdminCommand{Username: "owner", Password: "secure-password"}); err != nil {
		t.Fatal(err)
	}
	profile, err = service.Profile(1)
	if err != nil || profile.RequiresAdminSetup || profile.Username != "owner" {
		t.Fatalf("unexpected configured profile: %#v %v", profile, err)
	}
}
