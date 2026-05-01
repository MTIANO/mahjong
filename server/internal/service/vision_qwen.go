package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
		model = "qwen-vl-max"
	}
	return &QwenVisionService{
		apiKey:   apiKey,
		endpoint: endpoint,
		model:    model,
	}
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type chatContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Error   *qwenError   `json:"error,omitempty"`
}

type chatChoice struct {
	Message chatRespMessage `json:"message"`
}

type chatRespMessage struct {
	Content string `json:"content"`
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
	imageBase64 := base64.StdEncoding.EncodeToString(image)
	dataURL := "data:image/jpeg;base64," + imageBase64

	reqBody := chatRequest{
		Model: s.model,
		Messages: []chatMessage{
			{
				Role: "user",
				Content: []chatContentPart{
					{Type: "text", Text: tileRecognitionPrompt},
					{Type: "image_url", ImageURL: &imageURL{URL: dataURL}},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(s.endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

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
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("API error [%s]: %s", chatResp.Error.Code, chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	tileStr := cleanTileString(chatResp.Choices[0].Message.Content)
	if tileStr == "" {
		return nil, fmt.Errorf("no tile result in response, raw: %s", chatResp.Choices[0].Message.Content)
	}

	tiles, err := mahjong.ParseTiles(tileStr)
	if err != nil {
		return nil, fmt.Errorf("parse tiles %q: %w", tileStr, err)
	}

	return tiles, nil
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
