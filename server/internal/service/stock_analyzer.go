package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

type AnalysisResult struct {
	BuyScore   int    `json:"buy_score"`
	BuyReason  string `json:"buy_reason"`
	TailScore  int    `json:"tail_score"`
	TailReason string `json:"tail_reason"`
}

type StockAnalyzer struct {
	apiKey   string
	endpoint string
	model    string
}

func NewStockAnalyzer(apiKey, endpoint, model string) *StockAnalyzer {
	return &StockAnalyzer{apiKey: apiKey, endpoint: endpoint, model: model}
}

func buildAnalysisPrompt(stock StockInfo) string {
	return fmt.Sprintf(`你是一位专业的A股分析师。请根据以下股票实时数据，分析该股票的短线交易价值，给出今日购买和今日尾盘购买的推荐度及理由。

股票数据：
- 代码: %s
- 名称: %s
- 当前价: %.2f
- 涨跌幅: %.2f%%
- 成交量: %d
- 换手率: %.2f%%
- 市盈率: %.2f
- 总市值: %.2f亿

请返回严格的 JSON 格式（不要包含任何其他文字）：
{
  "buy_score": <1-10的整数，10为最推荐>,
  "buy_reason": "<50字以内的今日购买理由>",
  "tail_score": <1-10的整数，10为最推荐>,
  "tail_reason": "<50字以内的尾盘购买理由>"
}`, stock.Code, stock.Name, stock.Price, stock.ChangePct, stock.Volume, stock.TurnoverRate, stock.PERatio, stock.MarketCap)
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *StockAnalyzer) Analyze(ctx context.Context, stock StockInfo) (*AnalysisResult, error) {
	prompt := buildAnalysisPrompt(stock)

	reqBody := chatRequest{
		Model: a.model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := strings.TrimRight(a.endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI API returned %d: %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("AI error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in AI response")
	}

	content := chatResp.Choices[0].Message.Content
	log.Printf("[StockAnalyzer] %s(%s) AI response: %s", stock.Name, stock.Code, content)

	return parseAnalysisResult(content)
}

var jsonBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

func parseAnalysisResult(raw string) (*AnalysisResult, error) {
	raw = strings.TrimSpace(raw)

	if matches := jsonBlockRe.FindStringSubmatch(raw); len(matches) > 1 {
		raw = matches[1]
	}

	var result AnalysisResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parse AI result %q: %w", raw, err)
	}
	return &result, nil
}
