package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type StockInfo struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Price        float64 `json:"price"`
	ChangePct    float64 `json:"change_pct"`
	Volume       int64   `json:"volume"`
	TurnoverRate float64 `json:"turnover_rate"`
	PERatio      float64 `json:"pe_ratio"`
	MarketCap    float64 `json:"market_cap"`
}

type StockDataService struct {
	endpoint string
}

func NewStockDataService(endpoint string) *StockDataService {
	return &StockDataService{endpoint: strings.TrimRight(endpoint, "/")}
}

type stockListResponse struct {
	Stocks []StockInfo `json:"stocks"`
}

func (s *StockDataService) GetHotStocks(count int) ([]StockInfo, error) {
	url := fmt.Sprintf("%s/api/stock/hot?count=%d", s.endpoint, count)
	return s.fetchStocks(url)
}

func (s *StockDataService) GetStockDetails(codes []string) ([]StockInfo, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	url := fmt.Sprintf("%s/api/stock/detail?codes=%s", s.endpoint, strings.Join(codes, ","))
	return s.fetchStocks(url)
}

func (s *StockDataService) fetchStocks(url string) ([]StockInfo, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request akshare: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("akshare returned %d: %s", resp.StatusCode, string(body))
	}

	var result stockListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return result.Stocks, nil
}
