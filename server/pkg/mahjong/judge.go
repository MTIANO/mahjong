package mahjong

func Judge(hand *Hand) []YakuResult {
	decomps := Decompose(hand.ClosedTiles)
	if len(decomps) == 0 {
		return nil
	}

	var bestResult []YakuResult
	bestHan := 0

	for _, decomp := range decomps {
		result := judgeDecomposition(hand, decomp)
		han := totalHan(result)
		if han > bestHan {
			bestHan = han
			bestResult = result
		}
	}

	return bestResult
}

func judgeDecomposition(hand *Hand, decomp Decomposition) []YakuResult {
	var results []YakuResult
	closed := hand.IsClosed()

	// 立直
	if hand.IsRiichi && closed {
		results = append(results, makeResult(YakuRiichi, true))
	}

	// 門前清自摸和
	if hand.IsTsumo && closed {
		results = append(results, makeResult(YakuTsumo, true))
	}

	// 断么九
	if checkTanyao(hand, decomp) {
		results = append(results, makeResult(YakuTanyao, closed))
	}

	// 平和
	if closed && checkPinfu(hand, decomp) {
		results = append(results, makeResult(YakuPinfu, true))
	}

	// 一盃口
	if closed && checkIipeikou(decomp) {
		results = append(results, makeResult(YakuIipeikou, true))
	}

	// 役牌
	results = append(results, checkYakuhai(hand, decomp)...)

	return results
}

func checkTanyao(hand *Hand, decomp Decomposition) bool {
	for _, t := range hand.AllTiles() {
		if t.IsTerminalOrHonor() {
			return false
		}
	}
	return true
}

func checkPinfu(hand *Hand, decomp Decomposition) bool {
	if len(decomp.Triplets) > 0 {
		return false
	}
	if len(decomp.Sequences) != 4 {
		return false
	}
	pair := decomp.Pair
	if isYakuhaiTile(pair, hand.SeatWind, hand.RoundWind) {
		return false
	}
	if hand.WinTile == (Tile{}) {
		return false
	}
	return hasTwoSidedWait(decomp.Sequences, hand.WinTile)
}

func checkIipeikou(decomp Decomposition) bool {
	for i := 0; i < len(decomp.Sequences); i++ {
		for j := i + 1; j < len(decomp.Sequences); j++ {
			if decomp.Sequences[i][0] == decomp.Sequences[j][0] {
				return true
			}
		}
	}
	return false
}

func checkYakuhai(hand *Hand, decomp Decomposition) []YakuResult {
	var results []YakuResult
	closed := hand.IsClosed()

	allTriplets := decomp.Triplets
	for _, m := range hand.Melds {
		if m.Type == MeldPon || m.Type == MeldOpenKan || m.Type == MeldClosedKan {
			allTriplets = append(allTriplets, m.Tiles[:3])
		}
	}

	for _, trip := range allTriplets {
		t := trip[0]
		if t.Suit != SuitHonor {
			continue
		}
		switch t.Number {
		case 5: // 白
			results = append(results, makeResult(YakuHaku, closed))
		case 6: // 發
			results = append(results, makeResult(YakuHatsu, closed))
		case 7: // 中
			results = append(results, makeResult(YakuChun, closed))
		}
		// Seat wind
		windNum := int(hand.SeatWind) + 1
		if t.Number == windNum {
			yaku := []YakuType{YakuTon, YakuNan, YakuXia, YakuPei}[hand.SeatWind]
			results = append(results, makeResult(yaku, closed))
		}
		// Round wind
		roundNum := int(hand.RoundWind) + 1
		if t.Number == roundNum {
			yaku := []YakuType{YakuBakazeTon, YakuBakazeNan, YakuBakazeXia, YakuBakazePei}[hand.RoundWind]
			results = append(results, makeResult(yaku, closed))
		}
	}
	return results
}

func isYakuhaiTile(t Tile, seatWind, roundWind Wind) bool {
	if t.Suit != SuitHonor {
		return false
	}
	if t.Number >= 5 { // 白發中
		return true
	}
	if t.Number == int(seatWind)+1 {
		return true
	}
	if t.Number == int(roundWind)+1 {
		return true
	}
	return false
}

func hasTwoSidedWait(sequences [][]Tile, winTile Tile) bool {
	for _, seq := range sequences {
		if seq[0] == winTile && seq[0].Number != 7 {
			return true
		}
		if seq[2] == winTile && seq[2].Number != 3 {
			return true
		}
	}
	return false
}

func makeResult(yaku YakuType, closed bool) YakuResult {
	info := yakuInfo[yaku]
	han := info.HanClosed
	if !closed {
		han = info.HanOpen
	}
	if han == 0 {
		return YakuResult{}
	}
	return YakuResult{Yaku: yaku, Name: info.Name, Han: han}
}

func totalHan(results []YakuResult) int {
	total := 0
	for _, r := range results {
		total += r.Han
	}
	return total
}
