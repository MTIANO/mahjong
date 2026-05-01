package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/service"
	"github.com/mtiano/server/pkg/mahjong"
)

type RecognizeHandler struct {
	vision service.VisionService
}

type RecognizeResponse struct {
	Tiles    []string            `json:"tiles"`
	Yaku     []YakuResponse      `json:"yaku"`
	TotalHan int                 `json:"total_han"`
	Score    mahjong.ScoreResult `json:"score"`
}

type YakuResponse struct {
	Name string `json:"name"`
	Han  int    `json:"han"`
}

func NewRecognizeHandler(vision service.VisionService) *RecognizeHandler {
	return &RecognizeHandler{vision: vision}
}

func (h *RecognizeHandler) Handle(c *gin.Context) {
	file, _, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file required"})
		return
	}
	defer file.Close()

	imageData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read image"})
		return
	}

	tiles, err := h.vision.RecognizeTiles(c.Request.Context(), imageData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "recognition failed"})
		return
	}

	hand := mahjong.NewHand(tiles, nil, mahjong.Wind_East, mahjong.Wind_East, true, false)
	yakuResults := mahjong.Judge(hand)

	totalHan := 0
	var yakuResp []YakuResponse
	for _, y := range yakuResults {
		if y.Han > 0 {
			totalHan += y.Han
			yakuResp = append(yakuResp, YakuResponse{Name: y.Name, Han: y.Han})
		}
	}

	score := mahjong.CalculateScore(totalHan, 30, false, true)

	tileStrs := make([]string, len(tiles))
	for i, t := range tiles {
		tileStrs[i] = t.String()
	}

	c.JSON(http.StatusOK, RecognizeResponse{
		Tiles:    tileStrs,
		Yaku:     yakuResp,
		TotalHan: totalHan,
		Score:    score,
	})
}
