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
	"time"
)

type AnalysisResult struct {
	BuyScore    int    `json:"buy_score"`
	BuyReason   string `json:"buy_reason"`
	TailScore   int    `json:"tail_score"`
	TailReason  string `json:"tail_reason"`
	KeySignals  string `json:"key_signals"`
	RiskLevel   int    `json:"risk_level"`
	TrapWarning string `json:"trap_warning"`
	IsFallback  bool   `json:"is_fallback"`
}

type StockAnalyzer struct {
	apiKey   string
	endpoint string
	model    string
}

func NewStockAnalyzer(apiKey, endpoint, model string) *StockAnalyzer {
	return &StockAnalyzer{apiKey: apiKey, endpoint: endpoint, model: model}
}

const systemPrompt = `你是一位专注 A 股尾盘短线的资深分析师。策略:尾盘买入、次日早盘卖出,持股 ~12 小时,利用 T+1 尾盘主力表态的信息优势。

## 评分刻度(1-10,按以下比例分布)

8-10 分(强烈推荐,预计占合格票 30-40%)
  盘口 + 量价两层达标 + 无明显陷阱。
  满足下列任一加分项冲 9-10:
    - 清晰题材催化(名称/行业属于当前热点板块,如 AI 算力、新能源、消费电子等)
    - 量能显著放大(量比 ≥ 1.5 或换手 ≥ 8%)
    - 当前价接近日内高点(抗跌性好)

6-7 分(可观察,30-40%)
  盘口达标但量价或题材其中一项平淡,无明显陷阱。

4-5 分(弱势,20%)
  信号矛盾或有轻度陷阱特征。

1-3 分(规避,5-10%)
  明显陷阱 / 多项硬指标不及格。

## 数据缺失处理

若 turnover_rate / volume_ratio / pe_ratio / float_market_cap 为 0 或明显异常,
视为"数据缺失",按可得字段打分,不额外扣分,并在 key_signals 末尾附 "[数据部分缺失]"。

## 三大陷阱(检测到一项即降到 1-3 分并写 trap_warning)

1. 秒拉诱多:涨幅集中在尾盘最后几分钟,全天走势平淡 → 大概率做收盘价
2. 缩量拉升:价涨量缩,量比 < 1,无增量资金进场 → 假拉升真诱多
3. 弱转强陷阱:全天运行在均价线下方,仅尾盘勉强翻红 → 弱势股反抽

## 风险等级(1-5,与评分反向)

1: 低风险(多信号共振,量价健康)
2: 较低(主要信号满足,个别瑕疵)
3: 中等(信号矛盾,需谨慎)
4: 较高(多个风险信号)
5: 高风险(明显陷阱特征)`

