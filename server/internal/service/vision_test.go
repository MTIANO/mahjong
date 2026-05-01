package service

import (
	"context"
	"testing"
)

func TestStubVisionService(t *testing.T) {
	svc := NewStubVisionService()
	tiles, err := svc.RecognizeTiles(context.Background(), []byte("fake image"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tiles) != 14 {
		t.Errorf("expected 14 tiles from stub, got %d", len(tiles))
	}
}
