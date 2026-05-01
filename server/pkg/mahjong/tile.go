package mahjong

import (
	"fmt"
	"strconv"
	"strings"
)

type Suit int

const (
	SuitMan   Suit = iota // 万子
	SuitPin               // 筒子
	SuitSou               // 索子
	SuitHonor             // 字牌: 1=東 2=南 3=西 4=北 5=白 6=發 7=中
)

type Tile struct {
	Suit   Suit
	Number int // 1-9 for suited, 1-7 for honors
}

func NewTile(suit Suit, number int) Tile {
	return Tile{Suit: suit, Number: number}
}

func (t Tile) String() string {
	suffixes := []string{"m", "p", "s", "z"}
	return fmt.Sprintf("%d%s", t.Number, suffixes[t.Suit])
}

func (t Tile) IsTerminal() bool {
	return t.Suit != SuitHonor && (t.Number == 1 || t.Number == 9)
}

func (t Tile) IsHonor() bool {
	return t.Suit == SuitHonor
}

func (t Tile) IsTerminalOrHonor() bool {
	return t.IsTerminal() || t.IsHonor()
}

func (t Tile) IsSimple() bool {
	return !t.IsTerminalOrHonor()
}

func ParseTiles(s string) ([]Tile, error) {
	var tiles []Tile
	var numbers []int

	for _, ch := range s {
		switch {
		case ch >= '1' && ch <= '9':
			numbers = append(numbers, int(ch-'0'))
		case ch == 'm' || ch == 'p' || ch == 's' || ch == 'z':
			suit := parseSuitChar(ch)
			for _, n := range numbers {
				tiles = append(tiles, NewTile(suit, n))
			}
			numbers = nil
		default:
			return nil, fmt.Errorf("unexpected character: %c", ch)
		}
	}

	if len(numbers) > 0 {
		return nil, fmt.Errorf("trailing numbers without suit: %s", joinInts(numbers))
	}
	return tiles, nil
}

func parseSuitChar(ch rune) Suit {
	switch ch {
	case 'm':
		return SuitMan
	case 'p':
		return SuitPin
	case 's':
		return SuitSou
	default:
		return SuitHonor
	}
}

func joinInts(nums []int) string {
	strs := make([]string, len(nums))
	for i, n := range nums {
		strs[i] = strconv.Itoa(n)
	}
	return strings.Join(strs, ",")
}
