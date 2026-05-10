package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/service"
)

type StockHandler struct {
	store     *service.StockStore
	stockData *service.StockDataService
}

func NewStockHandler(store *service.StockStore, stockData *service.StockDataService) *StockHandler {
	return &StockHandler{store: store, stockData: stockData}
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

	c.JSON(http.StatusOK, gin.H{"message": "ok", "stock": stocks[0]})
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
