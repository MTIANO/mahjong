package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/config"
	"github.com/mtiano/server/internal/handler"
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

	r := gin.Default()

	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	recognizeHandler := handler.NewRecognizeHandler(vision)
	r.POST("/api/v1/recognize", recognizeHandler.Handle)
	r.POST("/api/v1/calculate", handler.HandleCalculate)

	if err := r.Run(cfg.Server.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
