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
	KeySignals string `json:"key_signals"`
}

type StockAnalyzer struct {
	apiKey   string
	endpoint string
	model    string
}

func NewStockAnalyzer(apiKey, endpoint, model string) *StockAnalyzer {
	return &StockAnalyzer{apiKey: apiKey, endpoint: endpoint, model: model}
}

const systemPrompt = `你是一位经验丰富的A股短线交易分析师，擅长从量价关系、资金流向和技术形态中发现短线机会。你的分析风格偏积极但理性——你的目标是帮助交易者发现尽可能多的有价值的交易机会，而不是过度保守导致错失良机。

评分标准：
- 8-10分：强烈推荐，多个积极信号共振（放量突破、资金流入、热门题材等）
- 6-7分：值得关注，有明确的交易逻辑支撑
- 4-5分：中性，没有明显的买入或卖出信号
- 1-3分：建议回避，存在明显的风险信号

你倾向于给出较高分数的情况：
- 换手率适中（3%-8%），说明交易活跃但不过热
- 涨跌幅在合理范围内（-3%到+5%），尚有上涨空间
- 市盈率合理或处于行业低估区间
- 成交量相比平时放大，资金关注度提升

你倾向于给出较低分数的情况：
- 换手率过高（>15%）或过低（<0.5%）
- 已大幅拉升（涨幅>7%），追高风险大
- 跌幅过大（<-5%），可能处于下跌趋势
- 市盈率为负或极高（>200），基本面存疑`

func buildAnalysisPrompt(stock StockInfo) string {
	return fmt.Sprintf(`请分析以下A股的短线交易价值：

【实时数据】
代码: %s | 名称: %s
当前价: %.2f | 涨跌幅: %.2f%%
成交量: %d手 | 换手率: %.2f%%
市盈率: %.2f | 总市值: %.2f亿

【分析要求】
1. 结合量价关系判断资金动向
2. 分析当前涨跌幅位置的追涨/抄底价值
3. 评估换手率是否暗示异动
4. 综合给出今日即时买入和尾盘买入的推荐度

请返回严格的 JSON（不要包含其他文字）：
{
  "buy_score": <1-10整数>,
  "buy_reason": "<40字以内的即时买入理由>",
  "tail_score": <1-10整数>,
  "tail_reason": "<40字以内的尾盘买入理由>",
  "key_signals": "<30字以内的关键技术信号，如：放量上攻、缩量回调等>"
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
			{Role: "system", Content: systemPrompt},
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
