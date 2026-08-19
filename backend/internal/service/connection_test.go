package service

import (
	"context"
	"fmt"
	"testing"

	"oscraper/internal/model"
	"oscraper/internal/openlist"
	"oscraper/pkg/cryptoutil"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubOpenListTester struct {
	identity *openlist.Identity
	err      error
}

func (s stubOpenListTester) TestConnection(context.Context, string, string) (*openlist.Identity, error) {
	return s.identity, s.err
}

func TestCreateConnectionEncryptsTokenAndReturnsMask(t *testing.T) {
	dsn := fmt.Sprintf("file:connection-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.OpenListConnection{}, &model.ScrapeTarget{}, &model.ScanRun{}, &model.MediaCandidate{}, &model.ScrapePreview{}, &model.ScrapeJob{}, &model.ScrapeJobOperation{}, &model.AdminAuditLog{}); err != nil {
		t.Fatal(err)
	}
	cipher, err := cryptoutil.New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	connectionService := NewConnectionService(db, cipher, stubOpenListTester{identity: &openlist.Identity{Username: "alice", BasePath: "/media"}})
	created, err := connectionService.Create(context.Background(), 1, ConnectionRequest{
		Name: "Home", BaseURL: "http://openlist.example:5244/", Token: "plain-token", QPSLimit: 5, QPMLimit: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.TokenMask == "plain-token" || !created.HasToken || created.BaseURL != "http://openlist.example:5244" {
		t.Fatalf("unexpected response: %#v", created)
	}
	var stored model.OpenListConnection
	if err := db.First(&stored, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.EncryptedToken == "plain-token" {
		t.Fatal("token was stored in plaintext")
	}
	decrypted, err := cipher.Decrypt(stored.EncryptedToken)
	if err != nil || decrypted != "plain-token" {
		t.Fatalf("unexpected encrypted token: %q %v", decrypted, err)
	}
}
