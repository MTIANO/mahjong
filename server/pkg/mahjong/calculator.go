package mahjong

type ScoreResult struct {
	Han             int
	Fu              int
	RonTotal        int
	TsumoFromParent int
	TsumoFromChild  int
	TsumoTotal      int
	ScoreName       string
}

func CalculateScore(han, fu int, isParent, isTsumo bool) ScoreResult {
	result := ScoreResult{Han: han, Fu: fu}

	if han >= 13 {
		result.ScoreName = "役満"
		applyFixedScore(&result, 8000, isParent, isTsumo)
		return result
	}
	if han >= 11 {
		result.ScoreName = "三倍満"
		applyFixedScore(&result, 6000, isParent, isTsumo)
		return result
	}
	if han >= 8 {
		result.ScoreName = "倍満"
		applyFixedScore(&result, 4000, isParent, isTsumo)
		return result
	}
	if han >= 6 {
		result.ScoreName = "跳満"
		applyFixedScore(&result, 3000, isParent, isTsumo)
		return result
	}
	if han >= 5 {
		result.ScoreName = "満貫"
		applyFixedScore(&result, 2000, isParent, isTsumo)
		return result
	}

	// Basic points = fu * 2^(han+2)
	basic := fu
	for i := 0; i < han+2; i++ {
		basic *= 2
	}
	if basic >= 2000 {
		result.ScoreName = "満貫"
		applyFixedScore(&result, 2000, isParent, isTsumo)
		return result
	}

	if isParent {
		if isTsumo {
			each := roundUp100(basic * 2)
			result.TsumoFromChild = each
			result.TsumoTotal = each * 3
			result.RonTotal = each * 3
		} else {
			result.RonTotal = roundUp100(basic * 6)
		}
	} else {
		if isTsumo {
			result.TsumoFromParent = roundUp100(basic * 2)
			result.TsumoFromChild = roundUp100(basic)
			result.TsumoTotal = result.TsumoFromParent + result.TsumoFromChild*2
			result.RonTotal = result.TsumoTotal
		} else {
			result.RonTotal = roundUp100(basic * 4)
		}
	}

	return result
}

func applyFixedScore(result *ScoreResult, basePoints int, isParent, isTsumo bool) {
	if isParent {
		result.RonTotal = basePoints * 6
		if isTsumo {
			result.TsumoFromChild = basePoints * 2
			result.TsumoTotal = basePoints * 6
		}
	} else {
		result.RonTotal = basePoints * 4
		if isTsumo {
			result.TsumoFromParent = basePoints * 2
			result.TsumoFromChild = basePoints
			result.TsumoTotal = basePoints * 4
		}
	}
}

func roundUp100(n int) int {
	return ((n + 99) / 100) * 100
}
