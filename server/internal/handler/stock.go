package handler

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/model"
	"github.com/mtiano/server/internal/service"
)

type StockHandler struct {
	store     *service.StockStore
	stockData *service.StockDataService
	analyzer  *service.StockAnalyzer
}

func NewStockHandler(store *service.StockStore, stockData *service.StockDataService, analyzer *service.StockAnalyzer) *StockHandler {
	return &StockHandler{store: store, stockData: stockData, analyzer: analyzer}
}

func (h *StockHandler) GetRecommendations(c *gin.Context) {
	source := c.Query("source")
	recs, err := h.store.GetTodayRecommendations(source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get recommendations"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"recommendations": recs})
}

func (h *StockHandler) GetWatchlist(c *gin.Context) {
	userID := c.GetInt64("user_id")
	items, err := h.store.GetWatchlist(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get watchlist"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"watchlist": items})
}

type addWatchlistRequest struct {
	StockCode string `json:"stock_code" binding:"required"`
}

func (h *StockHandler) AddWatchlist(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req addWatchlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stock_code is required"})
		return
	}

	stocks, err := h.stockData.GetStockDetails([]string{req.StockCode})
	if err != nil || len(stocks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stock code or failed to fetch stock info"})
		return
	}

	if err := h.store.AddWatchlistItem(userID, stocks[0].Code, stocks[0].Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add watchlist item"})
		return
	}

	go h.analyzeStock(stocks[0])

	c.JSON(http.StatusOK, gin.H{"message": "ok", "stock": stocks[0]})
}

func (h *StockHandler) analyzeStock(stock service.StockInfo) {
	result, err := h.analyzer.Analyze(context.Background(), stock)
	if err != nil {
		log.Printf("[StockHandler] analyze %s(%s) failed: %v", stock.Name, stock.Code, err)
		return
	}
	rec := &model.StockRecommendation{
		StockCode:    stock.Code,
		StockName:    stock.Name,
		Source:       "watchlist",
		BuyScore:     result.BuyScore,
		BuyReason:    result.BuyReason,
		TailScore:    result.TailScore,
		TailReason:   result.TailReason,
		KeySignals:   result.KeySignals,
		RiskLevel:    result.RiskLevel,
		TrapWarning:  result.TrapWarning,
		AnalysisDate: time.Now().Format("2006-01-02"),
	}
	if err := h.store.UpsertRecommendation(rec); err != nil {
		log.Printf("[StockHandler] upsert %s failed: %v", stock.Code, err)
		return
	}
	log.Printf("[StockHandler] analyzed %s(%s): buy=%d tail=%d", stock.Name, stock.Code, result.BuyScore, result.TailScore)
}

func (h *StockHandler) GetQuote(c *gin.Context) {
	code := c.Param("code")
	quote, err := h.stockData.GetStockQuote(code)
	if err != nil {
		log.Printf("[StockHandler] GetQuote %s: %v", code, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get quote"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"quote": quote})
}

func (h *StockHandler) GetDailyKline(c *gin.Context) {
	code := c.Param("code")
	count := 60
	if q := c.Query("count"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			count = n
		}
	}
	klines, err := h.stockData.GetDailyKline(code, count)
	if err != nil {
		log.Printf("[StockHandler] GetDailyKline %s: %v", code, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get daily kline"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"klines": klines})
}

func (h *StockHandler) GetMinuteKline(c *gin.Context) {
	code := c.Param("code")
	data, err := h.stockData.GetMinuteKline(code)
	if err != nil {
		log.Printf("[StockHandler] GetMinuteKline %s: %v", code, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get minute kline"})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *StockHandler) RemoveWatchlist(c *gin.Context) {
	userID := c.GetInt64("user_id")
	code := c.Param("code")

	if err := h.store.RemoveWatchlistItem(userID, code); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove watchlist item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
