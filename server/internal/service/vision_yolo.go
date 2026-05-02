package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/mtiano/server/pkg/mahjong"
)

type YoloVisionService struct {
	endpoint    string
	client      *http.Client
	LastRedDora int
}

type yoloPredictResponse struct {
	Tiles   string `json:"tiles"`
	Count   int    `json:"count"`
	RedDora int    `json:"red_dora"`
}

func NewYoloVisionService(endpoint string) *YoloVisionService {
	return &YoloVisionService{
		endpoint: endpoint,
		client:   &http.Client{},
	}
}

func (s *YoloVisionService) RecognizeTiles(ctx context.Context, image []byte) ([]mahjong.Tile, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("image", "image.jpg")
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(image); err != nil {
		return nil, fmt.Errorf("write image: %w", err)
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", s.endpoint+"/predict", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("yolo service request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yolo service error (status %d): %s", resp.StatusCode, string(body))
	}

	var result yoloPredictResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if result.Tiles == "" {
		return nil, fmt.Errorf("no tiles detected")
	}

	s.LastRedDora = result.RedDora

	tiles, err := mahjong.ParseTiles(result.Tiles)
	if err != nil {
		return nil, fmt.Errorf("parse tiles '%s': %w", result.Tiles, err)
	}

	return tiles, nil
}
