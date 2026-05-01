package mahjong

import "testing"

func TestCalculateScore_1Han30Fu_Child(t *testing.T) {
	score := CalculateScore(1, 30, false, false)
	// 子ロン: 1000
	if score.RonTotal != 1000 {
		t.Errorf("expected ron=1000, got %d", score.RonTotal)
	}
}

func TestCalculateScore_3Han40Fu_Child(t *testing.T) {
	score := CalculateScore(3, 40, false, false)
	// 子ロン: 5200
	if score.RonTotal != 5200 {
		t.Errorf("expected ron=5200, got %d", score.RonTotal)
	}
}

func TestCalculateScore_4Han30Fu_Child(t *testing.T) {
	score := CalculateScore(4, 30, false, false)
	// 子ロン: 7700 (basic points = 30*2^6 = 1920 < 2000)
	if score.RonTotal != 7700 {
		t.Errorf("expected ron=7700, got %d", score.RonTotal)
	}
}

func TestCalculateScore_Mangan_Child(t *testing.T) {
	score := CalculateScore(5, 0, false, false)
	// 満貫 子ロン: 8000
	if score.RonTotal != 8000 {
		t.Errorf("expected ron=8000, got %d", score.RonTotal)
	}
}

func TestCalculateScore_Mangan_Parent(t *testing.T) {
	score := CalculateScore(5, 0, true, false)
	// 満貫 親ロン: 12000
	if score.RonTotal != 12000 {
		t.Errorf("expected ron=12000, got %d", score.RonTotal)
	}
}

func TestCalculateScore_Tsumo_Child(t *testing.T) {
	score := CalculateScore(3, 40, false, true)
	// 子ツモ: parent pays 2600, children pay 1300
	if score.TsumoFromParent != 2600 {
		t.Errorf("expected tsumo parent=2600, got %d", score.TsumoFromParent)
	}
	if score.TsumoFromChild != 1300 {
		t.Errorf("expected tsumo child=1300, got %d", score.TsumoFromChild)
	}
}

func TestCalculateScore_Tsumo_Parent(t *testing.T) {
	score := CalculateScore(3, 40, true, true)
	// 親ツモ: all children pay 2600
	if score.TsumoFromChild != 2600 {
		t.Errorf("expected tsumo all=2600, got %d", score.TsumoFromChild)
	}
}
