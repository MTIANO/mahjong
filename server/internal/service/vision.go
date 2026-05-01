package service

import (
	"context"

	"github.com/mtiano/server/pkg/mahjong"
)

type VisionService interface {
	RecognizeTiles(ctx context.Context, image []byte) ([]mahjong.Tile, error)
}
