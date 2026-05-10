package model

import "time"

type StockRecommendation struct {
	ID           int64     `json:"id"`
	StockCode    string    `json:"stock_code"`
	StockName    string    `json:"stock_name"`
	Source       string    `json:"source"`
	BuyScore     int       `json:"buy_score"`
	BuyReason    string    `json:"buy_reason"`
	TailScore    int       `json:"tail_score"`
	TailReason   string    `json:"tail_reason"`
	KeySignals   string    `json:"key_signals"`
	AnalysisDate string    `json:"analysis_date"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
