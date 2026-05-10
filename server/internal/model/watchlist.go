package model

import "time"

type WatchlistItem struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	StockCode string    `json:"stock_code"`
	StockName string    `json:"stock_name"`
	CreatedAt time.Time `json:"created_at"`
}
