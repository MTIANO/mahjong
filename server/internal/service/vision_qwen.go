package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/mtiano/server/pkg/mahjong"
)

type QwenVisionService struct {
	apiKey   string
	endpoint string
	model    string
}

func NewQwenVisionService(apiKey, endpoint, model string) *QwenVisionService {
	if endpoint == "" {
		endpoint = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	if model == "" {
		model = "qwen3.6-plus-2026-04-02"
	}
	return &QwenVisionService{
		apiKey:   apiKey,
		endpoint: endpoint,
		model:    model,
	}
}

type responsesRequest struct {
	Model string      `json:"model"`
	Input []inputItem `json:"input"`
}

type inputItem struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type responsesResponse struct {
	ID     string       `json:"id"`
	Output []outputItem `json:"output"`
	Error  *qwenError   `json:"error,omitempty"`
}

type outputItem struct {
	Type    string          `json:"type"`
	Content []outputContent `json:"content,omitempty"`
}

type outputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type qwenError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

const tileRecognitionPrompt = `你是一个日本麻将牌识别专家。请识别图片中的所有麻将牌，并以紧凑记法输出。

记法规则：
- 万子(m): 1m-9m
- 筒子(p): 1p-9p
- 索子(s): 1s-9s
- 字牌(z): 1z=東, 2z=南, 3z=西, 4z=北, 5z=白, 6z=發, 7z=中

输出格式要求：
- 只输出牌的紧凑记法，不要有任何其他文字说明
- 同花色的牌数字连写，如: 123m456p789s11z
- 按万子、筒子、索子、字牌的顺序排列
- 示例输出: 123m456p789s11z

请识别图片中的所有麻将牌：`

func (s *QwenVisionService) RecognizeTiles(ctx context.Context, image []byte) ([]mahjong.Tile, error) {
	log.Printf("[Qwen] 开始识别，图片大小: %d bytes", len(image))

	imageBase64 := base64.StdEncoding.EncodeToString(image)
	dataURL := "data:image/jpeg;base64," + imageBase64

	reqBody := responsesRequest{
		Model: s.model,
		Input: []inputItem{
			{
				Role: "user",
				Content: []contentPart{
					{Type: "input_text", Text: tileRecognitionPrompt},
					{Type: "input_image", ImageURL: dataURL},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(s.endpoint, "/") + "/responses"
	log.Printf("[Qwen] 请求URL: %s, 模型: %s", url, s.model)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[Qwen] 请求失败: %v", err)
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	log.Printf("[Qwen] 响应状态: %d, body长度: %d", resp.StatusCode, len(body))

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Qwen] API错误响应: %s", string(body))
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var qwenResp responsesResponse
	if err := json.Unmarshal(body, &qwenResp); err != nil {
		log.Printf("[Qwen] 解析响应失败: %v, body: %s", err, string(body))
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if qwenResp.Error != nil {
		log.Printf("[Qwen] API返回错误: [%s] %s", qwenResp.Error.Code, qwenResp.Error.Message)
		return nil, fmt.Errorf("API error [%s]: %s", qwenResp.Error.Code, qwenResp.Error.Message)
	}

	tileStr := extractTileString(qwenResp)
	log.Printf("[Qwen] 提取到牌型字符串: %q", tileStr)

	if tileStr == "" {
		log.Printf("[Qwen] 未提取到牌型，原始output: %+v", qwenResp.Output)
		return nil, fmt.Errorf("no tile result in response")
	}

	tiles, err := mahjong.ParseTiles(tileStr)
	if err != nil {
		log.Printf("[Qwen] 解析牌型失败: %v", err)
		return nil, fmt.Errorf("parse tiles %q: %w", tileStr, err)
	}

	log.Printf("[Qwen] 识别成功，共 %d 张牌: %s", len(tiles), tileStr)
	return tiles, nil
}

func extractTileString(resp responsesResponse) string {
	for _, item := range resp.Output {
		if item.Type == "message" {
			for _, content := range item.Content {
				if content.Type == "output_text" {
					return cleanTileString(content.Text)
				}
			}
		}
	}
	return ""
}

func cleanTileString(s string) string {
	s = strings.TrimSpace(s)
	var result strings.Builder
	for _, ch := range s {
		if (ch >= '1' && ch <= '9') || ch == 'm' || ch == 'p' || ch == 's' || ch == 'z' {
			result.WriteRune(ch)
		}
	}
	return result.String()
}
