package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/pkg/mahjong"
)

type CalculateRequest struct {
	Tiles     string `json:"tiles" binding:"required"`
	Dora      string `json:"dora"`
	IsTsumo   bool   `json:"is_tsumo"`
	SeatWind  int    `json:"seat_wind"`
	RoundWind int    `json:"round_wind"`
}

// countDora 统计手牌中宝牌指示牌对应的实际宝牌数量
func countDora(handTiles []mahjong.Tile, doraIndicators []mahjong.Tile) int {
	count := 0
	for _, indicator := range doraIndicators {
		dora := nextTile(indicator)
		for _, t := range handTiles {
			if t == dora {
				count++
			}
		}
	}
	return count
}

// nextTile 返回宝牌指示牌的下一张（实际宝牌）
func nextTile(t mahjong.Tile) mahjong.Tile {
	if t.Suit == mahjong.SuitHonor {
		if t.Number <= 4 {
			// 風牌: 東→南→西→北→東
			return mahjong.NewTile(mahjong.SuitHonor, t.Number%4+1)
		}
		// 三元牌: 白→發→中→白 (5→6→7→5)
		return mahjong.NewTile(mahjong.SuitHonor, (t.Number-5+1)%3+5)
	}
	// 数牌: 9→1
	return mahjong.NewTile(t.Suit, t.Number%9+1)
}

func HandleCalculate(c *gin.Context) {
	var req CalculateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	tiles, err := mahjong.ParseTiles(req.Tiles)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "牌型格式错误: " + err.Error()})
		return
	}

	if len(tiles) < 13 || len(tiles) > 14 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "手牌数量应为13或14张"})
		return
	}

	hand := mahjong.NewHand(
		tiles,
		nil,
		mahjong.Wind(req.SeatWind),
		mahjong.Wind(req.RoundWind),
		req.IsTsumo,
		false,
	)

	yakuResults := mahjong.Judge(hand)

	totalHan := 0
	var yakuResp []YakuResponse
	for _, y := range yakuResults {
		if y.Han > 0 {
			totalHan += y.Han
			yakuResp = append(yakuResp, YakuResponse{Name: y.Name, Han: y.Han})
		}
	}

	// 计算宝牌加番
	if req.Dora != "" {
		doraTiles, err := mahjong.ParseTiles(req.Dora)
		if err == nil {
			doraCount := countDora(tiles, doraTiles)
			if doraCount > 0 {
				totalHan += doraCount
				yakuResp = append(yakuResp, YakuResponse{Name: "宝牌", Han: doraCount})
			}
		}
	}

	score := mahjong.CalculateScore(totalHan, 30, req.SeatWind == 0, req.IsTsumo)

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
