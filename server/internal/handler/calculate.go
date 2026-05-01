package handler

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/pkg/mahjong"
)

type MeldRequest struct {
	Type  string `json:"type"`
	Tiles string `json:"tiles"`
}

type CalculateRequest struct {
	Tiles     string        `json:"tiles" binding:"required"`
	Melds     []MeldRequest `json:"melds"`
	Dora      string        `json:"dora"`
	IsTsumo   bool          `json:"is_tsumo"`
	IsParent  bool          `json:"is_parent"`
	SeatWind  int           `json:"seat_wind"`
	RoundWind int           `json:"round_wind"`
}

func parseMelds(reqs []MeldRequest) ([]mahjong.Meld, error) {
	var melds []mahjong.Meld
	for _, r := range reqs {
		tiles, err := mahjong.ParseTiles(r.Tiles)
		if err != nil {
			return nil, fmt.Errorf("副露牌解析失败: %v", err)
		}
		var meldType mahjong.MeldType
		switch r.Type {
		case "chi":
			if len(tiles) != 3 {
				return nil, fmt.Errorf("吃需要3张牌")
			}
			meldType = mahjong.MeldChi
		case "pon":
			if len(tiles) != 3 {
				return nil, fmt.Errorf("碰需要3张牌")
			}
			meldType = mahjong.MeldPon
		case "open_kan":
			if len(tiles) != 4 {
				return nil, fmt.Errorf("明杠需要4张牌")
			}
			meldType = mahjong.MeldOpenKan
		case "closed_kan":
			if len(tiles) != 4 {
				return nil, fmt.Errorf("暗杠需要4张牌")
			}
			meldType = mahjong.MeldClosedKan
		default:
			return nil, fmt.Errorf("未知副露类型: %s", r.Type)
		}
		melds = append(melds, mahjong.Meld{Type: meldType, Tiles: tiles})
	}
	return melds, nil
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
		log.Printf("[Calculate] 参数绑定失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	log.Printf("[Calculate] 收到请求: tiles=%s, melds=%d个, dora=%s, tsumo=%v, seat=%d, round=%d",
		req.Tiles, len(req.Melds), req.Dora, req.IsTsumo, req.SeatWind, req.RoundWind)

	tiles, err := mahjong.ParseTiles(req.Tiles)
	if err != nil {
		log.Printf("[Calculate] 牌型解析失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "牌型格式错误: " + err.Error()})
		return
	}

	melds, err := parseMelds(req.Melds)
	if err != nil {
		log.Printf("[Calculate] 副露解析失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[Calculate] 解析手牌: %d 张, 副露: %d 组, %v", len(tiles), len(melds), tiles)

	hand := mahjong.NewHand(
		tiles,
		melds,
		mahjong.Wind(req.SeatWind),
		mahjong.Wind(req.RoundWind),
		req.IsTsumo,
		false,
	)

	yakuResults := mahjong.Judge(hand)
	log.Printf("[Calculate] 判定结果: %+v", yakuResults)

	yakuHan := 0
	var yakuResp []YakuResponse
	for _, y := range yakuResults {
		if y.Han > 0 {
			yakuHan += y.Han
			yakuResp = append(yakuResp, YakuResponse{Name: y.Name, Han: y.Han})
		}
	}

	// 无役判定：必须有至少一个役才能和牌（宝牌不算役）
	if yakuHan == 0 {
		log.Printf("[Calculate] 无役，不能和牌")
		tileStrs := make([]string, len(tiles))
		for i, t := range tiles {
			tileStrs[i] = t.String()
		}
		c.JSON(http.StatusOK, RecognizeResponse{
			Tiles:    tileStrs,
			Yaku:     yakuResp,
			TotalHan: 0,
			Score:    mahjong.ScoreResult{},
		})
		return
	}

	totalHan := yakuHan

	// 计算宝牌加番（包含副露中的牌）
	if req.Dora != "" {
		doraTiles, err := mahjong.ParseTiles(req.Dora)
		if err == nil {
			allTiles := hand.AllTiles()
			doraCount := countDora(allTiles, doraTiles)
			if doraCount > 0 {
				totalHan += doraCount
				yakuResp = append(yakuResp, YakuResponse{Name: "宝牌", Han: doraCount})
			}
			log.Printf("[Calculate] 宝牌指示: %v, 命中: %d", doraTiles, doraCount)
		}
	}

	score := mahjong.CalculateScore(totalHan, 30, req.IsParent, req.IsTsumo)
	log.Printf("[Calculate] 最终: %d番, 庄家=%v, 得分=%+v", totalHan, req.IsParent, score)

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
