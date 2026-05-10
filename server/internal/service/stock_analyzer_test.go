package service

import (
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
