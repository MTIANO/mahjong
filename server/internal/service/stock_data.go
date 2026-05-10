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

type StockQuote struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Price          float64 `json:"price"`
	Change         float64 `json:"change"`
	ChangePct      float64 `json:"change_pct"`
	Open           float64 `json:"open"`
	PrevClose      float64 `json:"prev_close"`
	High           float64 `json:"high"`
	Low            float64 `json:"low"`
	Volume         int64   `json:"volume"`
	Amount         float64 `json:"amount"`
	TurnoverRate   float64 `json:"turnover_rate"`
	PERatio        float64 `json:"pe_ratio"`
	PBRatio        float64 `json:"pb_ratio"`
	MarketCap      float64 `json:"market_cap"`
	FloatMarketCap float64 `json:"float_market_cap"`
	Amplitude      float64 `json:"amplitude"`
	VolumeRatio    float64 `json:"volume_ratio"`
}

type DailyKline struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Volume float64 `json:"volume"`
}

type MinuteData struct {
	Time     string  `json:"time"`
	Price    float64 `json:"price"`
	Volume   int64   `json:"volume"`
	AvgPrice float64 `json:"avg_price"`
}

type MinuteKline struct {
	PrevClose float64      `json:"prev_close"`
	Data      []MinuteData `json:"data"`
}

func (s *StockDataService) GetStockQuote(code string) (*StockQuote, error) {
	url := fmt.Sprintf("%s/api/stock/quote?code=%s", s.endpoint, code)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request quote: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("quote returned %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		Quote StockQuote `json:"quote"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &result.Quote, nil
}

func (s *StockDataService) GetDailyKline(code string, count int) ([]DailyKline, error) {
	url := fmt.Sprintf("%s/api/stock/kline/daily?code=%s&count=%d", s.endpoint, code, count)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request daily kline: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daily kline returned %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		Klines []DailyKline `json:"klines"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return result.Klines, nil
}

func (s *StockDataService) GetMinuteKline(code string) (*MinuteKline, error) {
	url := fmt.Sprintf("%s/api/stock/kline/minute?code=%s", s.endpoint, code)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request minute kline: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("minute kline returned %d: %s", resp.StatusCode, string(body))
	}
	var result MinuteKline
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &result, nil
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
