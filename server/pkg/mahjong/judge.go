package mahjong

func Judge(hand *Hand) []YakuResult {
	allTiles := hand.AllTiles()

	// 国士无双判定
	if hand.IsClosed() && checkKokushi(allTiles) {
		return []YakuResult{makeResult(YakuKokushi, true)}
	}

	// 七对子判定
	if hand.IsClosed() && len(hand.Melds) == 0 {
		pairs := DecomposeSevenPairs(hand.ClosedTiles)
		if len(pairs) > 0 {
			result := judgeChiitoitsu(hand)
			// 也尝试普通分解，取最优
			normalResult := judgeNormal(hand)
			if totalHan(normalResult) > totalHan(result) {
				return normalResult
			}
			return result
		}
	}

	return judgeNormal(hand)
}

func judgeNormal(hand *Hand) []YakuResult {
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

func judgeChiitoitsu(hand *Hand) []YakuResult {
	var results []YakuResult
	results = append(results, makeResult(YakuChiitoitsu, true))

	// 自摸
	if hand.IsTsumo {
		results = append(results, makeResult(YakuTsumo, true))
	}

	// 立直
	if hand.IsRiichi {
		results = append(results, makeResult(YakuRiichi, true))
	}

	// 断幺九
	allSimple := true
	for _, t := range hand.ClosedTiles {
		if t.IsTerminalOrHonor() {
			allSimple = false
			break
		}
	}
	if allSimple {
		results = append(results, makeResult(YakuTanyao, true))
	}

	// 混老头
	if checkHonroutouTiles(hand.ClosedTiles) {
		results = append(results, makeResult(YakuHonroutou, true))
	}

	// 混一色
	if checkHonitsuTiles(hand.ClosedTiles) {
		results = append(results, makeResult(YakuHonitsu, true))
	}

	// 清一色
	if checkChinitsuTiles(hand.ClosedTiles) {
		results = append(results, makeResult(YakuChinitsu, true))
	}

	return results
}

func judgeDecomposition(hand *Hand, decomp Decomposition) []YakuResult {
	var results []YakuResult
	closed := hand.IsClosed()
	allTiles := hand.AllTiles()

	// 所有刻子（含副露）
	allTriplets := getAllTriplets(hand, decomp)
	// 所有顺子（含副露）
	allSequences := getAllSequences(hand, decomp)

	// === 役满判定 ===
	yakuman := judgeYakuman(hand, decomp, allTiles, allTriplets)
	if len(yakuman) > 0 {
		return yakuman
	}

	// === 一般役 ===

	// 立直
	if hand.IsRiichi && closed {
		results = append(results, makeResult(YakuRiichi, true))
	}

	// 門前清自摸和
	if hand.IsTsumo && closed {
		results = append(results, makeResult(YakuTsumo, true))
	}

	// 断么九
	if checkTanyao(allTiles) {
		results = append(results, makeResult(YakuTanyao, closed))
	}

	// 平和
	if closed && checkPinfu(hand, decomp) {
		results = append(results, makeResult(YakuPinfu, true))
	}

	// 二盃口 / 一盃口
	if closed && checkRyanpeikou(decomp) {
		results = append(results, makeResult(YakuRyanpeikou, true))
	} else if closed && checkIipeikou(decomp) {
		results = append(results, makeResult(YakuIipeikou, true))
	}

	// 役牌
	results = append(results, checkYakuhai(hand, allTriplets, closed)...)

	// 対々和
	if checkToitoi(allTriplets, allSequences) {
		results = append(results, makeResult(YakuToitoi, closed))
	}

	// 三暗刻
	if checkSanankou(hand, decomp) {
		results = append(results, makeResult(YakuSanankou, closed))
	}

	// 三色同順
	if checkSanshokuDoujun(allSequences) {
		results = append(results, makeResult(YakuSanshokuDoujun, closed))
	}

	// 一気通貫
	if checkIttsu(allSequences) {
		results = append(results, makeResult(YakuIttsu, closed))
	}

	// 三色同刻
	if checkSanshokuDoukou(allTriplets) {
		results = append(results, makeResult(YakuSanshokuDoukou, closed))
	}

	// 混全帯么九 / 純全帯么九
	if checkJunchan(decomp, hand) {
		results = append(results, makeResult(YakuJunchan, closed))
	} else if checkChanta(decomp, hand) {
		results = append(results, makeResult(YakuChanta, closed))
	}

	// 小三元
	if checkShousangen(allTriplets, decomp.Pair) {
		results = append(results, makeResult(YakuShousangen, closed))
	}

	// 混老頭
	if checkHonroutouTiles(allTiles) {
		results = append(results, makeResult(YakuHonroutou, closed))
	}

	// 清一色 / 混一色
	if checkChinitsuTiles(allTiles) {
		results = append(results, makeResult(YakuChinitsu, closed))
	} else if checkHonitsuTiles(allTiles) {
		results = append(results, makeResult(YakuHonitsu, closed))
	}

	return results
}

// === 役满判定 ===

func judgeYakuman(hand *Hand, decomp Decomposition, allTiles []Tile, allTriplets [][]Tile) []YakuResult {
	var results []YakuResult
	closed := hand.IsClosed()

	// 四暗刻
	if closed && len(decomp.Triplets) == 4 {
		results = append(results, makeResult(YakuSuuankou, true))
	}

	// 大三元
	if checkDaisangen(allTriplets) {
		results = append(results, makeResult(YakuDaisangen, closed))
	}

	// 字一色
	if checkTsuuiisou(allTiles) {
		results = append(results, makeResult(YakuTsuuiisou, closed))
	}

	// 绿一色
	if checkRyuuiisou(allTiles) {
		results = append(results, makeResult(YakuRyuuiisou, closed))
	}

	// 清老头
	if checkChinroutou(allTiles) {
		results = append(results, makeResult(YakuChinroutou, closed))
	}

	// 四喜和
	if checkSuushiihou(allTriplets, decomp.Pair) {
		results = append(results, makeResult(YakuSuushiihou, closed))
	}

	// 九莲宝灯
	if closed && checkChuurenpoutou(hand.ClosedTiles) {
		results = append(results, makeResult(YakuChuurenpoutou, true))
	}

	return results
}

// === 辅助函数 ===

func getAllTriplets(hand *Hand, decomp Decomposition) [][]Tile {
	all := make([][]Tile, len(decomp.Triplets))
	copy(all, decomp.Triplets)
	for _, m := range hand.Melds {
		if m.Type == MeldPon || m.Type == MeldOpenKan || m.Type == MeldClosedKan {
			all = append(all, m.Tiles[:3])
		}
	}
	return all
}

func getAllSequences(hand *Hand, decomp Decomposition) [][]Tile {
	all := make([][]Tile, len(decomp.Sequences))
	copy(all, decomp.Sequences)
	for _, m := range hand.Melds {
		if m.Type == MeldChi {
			all = append(all, m.Tiles[:3])
		}
	}
	return all
}

// === 一般役判定 ===

func checkTanyao(allTiles []Tile) bool {
	for _, t := range allTiles {
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
	return true
}

func checkIipeikou(decomp Decomposition) bool {
	count := countPairs(decomp.Sequences)
	return count >= 1
}

func checkRyanpeikou(decomp Decomposition) bool {
	if len(decomp.Sequences) != 4 {
		return false
	}
	return countPairs(decomp.Sequences) == 2
}

func countPairs(sequences [][]Tile) int {
	used := make([]bool, len(sequences))
	count := 0
	for i := 0; i < len(sequences); i++ {
		if used[i] {
			continue
		}
		for j := i + 1; j < len(sequences); j++ {
			if used[j] {
				continue
			}
			if sequences[i][0] == sequences[j][0] {
				used[i] = true
				used[j] = true
				count++
				break
			}
		}
	}
	return count
}

func checkToitoi(allTriplets, allSequences [][]Tile) bool {
	return len(allTriplets) == 4 && len(allSequences) == 0
}

func checkSanankou(hand *Hand, decomp Decomposition) bool {
	ankou := len(decomp.Triplets)
	for _, m := range hand.Melds {
		if m.Type == MeldClosedKan {
			ankou++
		}
	}
	return ankou >= 3
}

func checkSanshokuDoujun(allSequences [][]Tile) bool {
	for i := 0; i < len(allSequences); i++ {
		for j := i + 1; j < len(allSequences); j++ {
			for k := j + 1; k < len(allSequences); k++ {
				si, sj, sk := allSequences[i], allSequences[j], allSequences[k]
				if si[0].Number == sj[0].Number && sj[0].Number == sk[0].Number {
					suits := map[Suit]bool{si[0].Suit: true, sj[0].Suit: true, sk[0].Suit: true}
					if len(suits) == 3 && !suits[SuitHonor] {
						return true
					}
				}
			}
		}
	}
	return false
}

func checkIttsu(allSequences [][]Tile) bool {
	for suit := SuitMan; suit <= SuitSou; suit++ {
		has1, has4, has7 := false, false, false
		for _, seq := range allSequences {
			if seq[0].Suit == suit {
				switch seq[0].Number {
				case 1:
					has1 = true
				case 4:
					has4 = true
				case 7:
					has7 = true
				}
			}
		}
		if has1 && has4 && has7 {
			return true
		}
	}
	return false
}

func checkSanshokuDoukou(allTriplets [][]Tile) bool {
	for i := 0; i < len(allTriplets); i++ {
		for j := i + 1; j < len(allTriplets); j++ {
			for k := j + 1; k < len(allTriplets); k++ {
				ti, tj, tk := allTriplets[i][0], allTriplets[j][0], allTriplets[k][0]
				if ti.Number == tj.Number && tj.Number == tk.Number {
					suits := map[Suit]bool{ti.Suit: true, tj.Suit: true, tk.Suit: true}
					if len(suits) == 3 && !suits[SuitHonor] {
						return true
					}
				}
			}
		}
	}
	return false
}

func checkChanta(decomp Decomposition, hand *Hand) bool {
	hasHonor := false
	// 检查每个面子是否都含幺九牌
	for _, seq := range decomp.Sequences {
		if !groupHasTerminalOrHonor(seq) {
			return false
		}
	}
	for _, trip := range decomp.Triplets {
		if trip[0].IsHonor() {
			hasHonor = true
		}
		if !trip[0].IsTerminalOrHonor() {
			return false
		}
	}
	for _, m := range hand.Melds {
		if m.Type == MeldChi {
			if !groupHasTerminalOrHonor(m.Tiles) {
				return false
			}
		} else {
			if m.Tiles[0].IsHonor() {
				hasHonor = true
			}
			if !m.Tiles[0].IsTerminalOrHonor() {
				return false
			}
		}
	}
	// 雀头也必须含幺九
	if !decomp.Pair.IsTerminalOrHonor() {
		return false
	}
	if decomp.Pair.IsHonor() {
		hasHonor = true
	}
	return hasHonor // 必须有字牌才是混全带，否则是纯全带
}

func checkJunchan(decomp Decomposition, hand *Hand) bool {
	// 所有面子和雀头含老头牌，且无字牌
	for _, seq := range decomp.Sequences {
		if !groupHasTerminal(seq) {
			return false
		}
	}
	for _, trip := range decomp.Triplets {
		if !trip[0].IsTerminal() {
			return false
		}
	}
	for _, m := range hand.Melds {
		if m.Type == MeldChi {
			if !groupHasTerminal(m.Tiles) {
				return false
			}
		} else {
			if !m.Tiles[0].IsTerminal() {
				return false
			}
		}
	}
	if !decomp.Pair.IsTerminal() {
		return false
	}
	return true
}

func checkShousangen(allTriplets [][]Tile, pair Tile) bool {
	sangenCount := 0
	for _, trip := range allTriplets {
		if trip[0].Suit == SuitHonor && trip[0].Number >= 5 {
			sangenCount++
		}
	}
	pairIsSangen := pair.Suit == SuitHonor && pair.Number >= 5
	return sangenCount == 2 && pairIsSangen
}

func checkHonroutouTiles(tiles []Tile) bool {
	hasHonor := false
	hasTerminal := false
	for _, t := range tiles {
		if t.IsHonor() {
			hasHonor = true
		} else if t.IsTerminal() {
			hasTerminal = true
		} else {
			return false
		}
	}
	return hasHonor && hasTerminal
}

func checkHonitsuTiles(tiles []Tile) bool {
	suit := Suit(-1)
	hasHonor := false
	for _, t := range tiles {
		if t.IsHonor() {
			hasHonor = true
			continue
		}
		if suit == -1 {
			suit = t.Suit
		} else if t.Suit != suit {
			return false
		}
	}
	return hasHonor && suit >= 0
}

func checkChinitsuTiles(tiles []Tile) bool {
	if len(tiles) == 0 {
		return false
	}
	suit := tiles[0].Suit
	if suit == SuitHonor {
		return false
	}
	for _, t := range tiles {
		if t.Suit != suit {
			return false
		}
	}
	return true
}

// === 役满判定 ===

func checkKokushi(tiles []Tile) bool {
	if len(tiles) != 14 {
		return false
	}
	required := []Tile{
		{SuitMan, 1}, {SuitMan, 9},
		{SuitPin, 1}, {SuitPin, 9},
		{SuitSou, 1}, {SuitSou, 9},
		{SuitHonor, 1}, {SuitHonor, 2}, {SuitHonor, 3}, {SuitHonor, 4},
		{SuitHonor, 5}, {SuitHonor, 6}, {SuitHonor, 7},
	}
	counts := map[Tile]int{}
	for _, t := range tiles {
		counts[t]++
	}
	for _, r := range required {
		if counts[r] == 0 {
			return false
		}
	}
	return true
}

func checkDaisangen(allTriplets [][]Tile) bool {
	count := 0
	for _, trip := range allTriplets {
		if trip[0].Suit == SuitHonor && trip[0].Number >= 5 {
			count++
		}
	}
	return count == 3
}

func checkTsuuiisou(tiles []Tile) bool {
	for _, t := range tiles {
		if !t.IsHonor() {
			return false
		}
	}
	return true
}

func checkRyuuiisou(tiles []Tile) bool {
	for _, t := range tiles {
		if t.Suit == SuitSou {
			switch t.Number {
			case 2, 3, 4, 6, 8:
				continue
			default:
				return false
			}
		} else if t.Suit == SuitHonor && t.Number == 6 { // 發
			continue
		} else {
			return false
		}
	}
	return true
}

func checkChinroutou(tiles []Tile) bool {
	for _, t := range tiles {
		if !t.IsTerminal() {
			return false
		}
	}
	return true
}

func checkSuushiihou(allTriplets [][]Tile, pair Tile) bool {
	windCount := 0
	for _, trip := range allTriplets {
		if trip[0].Suit == SuitHonor && trip[0].Number >= 1 && trip[0].Number <= 4 {
			windCount++
		}
	}
	// 大四喜: 4刻 或 小四喜: 3刻+雀头
	if windCount == 4 {
		return true
	}
	if windCount == 3 && pair.Suit == SuitHonor && pair.Number >= 1 && pair.Number <= 4 {
		return true
	}
	return false
}

func checkChuurenpoutou(tiles []Tile) bool {
	if len(tiles) != 14 {
		return false
	}
	suit := tiles[0].Suit
	if suit == SuitHonor {
		return false
	}
	counts := [10]int{}
	for _, t := range tiles {
		if t.Suit != suit {
			return false
		}
		counts[t.Number]++
	}
	// 需要 1112345678999 + 任意1张
	required := [10]int{0, 3, 1, 1, 1, 1, 1, 1, 1, 3}
	extra := 0
	for i := 1; i <= 9; i++ {
		diff := counts[i] - required[i]
		if diff < 0 {
			return false
		}
		extra += diff
	}
	return extra == 1
}

// === 役牌判定 ===

func checkYakuhai(hand *Hand, allTriplets [][]Tile, closed bool) []YakuResult {
	var results []YakuResult
	for _, trip := range allTriplets {
		t := trip[0]
		if t.Suit != SuitHonor {
			continue
		}
		switch t.Number {
		case 5:
			results = append(results, makeResult(YakuHaku, closed))
		case 6:
			results = append(results, makeResult(YakuHatsu, closed))
		case 7:
			results = append(results, makeResult(YakuChun, closed))
		}
		windNum := int(hand.SeatWind) + 1
		if t.Number == windNum {
			yaku := []YakuType{YakuTon, YakuNan, YakuXia, YakuPei}[hand.SeatWind]
			results = append(results, makeResult(yaku, closed))
		}
		roundNum := int(hand.RoundWind) + 1
		if t.Number == roundNum {
			yaku := []YakuType{YakuBakazeTon, YakuBakazeNan, YakuBakazeXia, YakuBakazePei}[hand.RoundWind]
			results = append(results, makeResult(yaku, closed))
		}
	}
	return results
}

// === 通用辅助 ===

func isYakuhaiTile(t Tile, seatWind, roundWind Wind) bool {
	if t.Suit != SuitHonor {
		return false
	}
	if t.Number >= 5 {
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

func groupHasTerminalOrHonor(tiles []Tile) bool {
	for _, t := range tiles {
		if t.IsTerminalOrHonor() {
			return true
		}
	}
	return false
}

func groupHasTerminal(tiles []Tile) bool {
	for _, t := range tiles {
		if t.IsTerminal() {
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
