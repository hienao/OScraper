package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"openlistscraper/config"
	"openlistscraper/internal/logging"
	"openlistscraper/internal/router"
	"openlistscraper/pkg/cryptoutil"
	"openlistscraper/pkg/database"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}
	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	credentialCipher, err := cryptoutil.New(cfg.CredentialEncryptionKey)
	if err != nil {
		log.Fatalf("failed to initialize credential encryption: %v", err)
	}
	logManager, err := logging.NewManager(cfg)
	if err != nil {
		log.Fatalf("failed to initialize logs: %v", err)
	}
	defer logManager.Close()
	logging.SetDefaultManager(logManager)
	defer logging.SetDefaultManager(nil)

	engine := router.Setup(cfg, db, logManager, credentialCipher)
	server := &http.Server{Addr: ":" + cfg.ServerPort, Handler: engine, ReadHeaderTimeout: 10 * time.Second}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	logging.Info("server", "OpenlistScraper started", logging.Fields{"port": cfg.ServerPort, "environment": cfg.AppEnv})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			logging.Error("server", "HTTP server failed", logging.Fields{"error": err})
			log.Fatal(err)
		}
	case <-stop:
		logging.Info("server", "shutdown signal received", nil)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logging.Error("server", "graceful shutdown failed", logging.Fields{"error": err})
		}
	}
}
