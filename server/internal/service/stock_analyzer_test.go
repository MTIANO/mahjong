package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseAnalysisResult_Valid(t *testing.T) {
	raw := `{"buy_score": 7, "buy_reason": "MACD金叉", "tail_score": 5, "tail_reason": "尾盘不确定"}`
	result, err := parseAnalysisResult(raw)
	if err != nil {
		t.Fatalf("parseAnalysisResult: %v", err)
	}
	if result.BuyScore != 7 {
		t.Errorf("expected buy_score 7, got %d", result.BuyScore)
	}
	if result.BuyReason != "MACD金叉" {
		t.Errorf("expected buy_reason 'MACD金叉', got %q", result.BuyReason)
	}
	if result.TailScore != 5 {
		t.Errorf("expected tail_score 5, got %d", result.TailScore)
	}
}

func TestParseAnalysisResult_WithMarkdownWrapper(t *testing.T) {
	raw := "```json\n{\"buy_score\": 8, \"buy_reason\": \"放量突破\", \"tail_score\": 6, \"tail_reason\": \"资金流入\"}\n```"
	result, err := parseAnalysisResult(raw)
	if err != nil {
		t.Fatalf("parseAnalysisResult: %v", err)
	}
	if result.BuyScore != 8 {
		t.Errorf("expected 8, got %d", result.BuyScore)
	}
}

func TestParseAnalysisResult_Invalid(t *testing.T) {
	_, err := parseAnalysisResult("not json at all")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBuildAnalysisPrompt(t *testing.T) {
	stock := StockInfo{
		Code: "600519", Name: "贵州茅台", Price: 1680.5,
		ChangePct: 2.35, Volume: 12345678, TurnoverRate: 0.85,
		PERatio: 28.5, MarketCap: 21100.0,
	}
	prompt := buildAnalysisPrompt(stock)
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
}

func Test_fallbackScore_bucketing(t *testing.T) {
	// 5 项指标全部命中 2 分区间 → 合计 10 → buy_score cap 至 7
	strong := StockInfo{
		Code: "000001", Name: "平安银行",
		ChangePct: 4.0, VolumeRatio: 1.8, TurnoverRate: 7.5,
		FloatMarketCap: 100, Amplitude: 5.0,
	}
	res := fallbackScore(strong)
	if res.BuyScore != 7 {
		t.Errorf("strong: buy_score want 7 (capped), got %d", res.BuyScore)
	}
	if res.TailScore != 7 {
		t.Errorf("strong: tail_score want 7, got %d", res.TailScore)
	}
	if res.RiskLevel != 1 {
		t.Errorf("strong: risk want 1, got %d", res.RiskLevel)
	}
	if res.BuyReason != "AI 服务异常,规则兜底评分" {
		t.Errorf("unexpected buy_reason: %q", res.BuyReason)
	}

	// 全 0 字段 → 合计 0 → buy_score 0 (最低档) → risk clamp 到 5
	empty := StockInfo{Code: "000002", Name: "test"}
	res = fallbackScore(empty)
	if res.BuyScore != 0 {
		t.Errorf("empty: buy_score want 0, got %d", res.BuyScore)
	}
	if res.RiskLevel != 5 {
		t.Errorf("empty: risk want 5, got %d", res.RiskLevel)
	}

	// 中等:涨幅 2%(1分)、量比 1.2(1分)、换手 3%(1分)、流通 40 亿(1分)、振幅 2%(1分) = 5
	mid := StockInfo{
		Code: "600000", Name: "浦发银行",
		ChangePct: 2.0, VolumeRatio: 1.2, TurnoverRate: 3.0,
		FloatMarketCap: 40, Amplitude: 2.0,
	}
	res = fallbackScore(mid)
	if res.BuyScore != 5 {
		t.Errorf("mid: buy_score want 5, got %d", res.BuyScore)
	}
	if res.RiskLevel != 4 {
		t.Errorf("mid: risk want 4 (total=5, 6 - 5/2(int) = 4), got %d", res.RiskLevel)
	}
}

func Test_Analyze_retry_on_non_json(t *testing.T) {
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body string
		if calls == 1 {
			body = `{"choices":[{"message":{"content":"抱歉,我无法判断。"}}]}`
		} else {
			body = `{"choices":[{"message":{"content":"{\"buy_score\":8,\"buy_reason\":\"放量突破\",\"tail_score\":7,\"tail_reason\":\"尾盘可入\",\"key_signals\":\"量价齐升\",\"risk_level\":2,\"trap_warning\":\"\"}"}}]}`
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer ts.Close()

	a := NewStockAnalyzer("test-key", ts.URL, "test-model")
	res, err := a.Analyze(context.Background(), StockInfo{Code: "600519", Name: "贵州茅台", ChangePct: 3.0})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if res.BuyScore != 8 {
		t.Errorf("want buy_score 8 from retry, got %d", res.BuyScore)
	}
	if calls != 2 {
		t.Errorf("want 2 calls (original + retry), got %d", calls)
	}
	if res.IsFallback {
		t.Error("retry success should not be marked as fallback")
	}
}

func Test_Analyze_fallback_on_double_failure(t *testing.T) {
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"两次都不是 JSON"}}]}`))
	}))
	defer ts.Close()

	a := NewStockAnalyzer("test-key", ts.URL, "test-model")
	res, err := a.Analyze(context.Background(), StockInfo{
		Code: "000001", Name: "平安银行",
		ChangePct: 4.0, VolumeRatio: 1.8, TurnoverRate: 7.5,
		FloatMarketCap: 100, Amplitude: 5.0,
	})
	if err != nil {
		t.Fatalf("Analyze should fall back, got err: %v", err)
	}
	if !res.IsFallback {
		t.Error("expected IsFallback=true after double failure")
	}
	if res.BuyScore != 7 {
		t.Errorf("expected cap-7 fallback, got %d", res.BuyScore)
	}
	if calls != 2 {
		t.Errorf("expected exactly 2 AI calls before fallback, got %d", calls)
	}
}

func Test_Analyze_no_retry_on_4xx(t *testing.T) {
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer ts.Close()

	a := NewStockAnalyzer("bad-key", ts.URL, "test-model")
	res, err := a.Analyze(context.Background(), StockInfo{
		Code: "000001", Name: "test",
		ChangePct: 4.0, VolumeRatio: 1.8, TurnoverRate: 7.5,
		FloatMarketCap: 100, Amplitude: 5.0,
	})
	if err != nil {
		t.Fatalf("Analyze should fall back without error, got: %v", err)
	}
	if !res.IsFallback {
		t.Error("expected IsFallback=true on 4xx")
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 call (no retry on 4xx), got %d", calls)
	}
	if res.BuyScore != 7 {
		t.Errorf("expected cap-7 fallback, got %d", res.BuyScore)
	}
}
