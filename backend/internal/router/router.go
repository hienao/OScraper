package router

import (
	"context"
	"net/http"

	"oscraper/config"
	"oscraper/internal/handler"
	"oscraper/internal/logging"
	"oscraper/internal/middleware"
	"oscraper/internal/service"
	"oscraper/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthProvider func(context.Context) (report any, ready bool)

type Dependencies struct {
	Auth        *service.AuthService
	Connections *service.ConnectionService
	Targets     *service.TargetService
	Catalog     *service.CatalogService
	Settings    *service.SettingService
	Previews    *service.PreviewService
	Jobs        *service.JobService
	Batches     *service.BatchScrapeService
	JobRecords  *service.JobRecordSettingsService
	Logs        *service.LogService
	Health      HealthProvider
}

func New(cfg *config.Config, db *gorm.DB, logManager *logging.Manager, dependencies Dependencies) *gin.Engine {
	gin.SetMode(cfg.GinMode)
	router := gin.New()
	router.Use(logging.RequestIDMiddleware(), logging.AccessLogMiddleware(logManager), gin.Recovery(), corsMiddleware())

	authHandler := handler.NewAuthHandler(dependencies.Auth)
	connectionHandler := handler.NewConnectionHandler(dependencies.Connections)
	targetHandler := handler.NewTargetHandler(dependencies.Targets)
	catalogHandler := handler.NewCatalogHandler(dependencies.Catalog)
	settingHandler := handler.NewSettingHandler(dependencies.Settings)
	previewHandler := handler.NewPreviewHandler(dependencies.Previews)
	jobHandler := handler.NewJobHandler(dependencies.Jobs, dependencies.JobRecords)
	batchHandler := handler.NewBatchHandler(dependencies.Batches)
	logHandler := handler.NewLogHandler(logManager, db, dependencies.Logs)

	health := func(c *gin.Context) {
		if dependencies.Health == nil {
			response.Success(c, gin.H{"status": "ok"})
			return
		}
		report, ready := dependencies.Health(c.Request.Context())
		if !ready {
			c.JSON(http.StatusServiceUnavailable, response.Response{Code: -1, ErrorCode: "health.not_ready", Message: "service is not ready", Data: report})
			return
		}
		response.Success(c, report)
	}
	router.GET("/api/health", health)
	router.GET("/api/health/ready", health)
	router.GET("/api/health/live", func(c *gin.Context) { response.Success(c, gin.H{"status": "ok"}) })

	api := router.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", middleware.JWTAuth(cfg, db), authHandler.Logout)
			auth.POST("/setup-admin", middleware.JWTAuth(cfg, db), authHandler.SetupAdmin)
		}

		user := api.Group("/user", middleware.JWTAuth(cfg, db))
		{
			user.GET("/profile", authHandler.Profile)
			user.PUT("/password", middleware.AdminSetupComplete(), authHandler.ChangePassword)
		}

		connections := api.Group("/openlist-connections", middleware.JWTAuth(cfg, db), middleware.AdminSetupComplete(), middleware.AdminOnly())
		{
			connections.GET("", connectionHandler.List)
			connections.POST("", connectionHandler.Create)
			connections.POST("/test", connectionHandler.Test)
			connections.GET("/:id", connectionHandler.Get)
			connections.PUT("/:id", connectionHandler.Update)
			connections.DELETE("/:id", connectionHandler.Delete)
			connections.POST("/:id/test", connectionHandler.TestSaved)
			connections.POST("/:id/rotate-token", connectionHandler.RotateToken)
			connections.GET("/:id/tree", targetHandler.BrowseConnection)
		}

		targets := api.Group("/scrape-targets", middleware.JWTAuth(cfg, db), middleware.AdminSetupComplete(), middleware.AdminOnly())
		{
			targets.GET("", targetHandler.List)
			targets.POST("", targetHandler.Create)
			targets.GET("/:id", targetHandler.Get)
			targets.PUT("/:id", targetHandler.Update)
			targets.DELETE("/:id", targetHandler.Delete)
			targets.GET("/:id/tree", targetHandler.Browse)
			targets.GET("/:id/tree/children", targetHandler.Browse)
			targets.POST("/:id/scans", catalogHandler.Scan)
			targets.GET("/:id/scans/:scanId", catalogHandler.GetScan)
			targets.GET("/:id/candidates", catalogHandler.Candidates)
			targets.POST("/:id/previews", previewHandler.Create)
			targets.POST("/:id/previews/search", previewHandler.Search)
			targets.POST("/:id/previews/tmdb", previewHandler.Create)
			targets.GET("/:id/previews/:previewId", previewHandler.Get)
			targets.POST("/:id/jobs", jobHandler.Submit)
			targets.POST("/:id/batches", batchHandler.Create)
			targets.GET("/:id/batches/:batchId", batchHandler.Get)
			targets.POST("/:id/batches/:batchId/cancel", batchHandler.Cancel)
		}

		localStorage := api.Group("/local-storage", middleware.JWTAuth(cfg, db), middleware.AdminSetupComplete(), middleware.AdminOnly())
		{
			localStorage.GET("/status", targetHandler.LocalStatus)
			localStorage.GET("/tree", targetHandler.BrowseLocal)
		}

		jobs := api.Group("/scrape-jobs", middleware.JWTAuth(cfg, db), middleware.AdminSetupComplete(), middleware.AdminOnly())
		{
			jobs.GET("", jobHandler.List)
			jobs.GET("/settings", jobHandler.Settings)
			jobs.PUT("/settings", jobHandler.SaveSettings)
			jobs.GET("/:id", jobHandler.Get)
			jobs.GET("/:id/operations", jobHandler.Operations)
			jobs.POST("/:id/retry", jobHandler.Retry)
			jobs.POST("/:id/cancel", jobHandler.Cancel)
		}

		settings := api.Group("/settings", middleware.JWTAuth(cfg, db), middleware.AdminSetupComplete(), middleware.AdminOnly())
		{
			settings.GET("/scraping", settingHandler.GetScraping)
			settings.PUT("/scraping", settingHandler.SaveScraping)
			settings.POST("/scraping/test-tmdb", settingHandler.TestTMDB)
			settings.POST("/scraping/test-ai", settingHandler.TestAI)
		}

		admin := api.Group("/admin", middleware.JWTAuth(cfg, db), middleware.AdminSetupComplete(), middleware.AdminOnly())
		{
			admin.GET("/logs", logHandler.API)
			admin.GET("/logs/settings", logHandler.Settings)
			admin.PUT("/logs/settings", logHandler.SaveSettings)
			admin.DELETE("/logs/:type", logHandler.Clear)
			admin.GET("/application-logs", logHandler.Application)
			admin.GET("/audit-logs", logHandler.Audit)
		}
	}

	router.NoRoute(func(c *gin.Context) {
		response.Error(c, http.StatusNotFound, "http.not_found", "Route not found")
	})
	return router
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Origin") != "" {
			c.Header("Access-Control-Allow-Origin", "*")
		}
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, Idempotency-Key")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
