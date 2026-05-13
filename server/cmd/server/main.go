package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/config"
	stockcron "github.com/mtiano/server/internal/cron"
	"github.com/mtiano/server/internal/db"
	"github.com/mtiano/server/internal/handler"
	"github.com/mtiano/server/internal/middleware"
	"github.com/mtiano/server/internal/service"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	var vision service.VisionService
	switch cfg.Vision.Provider {
	case "qwen":
		vision = service.NewQwenVisionService(cfg.Vision.APIKey, cfg.Vision.Endpoint, cfg.Vision.Model)
	case "yolo":
		vision = service.NewYoloVisionService(cfg.Vision.Endpoint)
	default:
		vision = service.NewStubVisionService()
	}

	mysqlDB, err := db.InitMySQL(cfg.MySQL.DSN)
	if err != nil {
		log.Fatalf("failed to init mysql: %v", err)
	}
	defer mysqlDB.Close()

	store := service.NewStockStore(mysqlDB)
	authService := service.NewAuthService(cfg.WeChat.AppID, cfg.WeChat.Secret, cfg.JWT.Secret, cfg.JWT.ExpireHours, store)
	stockDataService := service.NewStockDataService(cfg.Stock.AKShareEndpoint)
	stockAnalyzer := service.NewStockAnalyzer(cfg.Stock.ScorerAPIKey, cfg.Stock.ScorerEndpoint, cfg.Stock.ScorerModel)
	themeScout := service.NewThemeScout(cfg.Stock.ThemeAPIKey, cfg.Stock.ThemeEndpoint, cfg.Stock.ThemeModel)

	cronJob := stockcron.NewStockCron(store, stockDataService, stockAnalyzer, themeScout)
	cronJob.Start()
	defer cronJob.Stop()

	r := gin.Default()

	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	recognizeHandler := handler.NewRecognizeHandler(vision)
	r.POST("/api/v1/recognize", recognizeHandler.Handle)
	r.POST("/api/v1/calculate", handler.HandleCalculate)

	authHandler := handler.NewAuthHandler(authService)
	r.POST("/api/v1/auth/login", authHandler.Login)

	stockHandler := handler.NewStockHandler(store, stockDataService, stockAnalyzer)
	stockGroup := r.Group("/api/v1/stock")
	stockGroup.Use(middleware.JWTAuth(authService))
	{
		stockGroup.GET("/recommendations", stockHandler.GetRecommendations)
		stockGroup.GET("/watchlist", stockHandler.GetWatchlist)
		stockGroup.POST("/watchlist", stockHandler.AddWatchlist)
		stockGroup.DELETE("/watchlist/:code", stockHandler.RemoveWatchlist)
		stockGroup.GET("/quote/:code", stockHandler.GetQuote)
		stockGroup.GET("/kline/daily/:code", stockHandler.GetDailyKline)
		stockGroup.GET("/kline/minute/:code", stockHandler.GetMinuteKline)
	}

	if err := r.Run(cfg.Server.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