func buildAnalysisPrompt(stock StockInfo) string {
	return fmt.Sprintf(`分析以下 A 股的尾盘短线交易价值(尾盘买入,次日早盘卖出):

【盘口数据】
代码: %s | 名称: %s
当前价: %.2f | 涨跌幅: %.2f%%
开盘价: %.2f | 昨收价: %.2f
最高价: %.2f | 最低价: %.2f | 振幅: %.2f%%
成交量: %d手 | 换手率: %.2f%% | 量比: %.2f
市盈率: %.2f | 总市值: %.2f亿 | 流通市值: %.2f亿

请结合盘口数据、股票名称推断的题材属性、量价配合度,输出 JSON:
{
  "buy_score": <1-10整数>,
  "buy_reason": "<40字以内的即时买入理由>",
  "tail_score": <1-10整数>,
  "tail_reason": "<40字以内的尾盘买入理由>",
  "key_signals": "<30字以内的关键信号>",
  "risk_level": <1-5整数>,
  "trap_warning": "<如有陷阱风险写30字以内警告,无则写空字符串>"
}

⚠️ 只输出 JSON 对象,不要任何前后文字、不要 代码块标记、不要解释。
无法判断也必须返回有效 JSON,score 填 3,trap_warning 写明原因。`,
		stock.Code, stock.Name, stock.Price, stock.ChangePct,
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

// Analyze 调 AI 做尾盘短线打分。themes 是当次 cron 由 ThemeScout 拉到的"今日热点板块"列表,
// 作为参考信号注入 user prompt;为空则 prompt 退化为不带题材提示版本(向后兼容)。
func (a *StockAnalyzer) Analyze(ctx context.Context, stock StockInfo, themes []string) (*AnalysisResult, error) {
	// Level 1
	res, err := a.callOnce(ctx, stock, themes, "")
	if err == nil {
		return annotateLimitUp(res, stock), nil
	}
	log.Printf("[StockAnalyzer] %s(%s) L1 failed: %v, retrying", stock.Name, stock.Code, err)

	// Level 2: 仅对 parse 失败 / 非 4xx 的错误重试
	if isNonRetryable(err) {
		log.Printf("[StockAnalyzer] %s(%s) non-retryable, falling back", stock.Name, stock.Code)
	} else {
		res, err = a.callOnce(ctx, stock, themes, "\n\n⚠️ 上一次返回的不是合法 JSON,请严格只输出 JSON 对象,不要任何前后缀。")
		if err == nil {
			return annotateLimitUp(res, stock), nil
		}
		log.Printf("[StockAnalyzer] %s(%s) L2 failed: %v, falling back", stock.Name, stock.Code, err)
	}

	// Level 3
	fb := fallbackScore(stock)
	fb.IsFallback = true
	return annotateLimitUp(fb, stock), nil
}

func isNonRetryable(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "returned 4") // HTTP 4xx
}

func (a *StockAnalyzer) callOnce(ctx context.Context, stock StockInfo, themes []string, extraUserSuffix string) (*AnalysisResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	prompt := buildAnalysisPrompt(stock)
	if hint := FormatThemesForPrompt(themes); hint != "" {
		prompt += "\n\n" + hint
	}
	prompt += extraUserSuffix

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

// fallbackScore 基于 pre-filter 可用指标做规则打分,用于 AI 不可用时兜底入库。
// 每项 0-2 分,合计 0-10;为避免规则分混入"强烈推荐"区,buy/tail_score 硬 cap 到 7。
func fallbackScore(stock StockInfo) *AnalysisResult {
	total := 0

	c := stock.ChangePct
	if c >= 3 && c <= 5 {
		total += 2
	} else if (c >= 0.5 && c < 3) || (c > 5 && c <= 9.5) {
		total += 1
	}

	if stock.VolumeRatio >= 1.5 {
		total += 2
	} else if stock.VolumeRatio >= 1.0 {
		total += 1
	}

	tr := stock.TurnoverRate
	if tr >= 5 && tr <= 10 {
		total += 2
	} else if (tr >= 2 && tr < 5) || (tr > 10 && tr <= 20) {
		total += 1
	}

	f := stock.FloatMarketCap
	if f >= 50 && f <= 200 {
		total += 2
	} else if (f >= 20 && f < 50) || (f > 200 && f <= 500) {
		total += 1
	}

	a := stock.Amplitude
	if a >= 3 && a <= 8 {
		total += 2
	} else if (a >= 1 && a < 3) || (a > 8 && a <= 12) {
		total += 1
	}

	capped := total
	if capped > 7 {
		capped = 7
	}
	risk := 6 - total/2
	if risk < 1 {
		risk = 1
	}
	if risk > 5 {
		risk = 5
	}

	signals := fmt.Sprintf("涨%.2f%% 量比%.2f 换手%.2f%% 流通%.0f亿", stock.ChangePct, stock.VolumeRatio, stock.TurnoverRate, stock.FloatMarketCap)

	return &AnalysisResult{
		BuyScore:    capped,
		BuyReason:   "AI 服务异常,规则兜底评分",
		TailScore:   capped,
		TailReason:  "AI 服务异常,规则兜底评分",
		KeySignals:  signals,
		RiskLevel:   risk,
		TrapWarning: "",
	}
}
