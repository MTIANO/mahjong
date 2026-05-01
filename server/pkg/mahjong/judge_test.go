package mahjong

import "testing"

func TestJudge_Tanyao(t *testing.T) {
	// 断么九: all simples, no terminals/honors
	// 234m 234p 456p 234s 22s — all simples, valid 14-tile hand
	tiles, _ := ParseTiles("234m234456p234s22s")
	hand := NewHand(tiles, nil, Wind_East, Wind_East, true, false)
	result := Judge(hand)
	if !containsYaku(result, YakuTanyao) {
		t.Errorf("expected Tanyao, got %v", yakuNames(result))
	}
}

func TestJudge_Riichi(t *testing.T) {
	// pair=11m seqs=123m,456p,789s,345s — closed hand with riichi
	tiles, _ := ParseTiles("123m456p789s11m345s")
	hand := NewHand(tiles, nil, Wind_East, Wind_East, false, true)
	result := Judge(hand)
	if !containsYaku(result, YakuRiichi) {
		t.Errorf("expected Riichi, got %v", yakuNames(result))
	}
}

func TestJudge_Tsumo(t *testing.T) {
	// pair=11m seqs=123m,456p,789s,345s — closed hand with tsumo
	tiles, _ := ParseTiles("123m456p789s11m345s")
	hand := NewHand(tiles, nil, Wind_East, Wind_East, true, false)
	result := Judge(hand)
	if !containsYaku(result, YakuTsumo) {
		t.Errorf("expected MenzenTsumo, got %v", yakuNames(result))
	}
}

func TestJudge_Yakuhai(t *testing.T) {
	// 役牌: triplet of dragons (中=7z)
	// pair=1s seqs=123m,123m,456p trip=777z
	tiles, _ := ParseTiles("123123m456p11s777z")
	hand := NewHand(tiles, nil, Wind_East, Wind_East, true, false)
	result := Judge(hand)
	if !containsYaku(result, YakuChun) {
		t.Errorf("expected Chun (中), got %v", yakuNames(result))
	}
}

func TestJudge_YakuhaiWithMeld(t *testing.T) {
	// 碰了白(5z)，手牌11张: 123m456p789s11s
	closed, _ := ParseTiles("123m456p789s11s")
	meld := Meld{Type: MeldPon, Tiles: []Tile{NewTile(SuitHonor, 5), NewTile(SuitHonor, 5), NewTile(SuitHonor, 5)}}
	hand := NewHand(closed, []Meld{meld}, Wind_East, Wind_East, false, false)
	result := Judge(hand)
	if !containsYaku(result, YakuHaku) {
		t.Errorf("expected Haku (白) with pon meld, got %v", yakuNames(result))
	}
}

func TestJudge_SeatWindMeld(t *testing.T) {
	// 碰了東(1z)，自风東，手牌: 234m567p789s22s
	closed, _ := ParseTiles("234m567p789s22s")
	meld := Meld{Type: MeldPon, Tiles: []Tile{NewTile(SuitHonor, 1), NewTile(SuitHonor, 1), NewTile(SuitHonor, 1)}}
	hand := NewHand(closed, []Meld{meld}, Wind_East, Wind_South, false, false)
	result := Judge(hand)
	if !containsYaku(result, YakuTon) {
		t.Errorf("expected seat wind Ton (東), got %v", yakuNames(result))
	}
}

func TestJudge_RoundWindMeld(t *testing.T) {
	// 碰了南(2z)，场风南，自风東，手牌: 234m567p789s22s
	closed, _ := ParseTiles("234m567p789s22s")
	meld := Meld{Type: MeldPon, Tiles: []Tile{NewTile(SuitHonor, 2), NewTile(SuitHonor, 2), NewTile(SuitHonor, 2)}}
	hand := NewHand(closed, []Meld{meld}, Wind_East, Wind_South, false, false)
	result := Judge(hand)
	if !containsYaku(result, YakuBakazeNan) {
		t.Errorf("expected round wind Nan (南), got %v", yakuNames(result))
	}
}

func TestJudge_DoubleWindMeld(t *testing.T) {
	// 碰了東(1z)，自风東+场风東=连风牌，手牌: 234m567p789s22s
	closed, _ := ParseTiles("234m567p789s22s")
	meld := Meld{Type: MeldPon, Tiles: []Tile{NewTile(SuitHonor, 1), NewTile(SuitHonor, 1), NewTile(SuitHonor, 1)}}
	hand := NewHand(closed, []Meld{meld}, Wind_East, Wind_East, false, false)
	result := Judge(hand)
	if !containsYaku(result, YakuTon) {
		t.Errorf("expected seat wind Ton (東), got %v", yakuNames(result))
	}
	if !containsYaku(result, YakuBakazeTon) {
		t.Errorf("expected round wind Ton (東), got %v", yakuNames(result))
	}
}

func TestJudge_NoYaku(t *testing.T) {
	// 无役手牌：有幺九牌但无役
	tiles, _ := ParseTiles("123m456p129s11z89s")
	hand := NewHand(tiles, nil, Wind_East, Wind_East, false, false)
	result := Judge(hand)
	han := 0
	for _, r := range result {
		han += r.Han
	}
	if han != 0 {
		t.Errorf("expected no yaku (0 han), got %v", yakuNames(result))
	}
}

func containsYaku(results []YakuResult, yaku YakuType) bool {
	for _, r := range results {
		if r.Yaku == yaku {
			return true
		}
	}
	return false
}

func yakuNames(results []YakuResult) []string {
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Name
	}
	return names
}
