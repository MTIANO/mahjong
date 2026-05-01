package service

import (
	"context"

	"github.com/mtiano/server/pkg/mahjong"
)

type StubVisionService struct{}

func NewStubVisionService() *StubVisionService {
	return &StubVisionService{}
}

func (s *StubVisionService) RecognizeTiles(ctx context.Context, image []byte) ([]mahjong.Tile, error) {
	// Return a fixed winning hand for development/testing
	tiles, _ := mahjong.ParseTiles("123m456p789s11z")
	extra, _ := mahjong.ParseTiles("123z")
	tiles = append(tiles, extra...)
	return tiles, nil
}
