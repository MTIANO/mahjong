package mahjong

import (
	"testing"
)

func TestNewTile(t *testing.T) {
	tile := NewTile(SuitMan, 1)
	if tile.Suit != SuitMan || tile.Number != 1 {
		t.Errorf("expected 1m, got %v", tile)
	}
}

func TestTileString(t *testing.T) {
	tests := []struct {
		tile Tile
		want string
	}{
		{NewTile(SuitMan, 1), "1m"},
		{NewTile(SuitPin, 5), "5p"},
		{NewTile(SuitSou, 9), "9s"},
		{NewTile(SuitHonor, 1), "1z"},
	}
	for _, tt := range tests {
		if got := tt.tile.String(); got != tt.want {
			t.Errorf("Tile.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestParseTiles(t *testing.T) {
	tiles, err := ParseTiles("123m456p789s11z")
	if err != nil {
		t.Fatalf("ParseTiles error: %v", err)
	}
	if len(tiles) != 11 {
		t.Fatalf("expected 11 tiles, got %d", len(tiles))
	}
	if tiles[0].String() != "1m" {
		t.Errorf("first tile = %q, want \"1m\"", tiles[0].String())
	}
	if tiles[9].String() != "1z" {
		t.Errorf("tile[9] = %q, want \"1z\"", tiles[9].String())
	}
}

func TestIsTerminal(t *testing.T) {
	if !NewTile(SuitMan, 1).IsTerminal() {
		t.Error("1m should be terminal")
	}
	if !NewTile(SuitMan, 9).IsTerminal() {
		t.Error("9m should be terminal")
	}
	if NewTile(SuitMan, 5).IsTerminal() {
		t.Error("5m should not be terminal")
	}
}

func TestIsHonor(t *testing.T) {
	if !NewTile(SuitHonor, 1).IsHonor() {
		t.Error("1z should be honor")
	}
	if NewTile(SuitMan, 1).IsHonor() {
		t.Error("1m should not be honor")
	}
}
