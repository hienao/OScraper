package router

import (
	"net/http"
	"time"

	"openlistscraper/config"
	"openlistscraper/internal/handler"
	"openlistscraper/internal/logging"
	"openlistscraper/internal/middleware"
	"openlistscraper/internal/openlist"
	"openlistscraper/internal/provider/tmdb"
	"openlistscraper/internal/service"
	"openlistscraper/pkg/cryptoutil"
	"openlistscraper/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Setup(cfg *config.Config, db *gorm.DB, logManager *logging.Manager, credentialCipher *cryptoutil.Cipher) *gin.Engine {
	gin.SetMode(cfg.GinMode)
	router := gin.New()
	router.Use(logging.RequestIDMiddleware(), logging.AccessLogMiddleware(logManager), gin.Recovery(), corsMiddleware())

	authService := service.NewAuthService(cfg, db)
	if err := authService.InitBootstrapAdmin(); err != nil {
		panic("failed to initialize bootstrap administrator: " + err.Error())
	}
	openListClient := openlist.NewClient(time.Duration(cfg.HTTPTimeoutSeconds) * time.Second)
	connectionService := service.NewConnectionService(db, credentialCipher, openListClient)
	targetService := service.NewTargetService(db, credentialCipher, openListClient)
	catalogService := service.NewCatalogService(db, credentialCipher, openListClient)
	tmdbClient := tmdb.NewClient()
	settingService := service.NewSettingService(db, credentialCipher, tmdbClient)
	previewService := service.NewPreviewService(db, settingService, tmdbClient)

	authHandler := handler.NewAuthHandler(authService)
	connectionHandler := handler.NewConnectionHandler(connectionService)
	targetHandler := handler.NewTargetHandler(targetService)
	catalogHandler := handler.NewCatalogHandler(catalogService)
	settingHandler := handler.NewSettingHandler(settingService)
	previewHandler := handler.NewPreviewHandler(previewService)
	logHandler := handler.NewLogHandler(logManager, db)

	router.GET("/api/health", func(c *gin.Context) {
		response.Success(c, gin.H{"status": "ok", "time": time.Now().UTC()})
	})

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
		}

		settings := api.Group("/settings", middleware.JWTAuth(cfg, db), middleware.AdminSetupComplete(), middleware.AdminOnly())
		{
			settings.GET("/scraping", settingHandler.GetScraping)
			settings.PUT("/scraping", settingHandler.SaveScraping)
			settings.POST("/scraping/test-tmdb", settingHandler.TestTMDB)
		}

		admin := api.Group("/admin", middleware.JWTAuth(cfg, db), middleware.AdminSetupComplete(), middleware.AdminOnly())
		{
			admin.GET("/logs", logHandler.API)
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
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
