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

// ThemeScout 用一个语义/政策语感较强的模型(默认 Qwen-Max)拉取"今日 A 股热点板块",
// 用于在 StockAnalyzer 打分时作为参考信号注入 user prompt。
//
// 设计上故意保持小职责:只输出一份字符串列表,失败时返回空列表 + log 不阻塞主流程。
type ThemeScout struct {
	apiKey   string
	endpoint string
	model    string
}

func NewThemeScout(apiKey, endpoint, model string) *ThemeScout {
	return &ThemeScout{apiKey: apiKey, endpoint: endpoint, model: model}
}

const themeSystemPrompt = `你是一位 A 股资深行业研究员。任务:基于你掌握的近期(本月内)宏观/政策/产业链信息,列出今日 A 股市场最受资金关注的热点板块或细分概念,按热度从高到低。

要求:
- 只输出 5-8 个,避免泛泛的"消费"、"金融"这种大类
- 优先具体题材,例如:AI 算力、固态电池、华为产业链、电力体制改革、低空经济、商业航天、可控核聚变、稀土永磁 等
- 严格只输出 JSON 对象,不要任何解释或前后文本
- 无法判断时输出空列表`

const themeUserPrompt = `请按当前 A 股市场情况,输出今日热点板块 JSON:
{
  "themes": ["板块1", "板块2", ...]
}`

type themeResponse struct {
	Themes []string `json:"themes"`
}

var themeJSONBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

// FetchThemes 调用 Qwen 拿今日热点列表。失败 / 拒答 / 超时一律返回 nil(不阻塞)。
func (t *ThemeScout) FetchThemes(ctx context.Context) []string {
	if t == nil || t.apiKey == "" || t.endpoint == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	reqBody := chatRequest{
		Model: t.model,
		Messages: []chatMessage{
			{Role: "system", Content: themeSystemPrompt},
			{Role: "user", Content: themeUserPrompt},
		},
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[ThemeScout] marshal: %v", err)
		return nil
	}

	url := strings.TrimRight(t.endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("[ThemeScout] create request: %v", err)
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[ThemeScout] send: %v", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[ThemeScout] read: %v", err)
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("[ThemeScout] HTTP %d: %s", resp.StatusCode, string(body))
		return nil
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		log.Printf("[ThemeScout] unmarshal envelope: %v", err)
		return nil
	}
	if len(chatResp.Choices) == 0 {
		log.Printf("[ThemeScout] no choices")
		return nil
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	if matches := themeJSONBlockRe.FindStringSubmatch(content); len(matches) > 1 {
		content = matches[1]
	}

	var parsed themeResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		log.Printf("[ThemeScout] parse %q: %v", content, err)
		return nil
	}

	cleaned := make([]string, 0, len(parsed.Themes))
	for _, th := range parsed.Themes {
		th = strings.TrimSpace(th)
		if th != "" {
			cleaned = append(cleaned, th)
		}
	}
	log.Printf("[ThemeScout] %d themes: %s", len(cleaned), strings.Join(cleaned, " / "))
	return cleaned
}

// FormatThemesForPrompt 把题材列表格式化成 prompt 注入用的一行文本;空列表返回空串。
func FormatThemesForPrompt(themes []string) string {
	if len(themes) == 0 {
		return ""
	}
	return fmt.Sprintf("【今日参考热点(从高到低)】%s", strings.Join(themes, " / "))
}
