package mahjong

import "testing"

func TestNewHand(t *testing.T) {
	tiles, _ := ParseTiles("1234m456p789s1122z")
	hand := NewHand(tiles, nil, Wind_East, Wind_East, false, false)
	if len(hand.ClosedTiles) != 14 {
		t.Errorf("expected 14 closed tiles, got %d", len(hand.ClosedTiles))
	}
	if hand.SeatWind != Wind_East {
		t.Errorf("expected seat wind East")
	}
}

func TestHandWithMeld(t *testing.T) {
	closed, _ := ParseTiles("456p789s1122z")
	meld := Meld{
		Type:  MeldChi,
		Tiles: []Tile{NewTile(SuitMan, 1), NewTile(SuitMan, 2), NewTile(SuitMan, 3)},
	}
	hand := NewHand(closed, []Meld{meld}, Wind_East, Wind_East, false, false)
	if len(hand.Melds) != 1 {
		t.Errorf("expected 1 meld, got %d", len(hand.Melds))
	}
	if hand.IsClosed() {
		t.Error("hand with chi should not be closed")
	}
}

func TestHandIsClosed(t *testing.T) {
	tiles, _ := ParseTiles("1234m456p789s1122z")
	hand := NewHand(tiles, nil, Wind_East, Wind_East, false, false)
	if !hand.IsClosed() {
		t.Error("hand with no melds should be closed")
	}
}
