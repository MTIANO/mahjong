package mahjong

type Wind int

const (
	Wind_East  Wind = iota // 東
	Wind_South             // 南
	Wind_West              // 西
	Wind_North             // 北
)

type MeldType int

const (
	MeldChi      MeldType = iota // 吃 (sequence from another player)
	MeldPon                      // 碰
	MeldOpenKan                  // 明槓
	MeldClosedKan                // 暗槓
)

type Meld struct {
	Type  MeldType
	Tiles []Tile
}

type Hand struct {
	ClosedTiles []Tile
	Melds       []Meld
	WinTile     Tile
	SeatWind    Wind
	RoundWind   Wind
	IsTsumo     bool // 自摸
	IsRiichi    bool // 立直
}

func NewHand(closedTiles []Tile, melds []Meld, seatWind, roundWind Wind, isTsumo, isRiichi bool) *Hand {
	return &Hand{
		ClosedTiles: closedTiles,
		Melds:       melds,
		SeatWind:    seatWind,
		RoundWind:   roundWind,
		IsTsumo:     isTsumo,
		IsRiichi:    isRiichi,
	}
}

func (h *Hand) IsClosed() bool {
	for _, m := range h.Melds {
		if m.Type != MeldClosedKan {
			return false
		}
	}
	return true
}

func (h *Hand) AllTiles() []Tile {
	all := make([]Tile, len(h.ClosedTiles))
	copy(all, h.ClosedTiles)
	for _, m := range h.Melds {
		all = append(all, m.Tiles...)
	}
	return all
}
