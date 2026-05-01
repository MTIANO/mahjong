package mahjong

import "sort"

type Decomposition struct {
	Pair      Tile
	Sequences [][]Tile // 順子
	Triplets  [][]Tile // 刻子
}

func Decompose(tiles []Tile) []Decomposition {
	n := len(tiles)
	// 手牌数必须是 3k+2 (14, 11, 8, 5, 2)
	if n < 2 || (n-2)%3 != 0 {
		return nil
	}

	sorted := make([]Tile, len(tiles))
	copy(sorted, tiles)
	sortTiles(sorted)

	var results []Decomposition
	seen := map[string]bool{}

	for i := 0; i < len(sorted)-1; i++ {
		if i > 0 && sorted[i] == sorted[i-1] {
			continue
		}
		if sorted[i] == sorted[i+1] {
			pair := sorted[i]
			remaining := removeTwo(sorted, i)
			decomps := findMentsu(remaining, nil, nil)
			for _, d := range decomps {
				d.Pair = pair
				key := decompKey(d)
				if !seen[key] {
					seen[key] = true
					results = append(results, d)
				}
			}
		}
	}
	return results
}

func DecomposeSevenPairs(tiles []Tile) []Decomposition {
	if len(tiles) != 14 {
		return nil
	}

	sorted := make([]Tile, len(tiles))
	copy(sorted, tiles)
	sortTiles(sorted)

	for i := 0; i < 14; i += 2 {
		if sorted[i] != sorted[i+1] {
			return nil
		}
	}

	return []Decomposition{{Pair: sorted[0]}}
}

func findMentsu(tiles []Tile, seqs, trips [][]Tile) []Decomposition {
	if len(tiles) == 0 {
		return []Decomposition{{Sequences: copyGroups(seqs), Triplets: copyGroups(trips)}}
	}

	var results []Decomposition
	first := tiles[0]

	// Try triplet (刻子)
	if len(tiles) >= 3 && tiles[1] == first && tiles[2] == first {
		rest := tiles[3:]
		results = append(results, findMentsu(rest, seqs, append(trips, []Tile{first, first, first}))...)
	}

	// Try sequence (順子) — only for suited tiles
	if first.Suit != SuitHonor {
		second := Tile{Suit: first.Suit, Number: first.Number + 1}
		third := Tile{Suit: first.Suit, Number: first.Number + 2}
		idx2 := findTile(tiles[1:], second)
		if idx2 >= 0 {
			remaining1 := removeTile(tiles[1:], idx2)
			idx3 := findTile(remaining1, third)
			if idx3 >= 0 {
				remaining2 := removeTile(remaining1, idx3)
				results = append(results, findMentsu(remaining2, append(seqs, []Tile{first, second, third}), trips)...)
			}
		}
	}

	return results
}

func sortTiles(tiles []Tile) {
	sort.Slice(tiles, func(i, j int) bool {
		if tiles[i].Suit != tiles[j].Suit {
			return tiles[i].Suit < tiles[j].Suit
		}
		return tiles[i].Number < tiles[j].Number
	})
}

func removeTwo(tiles []Tile, idx int) []Tile {
	result := make([]Tile, 0, len(tiles)-2)
	result = append(result, tiles[:idx]...)
	result = append(result, tiles[idx+2:]...)
	return result
}

func findTile(tiles []Tile, target Tile) int {
	for i, t := range tiles {
		if t == target {
			return i
		}
	}
	return -1
}

func removeTile(tiles []Tile, idx int) []Tile {
	result := make([]Tile, 0, len(tiles)-1)
	result = append(result, tiles[:idx]...)
	result = append(result, tiles[idx+1:]...)
	return result
}

func copyGroups(groups [][]Tile) [][]Tile {
	if groups == nil {
		return nil
	}
	cp := make([][]Tile, len(groups))
	for i, g := range groups {
		cp[i] = make([]Tile, len(g))
		copy(cp[i], g)
	}
	return cp
}

func decompKey(d Decomposition) string {
	key := d.Pair.String() + "|"
	for _, s := range d.Sequences {
		key += s[0].String()
	}
	key += "|"
	for _, tr := range d.Triplets {
		key += tr[0].String()
	}
	return key
}
