package mahjong

import "testing"

func TestDecompose_BasicWin(t *testing.T) {
	// 123m 456m 789m 123p 11s — standard 4 mentsu + 1 jantai
	tiles, _ := ParseTiles("123456789m123p11s")
	results := Decompose(tiles)
	if len(results) == 0 {
		t.Fatal("expected at least one decomposition")
	}
	found := false
	for _, r := range results {
		if r.Pair.String() == "1s" && len(r.Sequences) == 4 {
			found = true
		}
	}
	if !found {
		t.Error("expected decomposition with pair=1s and 4 sequences")
	}
}

func TestDecompose_NoWin(t *testing.T) {
	// Invalid hand — no valid decomposition
	tiles, _ := ParseTiles("12345m12345p123s")
	results := Decompose(tiles)
	if len(results) != 0 {
		t.Errorf("expected no decomposition, got %d", len(results))
	}
}

func TestDecompose_SevenPairs(t *testing.T) {
	tiles, _ := ParseTiles("1122334455m11s1z")
	// Not 7 pairs (only 5 pairs + extra), so not valid as 7 pairs
	results := DecomposeSevenPairs(tiles)
	if len(results) != 0 {
		t.Error("expected no seven pairs decomposition")
	}

	// Actual seven pairs
	tiles2, _ := ParseTiles("11223344556677m")
	results2 := DecomposeSevenPairs(tiles2)
	if len(results2) != 1 {
		t.Errorf("expected 1 seven pairs decomposition, got %d", len(results2))
	}
}
