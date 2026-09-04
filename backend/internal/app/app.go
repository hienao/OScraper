package app

import (
	"context"
	"errors"
	"time"

	"oscraper/config"
	"oscraper/internal/logging"
	"oscraper/internal/maintenance"
	"oscraper/internal/openlist"
	"oscraper/internal/provider/ai"
	"oscraper/internal/provider/tmdb"
	"oscraper/internal/router"
	"oscraper/internal/service"
	"oscraper/pkg/cryptoutil"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ComponentHealth struct {
	OK      bool `json:"ok"`
	Details any  `json:"details,omitempty"`
}

type HealthReport struct {
	Status     string                     `json:"status"`
	Time       time.Time                  `json:"time"`
	Components map[string]ComponentHealth `json:"components"`
}

type App struct {
	Engine      *gin.Engine
	Jobs        *service.JobService
	Batches     *service.BatchScrapeService
	Catalog     *service.CatalogService
	Maintenance *maintenance.Service
	Logs        *service.LogService
	JobRecords  *service.JobRecordSettingsService
	targets     *service.TargetService
	db          *gorm.DB
	logs        *logging.Manager
	cancel      context.CancelFunc
}

func New(cfg *config.Config, db *gorm.DB, logs *logging.Manager, cipher *cryptoutil.Cipher) (*App, error) {
	rootCtx, cancel := context.WithCancel(context.Background())
	openListClient := openlist.NewClient(time.Duration(cfg.HTTPTimeoutSeconds) * time.Second)
	quota := service.NewConnectionQuota()
	authService := service.NewAuthService(cfg, db)
	if err := authService.InitBootstrapAdmin(); err != nil {
		cancel()
		return nil, err
	}
	connectionService := service.NewConnectionService(db, cipher, openListClient)
	targetService := service.NewTargetService(db, cipher, openListClient, cfg.LocalMediaRoot)
	tmdbClient := tmdb.NewClient()
	aiClient := ai.NewClient()
	settingService := service.NewSettingService(db, cipher, tmdbClient, aiClient)
	recognitionService := service.NewAIRecognitionService(settingService, aiClient)
	catalogService := service.NewCatalogServiceWithRuntime(db, cipher, openListClient, quota, cfg.LocalMediaRoot, cfg.ScanWorkers, cfg.ScanQueueSize, recognitionService)
	if err := catalogService.Start(); err != nil {
		cancel()
		return nil, err
	}
	previewService := service.NewPreviewService(db, settingService, tmdbClient, catalogService)
	jobService, err := service.NewJobService(db, cfg, cipher, openListClient, catalogService, quota, settingService)
	if err != nil {
		cancel()
		_ = catalogService.Shutdown(context.Background())
		return nil, err
	}
	batchService := service.NewBatchScrapeService(db, settingService, previewService, jobService, cfg.ScanWorkers, cfg.ScanQueueSize)
	if err := batchService.Start(); err != nil {
		cancel()
		_ = jobService.Shutdown(context.Background())
		_ = catalogService.Shutdown(context.Background())
		return nil, err
	}
	logService := service.NewLogService(logs, db, cfg.LogRetentionDays)
	if err := logService.Start(rootCtx); err != nil {
		cancel()
		_ = jobService.Shutdown(context.Background())
		_ = catalogService.Shutdown(context.Background())
		return nil, err
	}
	maintenanceService := maintenance.New(db, cfg.DataRetentionDays, cfg.JobRetentionDays)
	jobRecordSettings := service.NewJobRecordSettingsService(db, maintenanceService, cfg.JobRetentionDays)
	if err := jobRecordSettings.Initialize(); err != nil {
		cancel()
		_ = logService.Shutdown(context.Background())
		_ = jobService.Shutdown(context.Background())
		_ = catalogService.Shutdown(context.Background())
		return nil, err
	}
	if err := maintenanceService.Start(rootCtx); err != nil {
		cancel()
		_ = logService.Shutdown(context.Background())
		_ = jobService.Shutdown(context.Background())
		_ = catalogService.Shutdown(context.Background())
		return nil, err
	}
	application := &App{Jobs: jobService, Batches: batchService, Catalog: catalogService, Maintenance: maintenanceService, Logs: logService, JobRecords: jobRecordSettings, targets: targetService, db: db, logs: logs, cancel: cancel}
	application.Engine = router.New(cfg, db, logs, router.Dependencies{
		Auth: authService, Connections: connectionService, Targets: targetService, Catalog: catalogService,
		Settings: settingService, Previews: previewService, Jobs: jobService, Batches: batchService, JobRecords: jobRecordSettings, Logs: logService, Health: application.Health,
	})
	return application, nil
}

func (a *App) Health(ctx context.Context) (any, bool) {
	businessOK := ping(ctx, a.db)
	logsOK := a.logs != nil && ping(ctx, a.logs.DB)
	var logDetails any
	if a.logs != nil {
		logDetails = a.logs.Stats()
	}
	local := a.targets.LocalStatus()
	ready := businessOK && logsOK
	status := "ok"
	if !ready {
		status = "degraded"
	}
	return HealthReport{
		Status: status, Time: time.Now().UTC(),
		Components: map[string]ComponentHealth{
			"database":    {OK: businessOK},
			"logging":     {OK: logsOK, Details: logDetails},
			"jobs":        {OK: true, Details: a.Jobs.Metrics()},
			"batches":     {OK: true, Details: a.Batches.Metrics()},
			"scans":       {OK: true, Details: a.Catalog.Metrics()},
			"maintenance": {OK: a.Maintenance.Status().LastError == "", Details: a.Maintenance.Status()},
			"local_media": {OK: local.Mounted && local.Readable, Details: local},
		},
	}, ready
}

func (a *App) Shutdown(ctx context.Context) error {
	a.cancel()
	return errors.Join(a.Batches.Shutdown(ctx), a.Catalog.Shutdown(ctx), a.Jobs.Shutdown(ctx), a.Logs.Shutdown(ctx), a.Maintenance.Shutdown(ctx))
}

func ping(ctx context.Context, db *gorm.DB) bool {
	if db == nil {
		return false
	}
	sqlDB, err := db.DB()
	return err == nil && sqlDB.PingContext(ctx) == nil
}
