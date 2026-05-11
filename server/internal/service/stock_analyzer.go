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
	BuyScore    int    `json:"buy_score"`
	BuyReason   string `json:"buy_reason"`
	TailScore   int    `json:"tail_score"`
	TailReason  string `json:"tail_reason"`
	KeySignals  string `json:"key_signals"`
	RiskLevel   int    `json:"risk_level"`
	TrapWarning string `json:"trap_warning"`
}

type StockAnalyzer struct {
	apiKey   string
	endpoint string
	model    string
}

func NewStockAnalyzer(apiKey, endpoint, model string) *StockAnalyzer {
	return &StockAnalyzer{apiKey: apiKey, endpoint: endpoint, model: model}
}

const systemPrompt = `你是一位专注A股尾盘短线交易的资深分析师。你的核心策略是"尾盘买入、次日早盘卖出"，持股周期约12小时，利用T+1制度下尾盘主力最终表态的信息优势。

## 三层递进式选股框架

### 第一层：盘口特征初筛
- 涨幅3%-5%最佳（有上攻动力但追高风险可控），放宽至7%也可接受
- 量比≥1.2（资金关注度提升），2以内较安全
- 换手率5%-10%（流动性好且主力控盘适度）
- 流通市值50亿-200亿（最适合短线运作的体量）
- 市盈率合理（排除负值和>200的异常值）

### 第二层：题材验证
- 个股是否属于当日热点板块（AI算力、新能源、消费电子等热门概念）
- 有题材催化的股票次日延续强势概率远高于无题材个股

### 第三层：量价信号
- 量价配合健康：放量上攻（非缩量拉升）
- 换手率异动暗示资金关注

## 评分标准（宁缺毋滥）
- 8-10分：三层信号共振，强烈推荐（涨幅3-5%+量比>1.2+换手5-10%+流通市值50-200亿+题材热点）
- 6-7分：满足盘口条件+有一定题材支撑
- 4-5分：仅满足部分条件或信号矛盾
- 1-3分：存在明显陷阱或风险信号

## 三大陷阱识别（必须检查）
1. **秒拉诱多**：涨幅集中在尾盘最后几分钟，全天走势平淡 → 大概率做收盘价
2. **缩量拉升**：价涨量缩，量比<1，无增量资金进场 → 假拉升真诱多
3. **弱转强陷阱**：全天运行在均价线下方，仅尾盘勉强翻红 → 弱势股反抽

## 风险等级（1-5）
- 1：低风险（多信号共振，量价健康）
- 2：较低风险（主要信号满足，个别瑕疵）
- 3：中等风险（信号矛盾，需谨慎）
- 4：较高风险（多个风险信号）
- 5：高风险（明显陷阱特征）`

func buildAnalysisPrompt(stock StockInfo) string {
	return fmt.Sprintf(`分析以下A股的尾盘短线交易价值（尾盘买入，次日早盘卖出）：

【盘口数据】
代码: %s | 名称: %s
当前价: %.2f | 涨跌幅: %.2f%%
开盘价: %.2f | 昨收价: %.2f
最高价: %.2f | 最低价: %.2f | 振幅: %.2f%%
成交量: %d手 | 换手率: %.2f%% | 量比: %.2f
市盈率: %.2f | 总市值: %.2f亿 | 流通市值: %.2f亿

【逐层分析要求】
1. 盘口初筛：涨幅是否在3%%-7%%？量比是否≥1.2？换手率是否5%%-10%%？流通市值是否50-200亿？
2. 题材判断：根据股票名称和行业推断是否属于当前热门题材
3. 量价信号：量比和换手率是否暗示资金异动？是放量还是缩量？
4. 陷阱检测：是否存在秒拉诱多/缩量拉升/弱转强陷阱特征？

请返回严格的 JSON（不要包含其他文字）：
{
  "buy_score": <1-10整数>,
  "buy_reason": "<40字以内的即时买入理由>",
  "tail_score": <1-10整数>,
  "tail_reason": "<40字以内的尾盘买入理由>",
  "key_signals": "<30字以内的关键信号，如：放量突破+题材热点>",
  "risk_level": <1-5整数>,
  "trap_warning": "<如有陷阱风险写30字以内警告，无则写空字符串>"
}`, stock.Code, stock.Name, stock.Price, stock.ChangePct,
		stock.Open, stock.PrevClose, stock.High, stock.Low, stock.Amplitude,
		stock.Volume, stock.TurnoverRate, stock.VolumeRatio,
		stock.PERatio, stock.MarketCap, stock.FloatMarketCap)
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
