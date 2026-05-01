# MTiano 日麻工具集合 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a WeChat Mini Program + Go backend that provides mahjong yaku/score lookup and photo-based tile recognition with scoring.

**Architecture:** Static JSON data embedded in the Mini Program for fast local lookup of yaku and score tables. Go backend (Gin) handles image upload → AI vision recognition → scoring engine → return results. MySQL stores future user data.

**Tech Stack:** Native WeChat Mini Program, Go 1.26, Gin, MySQL, AI Vision API (interface-abstracted)

---

## Task 1: Go Module & Project Scaffolding

**Files:**
- Create: `server/go.mod`
- Create: `server/cmd/server/main.go`
- Create: `server/internal/config/config.go`
- Create: `server/configs/config.yaml`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/buchiyudan/Documents/mtiano
mkdir -p server/cmd/server server/internal/config server/configs
cd server
go mod init github.com/mtiano/server
```

- [ ] **Step 2: Install dependencies**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go get github.com/gin-gonic/gin
go get gopkg.in/yaml.v3
```

- [ ] **Step 3: Create config struct and loader**

`server/internal/config/config.go`:
```go
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Vision VisionConfig `yaml:"vision"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

type VisionConfig struct {
	Provider string `yaml:"provider"`
	APIKey   string `yaml:"api_key"`
	Endpoint string `yaml:"endpoint"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
```

- [ ] **Step 4: Create config file**

`server/configs/config.yaml`:
```yaml
server:
  port: ":8080"

vision:
  provider: "stub"
  api_key: ""
  endpoint: ""
```

- [ ] **Step 5: Create main.go with Gin server**

`server/cmd/server/main.go`:
```go
package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/config"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	r := gin.Default()

	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	if err := r.Run(cfg.Server.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
```

- [ ] **Step 6: Verify server starts**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go run cmd/server/main.go &
sleep 2
curl http://localhost:8080/api/v1/health
# Expected: {"status":"ok"}
kill %1
```

- [ ] **Step 7: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add server/
git commit -m "feat(server): scaffold Go project with Gin and config loading"
```

---

## Task 2: Mahjong Tile Model

**Files:**
- Create: `server/pkg/mahjong/tile.go`
- Create: `server/pkg/mahjong/tile_test.go`

- [ ] **Step 1: Write failing test for tile types and parsing**

`server/pkg/mahjong/tile_test.go`:
```go
package mahjong

import (
	"testing"
)

func TestNewTile(t *testing.T) {
	tile := NewTile(SuitMan, 1)
	if tile.Suit != SuitMan || tile.Number != 1 {
		t.Errorf("expected 1m, got %v", tile)
	}
}

func TestTileString(t *testing.T) {
	tests := []struct {
		tile Tile
		want string
	}{
		{NewTile(SuitMan, 1), "1m"},
		{NewTile(SuitPin, 5), "5p"},
		{NewTile(SuitSou, 9), "9s"},
		{NewTile(SuitHonor, 1), "1z"},
	}
	for _, tt := range tests {
		if got := tt.tile.String(); got != tt.want {
			t.Errorf("Tile.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestParseTiles(t *testing.T) {
	tiles, err := ParseTiles("123m456p789s11z")
	if err != nil {
		t.Fatalf("ParseTiles error: %v", err)
	}
	if len(tiles) != 11 {
		t.Fatalf("expected 11 tiles, got %d", len(tiles))
	}
	if tiles[0].String() != "1m" {
		t.Errorf("first tile = %q, want \"1m\"", tiles[0].String())
	}
	if tiles[9].String() != "1z" {
		t.Errorf("tile[9] = %q, want \"1z\"", tiles[9].String())
	}
}

func TestIsTerminal(t *testing.T) {
	if !NewTile(SuitMan, 1).IsTerminal() {
		t.Error("1m should be terminal")
	}
	if !NewTile(SuitMan, 9).IsTerminal() {
		t.Error("9m should be terminal")
	}
	if NewTile(SuitMan, 5).IsTerminal() {
		t.Error("5m should not be terminal")
	}
}

func TestIsHonor(t *testing.T) {
	if !NewTile(SuitHonor, 1).IsHonor() {
		t.Error("1z should be honor")
	}
	if NewTile(SuitMan, 1).IsHonor() {
		t.Error("1m should not be honor")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./pkg/mahjong/ -v
```
Expected: compilation error — package/types not defined yet.

- [ ] **Step 3: Implement tile model**

`server/pkg/mahjong/tile.go`:
```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./pkg/mahjong/ -v
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add server/pkg/mahjong/
git commit -m "feat(mahjong): add tile model with parsing and classification"
```

---

## Task 3: Hand Model & Meld Types

**Files:**
- Create: `server/pkg/mahjong/hand.go`
- Create: `server/pkg/mahjong/hand_test.go`

- [ ] **Step 1: Write failing test for hand model**

`server/pkg/mahjong/hand_test.go`:
```go
package mahjong

import "testing"

func TestNewHand(t *testing.T) {
	tiles, _ := ParseTiles("123m456p789s1122z")
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
	tiles, _ := ParseTiles("123m456p789s1122z")
	hand := NewHand(tiles, nil, Wind_East, Wind_East, false, false)
	if !hand.IsClosed() {
		t.Error("hand with no melds should be closed")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./pkg/mahjong/ -v -run TestNewHand
```
Expected: compilation error.

- [ ] **Step 3: Implement hand model**

`server/pkg/mahjong/hand.go`:
```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./pkg/mahjong/ -v -run TestHand
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add server/pkg/mahjong/hand.go server/pkg/mahjong/hand_test.go
git commit -m "feat(mahjong): add hand model with melds and wind context"
```

---

## Task 4: Hand Parser (Decompose into Mentsu)

**Files:**
- Create: `server/pkg/mahjong/parser.go`
- Create: `server/pkg/mahjong/parser_test.go`

- [ ] **Step 1: Write failing test for hand decomposition**

`server/pkg/mahjong/parser_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./pkg/mahjong/ -v -run TestDecompose
```
Expected: compilation error.

- [ ] **Step 3: Implement hand decomposition**

`server/pkg/mahjong/parser.go`:
```go
package mahjong

import "sort"

type Decomposition struct {
	Pair      Tile
	Sequences [][]Tile // 順子
	Triplets  [][]Tile // 刻子
}

func Decompose(tiles []Tile) []Decomposition {
	if len(tiles) != 14 {
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./pkg/mahjong/ -v -run TestDecompose
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add server/pkg/mahjong/parser.go server/pkg/mahjong/parser_test.go
git commit -m "feat(mahjong): add hand decomposition into mentsu groups"
```

---

## Task 5: Yaku Definitions & Judge

**Files:**
- Create: `server/pkg/mahjong/yaku.go`
- Create: `server/pkg/mahjong/judge.go`
- Create: `server/pkg/mahjong/judge_test.go`

- [ ] **Step 1: Write failing test for yaku judgment**

`server/pkg/mahjong/judge_test.go`:
```go
package mahjong

import "testing"

func TestJudge_Tanyao(t *testing.T) {
	// 断么九: all simples, no terminals/honors
	tiles, _ := ParseTiles("234m345p456s2233p")
	hand := NewHand(tiles, nil, Wind_East, Wind_East, true, false)
	result := Judge(hand)
	if !containsYaku(result, YakuTanyao) {
		t.Errorf("expected Tanyao, got %v", yakuNames(result))
	}
}

func TestJudge_Riichi(t *testing.T) {
	tiles, _ := ParseTiles("123m456p789s1122z")
	hand := NewHand(tiles, nil, Wind_East, Wind_East, false, true)
	result := Judge(hand)
	if !containsYaku(result, YakuRiichi) {
		t.Errorf("expected Riichi, got %v", yakuNames(result))
	}
}

func TestJudge_Tsumo(t *testing.T) {
	tiles, _ := ParseTiles("123m456p789s1122z")
	hand := NewHand(tiles, nil, Wind_East, Wind_East, true, false)
	result := Judge(hand)
	if !containsYaku(result, YakuTsumo) {
		t.Errorf("expected MenzenTsumo, got %v", yakuNames(result))
	}
}

func TestJudge_Pinfu(t *testing.T) {
	// 平和: all sequences, non-yakuhai pair, two-sided wait
	tiles, _ := ParseTiles("123m456p789s2345m")
	hand := NewHand(tiles, nil, Wind_South, Wind_East, true, false)
	hand.WinTile = NewTile(SuitMan, 2)
	result := Judge(hand)
	if !containsYaku(result, YakuPinfu) {
		t.Errorf("expected Pinfu, got %v", yakuNames(result))
	}
}

func TestJudge_Yakuhai(t *testing.T) {
	// 役牌: triplet of dragons (中=7z)
	tiles, _ := ParseTiles("123m456p11s777z23m")
	hand := NewHand(tiles, nil, Wind_East, Wind_East, true, false)
	result := Judge(hand)
	if !containsYaku(result, YakuChun) {
		t.Errorf("expected Chun (中), got %v", yakuNames(result))
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./pkg/mahjong/ -v -run TestJudge
```
Expected: compilation error.

- [ ] **Step 3: Create yaku type definitions**

`server/pkg/mahjong/yaku.go`:
```go
package mahjong

type YakuType int

const (
	YakuRiichi  YakuType = iota // 立直
	YakuTsumo                   // 門前清自摸和
	YakuTanyao                  // 断么九
	YakuPinfu                   // 平和
	YakuIipeikou                // 一盃口
	YakuTon                     // 自風牌 東
	YakuNan                     // 自風牌 南
	YakuXia                     // 自風牌 西
	YakuPei                     // 自風牌 北
	YakuBakazeTon               // 場風牌 東
	YakuBakazeNan               // 場風牌 南
	YakuBakazeXia               // 場風牌 西
	YakuBakazePei               // 場風牌 北
	YakuHaku                    // 白
	YakuHatsu                   // 發
	YakuChun                    // 中
)

type YakuResult struct {
	Yaku YakuType
	Name string
	Han  int
}

var yakuInfo = map[YakuType]struct {
	Name    string
	HanOpen int
	HanClosed int
}{
	YakuRiichi:    {"立直", 0, 1},
	YakuTsumo:    {"門前清自摸和", 0, 1},
	YakuTanyao:   {"断么九", 1, 1},
	YakuPinfu:    {"平和", 0, 1},
	YakuIipeikou: {"一盃口", 0, 1},
	YakuTon:      {"自風 東", 1, 1},
	YakuNan:      {"自風 南", 1, 1},
	YakuXia:      {"自風 西", 1, 1},
	YakuPei:      {"自風 北", 1, 1},
	YakuBakazeTon: {"場風 東", 1, 1},
	YakuBakazeNan: {"場風 南", 1, 1},
	YakuBakazeXia: {"場風 西", 1, 1},
	YakuBakazePei: {"場風 北", 1, 1},
	YakuHaku:     {"白", 1, 1},
	YakuHatsu:    {"發", 1, 1},
	YakuChun:     {"中", 1, 1},
}
```

- [ ] **Step 4: Implement the judge**

`server/pkg/mahjong/judge.go`:
```go
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
	// Must be all sequences
	if len(decomp.Triplets) > 0 {
		return false
	}
	if len(decomp.Sequences) != 4 {
		return false
	}
	// Pair must not be yakuhai
	pair := decomp.Pair
	if isYakuhaiTile(pair, hand.SeatWind, hand.RoundWind) {
		return false
	}
	// Win tile must allow a two-sided wait (両面待ち)
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
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./pkg/mahjong/ -v -run TestJudge
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add server/pkg/mahjong/yaku.go server/pkg/mahjong/judge.go server/pkg/mahjong/judge_test.go
git commit -m "feat(mahjong): add yaku definitions and judge engine for basic yaku"
```

---

## Task 6: Score Calculator

**Files:**
- Create: `server/pkg/mahjong/calculator.go`
- Create: `server/pkg/mahjong/calculator_test.go`

- [ ] **Step 1: Write failing test for score calculation**

`server/pkg/mahjong/calculator_test.go`:
```go
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
	// 子ロン: 7700 (not mangan yet — basic points = 30*2^6 = 1920 < 2000)
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./pkg/mahjong/ -v -run TestCalculateScore
```
Expected: compilation error.

- [ ] **Step 3: Implement score calculator**

`server/pkg/mahjong/calculator.go`:
```go
package mahjong

type ScoreResult struct {
	Han             int
	Fu              int
	RonTotal        int
	TsumoFromParent int
	TsumoFromChild  int
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
			result.RonTotal = each * 3
		} else {
			result.RonTotal = roundUp100(basic * 6)
		}
	} else {
		if isTsumo {
			result.TsumoFromParent = roundUp100(basic * 2)
			result.TsumoFromChild = roundUp100(basic)
			result.RonTotal = result.TsumoFromParent + result.TsumoFromChild*2
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
		}
	} else {
		result.RonTotal = basePoints * 4
		if isTsumo {
			result.TsumoFromParent = basePoints * 2
			result.TsumoFromChild = basePoints
		}
	}
}

func roundUp100(n int) int {
	return ((n + 99) / 100) * 100
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./pkg/mahjong/ -v -run TestCalculateScore
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add server/pkg/mahjong/calculator.go server/pkg/mahjong/calculator_test.go
git commit -m "feat(mahjong): add score calculator with han/fu lookup"
```

---

## Task 7: Vision Service Interface & Stub

**Files:**
- Create: `server/internal/service/vision.go`
- Create: `server/internal/service/vision_stub.go`
- Create: `server/internal/service/vision_test.go`

- [ ] **Step 1: Write test for the stub vision service**

`server/internal/service/vision_test.go`:
```go
package service

import (
	"context"
	"testing"

	"github.com/mtiano/server/pkg/mahjong"
)

func TestStubVisionService(t *testing.T) {
	svc := NewStubVisionService()
	tiles, err := svc.RecognizeTiles(context.Background(), []byte("fake image"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tiles) != 14 {
		t.Errorf("expected 14 tiles from stub, got %d", len(tiles))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./internal/service/ -v
```
Expected: compilation error.

- [ ] **Step 3: Implement vision interface and stub**

`server/internal/service/vision.go`:
```go
package service

import (
	"context"

	"github.com/mtiano/server/pkg/mahjong"
)

type VisionService interface {
	RecognizeTiles(ctx context.Context, image []byte) ([]mahjong.Tile, error)
}
```

`server/internal/service/vision_stub.go`:
```go
package service

import (
	"context"

	"github.com/mtiano/server/pkg/mahjong"
)

type StubVisionService struct{}

func NewStubVisionService() *StubVisionService {
	return &StubVisionService{}
}

func (s *StubVisionService) RecognizeTiles(ctx context.Context, image []byte) ([]mahjong.Tile, error) {
	// Return a fixed winning hand for development/testing
	tiles, _ := mahjong.ParseTiles("123m456p789s11z")
	// Pad to 14 tiles
	extra, _ := mahjong.ParseTiles("23z")
	tiles = append(tiles, extra...)
	return tiles, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./internal/service/ -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add server/internal/service/
git commit -m "feat(server): add vision service interface with stub implementation"
```

---

## Task 8: Recognize Handler & API Wiring

**Files:**
- Create: `server/internal/handler/recognize.go`
- Create: `server/internal/handler/handler_test.go`
- Modify: `server/cmd/server/main.go`

- [ ] **Step 1: Write test for recognize handler**

`server/internal/handler/handler_test.go`:
```go
package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/service"
)

func TestRecognizeHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	vision := service.NewStubVisionService()
	h := NewRecognizeHandler(vision)
	r.POST("/api/v1/recognize", h.Handle)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("image", "test.jpg")
	part.Write([]byte("fake image data"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/recognize", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp RecognizeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Tiles) == 0 {
		t.Error("expected tiles in response")
	}
}

func TestRecognizeHandler_NoImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	vision := service.NewStubVisionService()
	h := NewRecognizeHandler(vision)
	r.POST("/api/v1/recognize", h.Handle)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/recognize", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./internal/handler/ -v
```
Expected: compilation error.

- [ ] **Step 3: Implement recognize handler**

`server/internal/handler/recognize.go`:
```go
package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/service"
	"github.com/mtiano/server/pkg/mahjong"
)

type RecognizeHandler struct {
	vision service.VisionService
}

type RecognizeResponse struct {
	Tiles    []string             `json:"tiles"`
	Yaku     []YakuResponse       `json:"yaku"`
	TotalHan int                  `json:"total_han"`
	Score    mahjong.ScoreResult  `json:"score"`
}

type YakuResponse struct {
	Name string `json:"name"`
	Han  int    `json:"han"`
}

func NewRecognizeHandler(vision service.VisionService) *RecognizeHandler {
	return &RecognizeHandler{vision: vision}
}

func (h *RecognizeHandler) Handle(c *gin.Context) {
	file, _, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file required"})
		return
	}
	defer file.Close()

	imageData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read image"})
		return
	}

	tiles, err := h.vision.RecognizeTiles(c.Request.Context(), imageData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "recognition failed"})
		return
	}

	hand := mahjong.NewHand(tiles, nil, mahjong.Wind_East, mahjong.Wind_East, true, false)
	yakuResults := mahjong.Judge(hand)

	totalHan := 0
	var yakuResp []YakuResponse
	for _, y := range yakuResults {
		if y.Han > 0 {
			totalHan += y.Han
			yakuResp = append(yakuResp, YakuResponse{Name: y.Name, Han: y.Han})
		}
	}

	score := mahjong.CalculateScore(totalHan, 30, false, true)

	tileStrs := make([]string, len(tiles))
	for i, t := range tiles {
		tileStrs[i] = t.String()
	}

	c.JSON(http.StatusOK, RecognizeResponse{
		Tiles:    tileStrs,
		Yaku:     yakuResp,
		TotalHan: totalHan,
		Score:    score,
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go test ./internal/handler/ -v
```
Expected: all PASS.

- [ ] **Step 5: Update main.go to wire handler**

`server/cmd/server/main.go`:
```go
package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/config"
	"github.com/mtiano/server/internal/handler"
	"github.com/mtiano/server/internal/service"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	var vision service.VisionService
	switch cfg.Vision.Provider {
	default:
		vision = service.NewStubVisionService()
	}

	r := gin.Default()

	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	recognizeHandler := handler.NewRecognizeHandler(vision)
	r.POST("/api/v1/recognize", recognizeHandler.Handle)

	if err := r.Run(cfg.Server.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
```

- [ ] **Step 6: Verify full server integration**

```bash
cd /Users/buchiyudan/Documents/mtiano/server
go build ./cmd/server/
```
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add server/internal/handler/ server/cmd/server/main.go
git commit -m "feat(server): add recognize handler and wire API routes"
```

---

## Task 9: Mini Program Scaffolding

**Files:**
- Create: `miniprogram/app.js`
- Create: `miniprogram/app.json`
- Create: `miniprogram/app.wxss`
- Create: `miniprogram/project.config.json`
- Create: `miniprogram/sitemap.json`

- [ ] **Step 1: Create Mini Program app files**

`miniprogram/app.json`:
```json
{
  "pages": [
    "pages/index/index",
    "pages/yaku/yaku",
    "pages/score/score",
    "pages/camera/camera"
  ],
  "window": {
    "navigationBarBackgroundColor": "#1a1a2e",
    "navigationBarTitleText": "MTiano",
    "navigationBarTextStyle": "white",
    "backgroundColor": "#f5f5f5"
  },
  "tabBar": {
    "color": "#999",
    "selectedColor": "#1a1a2e",
    "list": [
      {
        "pagePath": "pages/index/index",
        "text": "首页"
      },
      {
        "pagePath": "pages/yaku/yaku",
        "text": "番型表"
      },
      {
        "pagePath": "pages/score/score",
        "text": "点数表"
      },
      {
        "pagePath": "pages/camera/camera",
        "text": "识别"
      }
    ]
  },
  "style": "v2",
  "sitemapLocation": "sitemap.json"
}
```

`miniprogram/app.js`:
```javascript
App({
  globalData: {
    serverUrl: 'http://localhost:8080'
  }
})
```

`miniprogram/app.wxss`:
```css
page {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  background-color: #f5f5f5;
  color: #333;
}

.container {
  padding: 20rpx;
}
```

`miniprogram/project.config.json`:
```json
{
  "description": "MTiano - 日麻工具集合",
  "packOptions": {
    "ignore": [],
    "include": []
  },
  "setting": {
    "bundle": false,
    "userConfirmedBundleSwitch": false,
    "urlCheck": true,
    "scopeDataCheck": false,
    "coverView": true,
    "es6": true,
    "postcss": true,
    "compileHotReLoad": false,
    "lazyloadPlaceholderEnable": false,
    "preloadBackgroundData": false,
    "minified": true,
    "autoAudits": false,
    "newFeature": false,
    "uglifyFileName": false,
    "uploadWithSourceMap": true,
    "enhance": true,
    "useMultiFrameRuntime": true,
    "showShadowRootInWxmlPanel": true,
    "packNpmManually": false,
    "packNpmRelationList": []
  },
  "compileType": "miniprogram",
  "condition": {}
}
```

`miniprogram/sitemap.json`:
```json
{
  "desc": "关于本文件的更多信息，请参考文档",
  "rules": [
    {
      "action": "allow",
      "page": "*"
    }
  ]
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add miniprogram/
git commit -m "feat(miniprogram): scaffold WeChat Mini Program with app config"
```

---

## Task 10: Mini Program — Index Page

**Files:**
- Create: `miniprogram/pages/index/index.wxml`
- Create: `miniprogram/pages/index/index.wxss`
- Create: `miniprogram/pages/index/index.js`
- Create: `miniprogram/pages/index/index.json`

- [ ] **Step 1: Create index page**

`miniprogram/pages/index/index.json`:
```json
{
  "navigationBarTitleText": "MTiano - 日麻工具"
}
```

`miniprogram/pages/index/index.wxml`:
```xml
<view class="container">
  <view class="header">
    <text class="title">MTiano</text>
    <text class="subtitle">日麻工具集合</text>
  </view>

  <view class="menu">
    <navigator url="/pages/yaku/yaku" class="menu-item">
      <text class="menu-icon">📋</text>
      <view class="menu-text">
        <text class="menu-title">番型表</text>
        <text class="menu-desc">查询所有番型及条件</text>
      </view>
    </navigator>

    <navigator url="/pages/score/score" class="menu-item">
      <text class="menu-icon">🔢</text>
      <view class="menu-text">
        <text class="menu-title">点数表</text>
        <text class="menu-desc">番数×符数快速算点</text>
      </view>
    </navigator>

    <navigator url="/pages/camera/camera" class="menu-item">
      <text class="menu-icon">📷</text>
      <view class="menu-text">
        <text class="menu-title">拍照识别</text>
        <text class="menu-desc">拍照自动算番得分</text>
      </view>
    </navigator>
  </view>
</view>
```

`miniprogram/pages/index/index.wxss`:
```css
.container {
  padding: 40rpx;
}

.header {
  text-align: center;
  margin-bottom: 60rpx;
  padding: 60rpx 0;
}

.title {
  display: block;
  font-size: 56rpx;
  font-weight: bold;
  color: #1a1a2e;
}

.subtitle {
  display: block;
  font-size: 28rpx;
  color: #666;
  margin-top: 12rpx;
}

.menu {
  display: flex;
  flex-direction: column;
  gap: 24rpx;
}

.menu-item {
  display: flex;
  align-items: center;
  background: #fff;
  border-radius: 16rpx;
  padding: 32rpx;
  box-shadow: 0 2rpx 8rpx rgba(0,0,0,0.05);
}

.menu-icon {
  font-size: 48rpx;
  margin-right: 24rpx;
}

.menu-text {
  flex: 1;
}

.menu-title {
  display: block;
  font-size: 32rpx;
  font-weight: 500;
  color: #333;
}

.menu-desc {
  display: block;
  font-size: 24rpx;
  color: #999;
  margin-top: 4rpx;
}
```

`miniprogram/pages/index/index.js`:
```javascript
Page({
  data: {}
})
```

- [ ] **Step 2: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add miniprogram/pages/index/
git commit -m "feat(miniprogram): add index page with navigation menu"
```

---

## Task 11: Mini Program — Yaku Data & Page

**Files:**
- Create: `miniprogram/data/yaku.js`
- Create: `miniprogram/pages/yaku/yaku.wxml`
- Create: `miniprogram/pages/yaku/yaku.wxss`
- Create: `miniprogram/pages/yaku/yaku.js`
- Create: `miniprogram/pages/yaku/yaku.json`

- [ ] **Step 1: Create yaku static data**

`miniprogram/data/yaku.js`:
```javascript
module.exports = [
  // 1番
  { name: "立直", nameJp: "リーチ", han: 1, hanOpen: 0, category: "1番",
    description: "門前で聴牌時に宣言", condition: "門前限定" },
  { name: "一発", nameJp: "一発", han: 1, hanOpen: 0, category: "1番",
    description: "立直宣言後一巡以内に和了", condition: "門前限定" },
  { name: "門前清自摸和", nameJp: "ツモ", han: 1, hanOpen: 0, category: "1番",
    description: "門前でツモ和了", condition: "門前限定" },
  { name: "断么九", nameJp: "タンヤオ", han: 1, hanOpen: 1, category: "1番",
    description: "么九牌を含まない", condition: "" },
  { name: "平和", nameJp: "ピンフ", han: 1, hanOpen: 0, category: "1番",
    description: "4面子が全て順子、雀頭が役牌でない、両面待ち", condition: "門前限定" },
  { name: "一盃口", nameJp: "イーペーコー", han: 1, hanOpen: 0, category: "1番",
    description: "同種同数の順子が2組", condition: "門前限定" },
  { name: "役牌 白", nameJp: "ハク", han: 1, hanOpen: 1, category: "1番",
    description: "白の刻子/槓子", condition: "" },
  { name: "役牌 發", nameJp: "ハツ", han: 1, hanOpen: 1, category: "1番",
    description: "發の刻子/槓子", condition: "" },
  { name: "役牌 中", nameJp: "チュン", han: 1, hanOpen: 1, category: "1番",
    description: "中の刻子/槓子", condition: "" },
  { name: "場風牌", nameJp: "バカゼ", han: 1, hanOpen: 1, category: "1番",
    description: "場風の刻子/槓子", condition: "" },
  { name: "自風牌", nameJp: "ジカゼ", han: 1, hanOpen: 1, category: "1番",
    description: "自風の刻子/槓子", condition: "" },
  { name: "嶺上開花", nameJp: "リンシャンカイホウ", han: 1, hanOpen: 1, category: "1番",
    description: "槓した後の嶺上牌でツモ和了", condition: "" },
  { name: "搶槓", nameJp: "チャンカン", han: 1, hanOpen: 1, category: "1番",
    description: "他家の加槓した牌でロン和了", condition: "" },
  { name: "海底摸月", nameJp: "ハイテイ", han: 1, hanOpen: 1, category: "1番",
    description: "最後のツモ牌で和了", condition: "" },
  { name: "河底撈魚", nameJp: "ホウテイ", han: 1, hanOpen: 1, category: "1番",
    description: "最後の捨て牌でロン和了", condition: "" },
  // 2番
  { name: "七対子", nameJp: "チートイツ", han: 2, hanOpen: 0, category: "2番",
    description: "7組の対子", condition: "門前限定" },
  { name: "対々和", nameJp: "トイトイ", han: 2, hanOpen: 2, category: "2番",
    description: "4面子が全て刻子/槓子", condition: "" },
  { name: "三暗刻", nameJp: "サンアンコー", han: 2, hanOpen: 2, category: "2番",
    description: "暗刻が3組", condition: "" },
  { name: "三色同順", nameJp: "サンショクドウジュン", han: 2, hanOpen: 1, category: "2番",
    description: "3色で同じ数字の順子", condition: "" },
  { name: "一気通貫", nameJp: "イッツー", han: 2, hanOpen: 1, category: "2番",
    description: "同じ種類で123・456・789の順子", condition: "" },
  { name: "混全帯么九", nameJp: "チャンタ", han: 2, hanOpen: 1, category: "2番",
    description: "全ての面子と雀頭に么九牌を含む", condition: "" },
  { name: "三色同刻", nameJp: "サンショクドーコー", han: 2, hanOpen: 2, category: "2番",
    description: "3色で同じ数字の刻子", condition: "" },
  { name: "小三元", nameJp: "ショウサンゲン", han: 2, hanOpen: 2, category: "2番",
    description: "三元牌のうち2つが刻子、1つが雀頭", condition: "" },
  { name: "混老頭", nameJp: "ホンロウトウ", han: 2, hanOpen: 2, category: "2番",
    description: "么九牌のみで構成", condition: "" },
  { name: "ダブル立直", nameJp: "ダブルリーチ", han: 2, hanOpen: 0, category: "2番",
    description: "第一ツモで立直宣言", condition: "門前限定" },
  // 3番
  { name: "混一色", nameJp: "ホンイツ", han: 3, hanOpen: 2, category: "3番",
    description: "1種類の数牌と字牌のみ", condition: "" },
  { name: "純全帯么九", nameJp: "ジュンチャン", han: 3, hanOpen: 2, category: "3番",
    description: "全ての面子と雀頭に老頭牌を含む（字牌なし）", condition: "" },
  { name: "二盃口", nameJp: "リャンペーコー", han: 3, hanOpen: 0, category: "3番",
    description: "一盃口が2組", condition: "門前限定" },
  // 6番
  { name: "清一色", nameJp: "チンイツ", han: 6, hanOpen: 5, category: "6番",
    description: "1種類の数牌のみ", condition: "" },
  // 役満
  { name: "国士無双", nameJp: "コクシムソウ", han: 13, hanOpen: 0, category: "役満",
    description: "13種の么九牌を1枚ずつ+1枚", condition: "門前限定" },
  { name: "四暗刻", nameJp: "スーアンコー", han: 13, hanOpen: 0, category: "役満",
    description: "暗刻が4組", condition: "門前限定" },
  { name: "大三元", nameJp: "ダイサンゲン", han: 13, hanOpen: 13, category: "役満",
    description: "三元牌が全て刻子/槓子", condition: "" },
  { name: "字一色", nameJp: "ツーイーソー", han: 13, hanOpen: 13, category: "役満",
    description: "字牌のみで構成", condition: "" },
  { name: "緑一色", nameJp: "リューイーソー", han: 13, hanOpen: 13, category: "役満",
    description: "2s3s4s6s8s發のみで構成", condition: "" },
  { name: "清老頭", nameJp: "チンロウトウ", han: 13, hanOpen: 13, category: "役満",
    description: "老頭牌のみで構成", condition: "" },
  { name: "四喜和", nameJp: "スーシーホー", han: 13, hanOpen: 13, category: "役満",
    description: "風牌4種が全て面子/雀頭", condition: "" },
  { name: "九蓮宝燈", nameJp: "チューレンポートー", han: 13, hanOpen: 0, category: "役満",
    description: "1112345678999+任意1枚（同種）", condition: "門前限定" },
  { name: "天和", nameJp: "テンホー", han: 13, hanOpen: 0, category: "役満",
    description: "親の配牌で和了", condition: "門前限定" },
  { name: "地和", nameJp: "チーホー", han: 13, hanOpen: 0, category: "役満",
    description: "子の第一ツモで和了", condition: "門前限定" }
]
```

- [ ] **Step 2: Create yaku page**

`miniprogram/pages/yaku/yaku.json`:
```json
{
  "navigationBarTitleText": "番型表"
}
```

`miniprogram/pages/yaku/yaku.wxml`:
```xml
<view class="container">
  <view class="search-bar">
    <input class="search-input" placeholder="搜索番型..." bindinput="onSearch" value="{{searchText}}" />
  </view>

  <view class="category-tabs">
    <view class="tab {{activeCategory === '' ? 'active' : ''}}" bindtap="onTabAll">全部</view>
    <view class="tab {{activeCategory === item ? 'active' : ''}}"
      wx:for="{{categories}}" wx:key="*this"
      data-category="{{item}}" bindtap="onTabChange">
      {{item}}
    </view>
  </view>

  <view class="yaku-list">
    <view class="yaku-card" wx:for="{{filteredYaku}}" wx:key="name">
      <view class="yaku-header">
        <text class="yaku-name">{{item.name}}</text>
        <text class="yaku-name-jp">{{item.nameJp}}</text>
        <view class="yaku-han">
          <text class="han-badge">{{item.han}}番</text>
          <text class="han-open" wx:if="{{item.hanOpen > 0}}">食下{{item.hanOpen}}番</text>
          <text class="han-open closed-only" wx:if="{{item.hanOpen === 0}}">門前限定</text>
        </view>
      </view>
      <text class="yaku-desc">{{item.description}}</text>
    </view>
  </view>
</view>
```

`miniprogram/pages/yaku/yaku.wxss`:
```css
.container {
  padding: 20rpx;
}

.search-bar {
  margin-bottom: 20rpx;
}

.search-input {
  background: #fff;
  border-radius: 12rpx;
  padding: 16rpx 24rpx;
  font-size: 28rpx;
  border: 1rpx solid #eee;
}

.category-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 12rpx;
  margin-bottom: 24rpx;
}

.tab {
  padding: 8rpx 20rpx;
  border-radius: 8rpx;
  font-size: 24rpx;
  background: #fff;
  color: #666;
  border: 1rpx solid #eee;
}

.tab.active {
  background: #1a1a2e;
  color: #fff;
  border-color: #1a1a2e;
}

.yaku-list {
  display: flex;
  flex-direction: column;
  gap: 16rpx;
}

.yaku-card {
  background: #fff;
  border-radius: 12rpx;
  padding: 24rpx;
  box-shadow: 0 2rpx 8rpx rgba(0,0,0,0.04);
}

.yaku-header {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12rpx;
  margin-bottom: 8rpx;
}

.yaku-name {
  font-size: 30rpx;
  font-weight: 600;
  color: #333;
}

.yaku-name-jp {
  font-size: 24rpx;
  color: #999;
}

.yaku-han {
  margin-left: auto;
  display: flex;
  gap: 8rpx;
  align-items: center;
}

.han-badge {
  background: #e74c3c;
  color: #fff;
  font-size: 22rpx;
  padding: 4rpx 12rpx;
  border-radius: 6rpx;
}

.han-open {
  font-size: 22rpx;
  color: #666;
}

.closed-only {
  color: #3498db;
}

.yaku-desc {
  font-size: 26rpx;
  color: #666;
  line-height: 1.5;
}
```

`miniprogram/pages/yaku/yaku.js`:
```javascript
const yakuData = require('../../data/yaku.js')

Page({
  data: {
    allYaku: yakuData,
    filteredYaku: yakuData,
    categories: ['1番', '2番', '3番', '6番', '役満'],
    activeCategory: '',
    searchText: ''
  },

  onSearch(e) {
    const text = e.detail.value.trim().toLowerCase()
    this.setData({ searchText: text })
    this.filterYaku()
  },

  onTabAll() {
    this.setData({ activeCategory: '' })
    this.filterYaku()
  },

  onTabChange(e) {
    const category = e.currentTarget.dataset.category
    this.setData({ activeCategory: category })
    this.filterYaku()
  },

  filterYaku() {
    const { allYaku, activeCategory, searchText } = this.data
    let filtered = allYaku

    if (activeCategory) {
      filtered = filtered.filter(y => y.category === activeCategory)
    }
    if (searchText) {
      filtered = filtered.filter(y =>
        y.name.toLowerCase().includes(searchText) ||
        y.nameJp.toLowerCase().includes(searchText) ||
        y.description.toLowerCase().includes(searchText)
      )
    }
    this.setData({ filteredYaku: filtered })
  }
})
```

- [ ] **Step 3: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add miniprogram/data/yaku.js miniprogram/pages/yaku/
git commit -m "feat(miniprogram): add yaku lookup page with search and filtering"
```

---

## Task 12: Mini Program — Score Table Page

**Files:**
- Create: `miniprogram/data/score.js`
- Create: `miniprogram/pages/score/score.wxml`
- Create: `miniprogram/pages/score/score.wxss`
- Create: `miniprogram/pages/score/score.js`
- Create: `miniprogram/pages/score/score.json`

- [ ] **Step 1: Create score lookup data**

`miniprogram/data/score.js`:
```javascript
// Score table: ron points for child (non-dealer)
// Format: scoreTable[han][fu] = { childRon, parentRon, childTsumo: [fromParent, fromChild], parentTsumo }
const scoreTable = {
  1: {
    30: { childRon: 1000, parentRon: 1500, childTsumo: [500, 300], parentTsumo: 500 },
    40: { childRon: 1300, parentRon: 2000, childTsumo: [700, 400], parentTsumo: 700 },
    50: { childRon: 1600, parentRon: 2400, childTsumo: [800, 400], parentTsumo: 800 },
    60: { childRon: 2000, parentRon: 2900, childTsumo: [1000, 500], parentTsumo: 1000 },
    70: { childRon: 2300, parentRon: 3400, childTsumo: [1200, 600], parentTsumo: 1200 },
    80: { childRon: 2600, parentRon: 3900, childTsumo: [1300, 700], parentTsumo: 1300 },
    90: { childRon: 2900, parentRon: 4400, childTsumo: [1500, 800], parentTsumo: 1500 },
    100: { childRon: 3200, parentRon: 4800, childTsumo: [1600, 800], parentTsumo: 1600 },
    110: { childRon: 3600, parentRon: 5300, childTsumo: [1800, 900], parentTsumo: 1800 }
  },
  2: {
    25: { childRon: 1600, parentRon: 2400, childTsumo: [800, 400], parentTsumo: 800 },
    30: { childRon: 2000, parentRon: 2900, childTsumo: [1000, 500], parentTsumo: 1000 },
    40: { childRon: 2600, parentRon: 3900, childTsumo: [1300, 700], parentTsumo: 1300 },
    50: { childRon: 3200, parentRon: 4800, childTsumo: [1600, 800], parentTsumo: 1600 },
    60: { childRon: 3900, parentRon: 5800, childTsumo: [2000, 1000], parentTsumo: 2000 },
    70: { childRon: 4500, parentRon: 6800, childTsumo: [2300, 1200], parentTsumo: 2300 },
    80: { childRon: 5200, parentRon: 7700, childTsumo: [2600, 1300], parentTsumo: 2600 },
    90: { childRon: 5800, parentRon: 8700, childTsumo: [2900, 1500], parentTsumo: 2900 },
    100: { childRon: 6400, parentRon: 9600, childTsumo: [3200, 1600], parentTsumo: 3200 },
    110: { childRon: 7100, parentRon: 10600, childTsumo: [3600, 1800], parentTsumo: 3600 }
  },
  3: {
    25: { childRon: 3200, parentRon: 4800, childTsumo: [1600, 800], parentTsumo: 1600 },
    30: { childRon: 3900, parentRon: 5800, childTsumo: [2000, 1000], parentTsumo: 2000 },
    40: { childRon: 5200, parentRon: 7700, childTsumo: [2600, 1300], parentTsumo: 2600 },
    50: { childRon: 6400, parentRon: 9600, childTsumo: [3200, 1600], parentTsumo: 3200 },
    60: { childRon: 7700, parentRon: 11600, childTsumo: [3900, 2000], parentTsumo: 3900 }
  },
  4: {
    25: { childRon: 6400, parentRon: 9600, childTsumo: [3200, 1600], parentTsumo: 3200 },
    30: { childRon: 7700, parentRon: 11600, childTsumo: [3900, 2000], parentTsumo: 3900 }
  },
  5: { childRon: 8000, parentRon: 12000, childTsumo: [4000, 2000], parentTsumo: 4000, name: "満貫" },
  6: { childRon: 12000, parentRon: 18000, childTsumo: [6000, 3000], parentTsumo: 6000, name: "跳満" },
  7: { childRon: 12000, parentRon: 18000, childTsumo: [6000, 3000], parentTsumo: 6000, name: "跳満" },
  8: { childRon: 16000, parentRon: 24000, childTsumo: [8000, 4000], parentTsumo: 8000, name: "倍満" },
  9: { childRon: 16000, parentRon: 24000, childTsumo: [8000, 4000], parentTsumo: 8000, name: "倍満" },
  10: { childRon: 16000, parentRon: 24000, childTsumo: [8000, 4000], parentTsumo: 8000, name: "倍満" },
  11: { childRon: 24000, parentRon: 36000, childTsumo: [12000, 6000], parentTsumo: 12000, name: "三倍満" },
  12: { childRon: 24000, parentRon: 36000, childTsumo: [12000, 6000], parentTsumo: 12000, name: "三倍満" },
  13: { childRon: 32000, parentRon: 48000, childTsumo: [16000, 8000], parentTsumo: 16000, name: "役満" }
}

module.exports = scoreTable
```

- [ ] **Step 2: Create score page**

`miniprogram/pages/score/score.json`:
```json
{
  "navigationBarTitleText": "点数表"
}
```

`miniprogram/pages/score/score.wxml`:
```xml
<view class="container">
  <view class="selector">
    <view class="selector-row">
      <text class="label">番数:</text>
      <picker mode="selector" range="{{hanOptions}}" value="{{hanIndex}}" bindchange="onHanChange">
        <view class="picker">{{hanOptions[hanIndex]}}番</view>
      </picker>
    </view>
    <view class="selector-row" wx:if="{{showFu}}">
      <text class="label">符数:</text>
      <picker mode="selector" range="{{fuOptions}}" value="{{fuIndex}}" bindchange="onFuChange">
        <view class="picker">{{fuOptions[fuIndex]}}符</view>
      </picker>
    </view>
    <view class="selector-row">
      <text class="label">庄家:</text>
      <switch checked="{{isParent}}" bindchange="onParentChange" color="#1a1a2e" />
    </view>
  </view>

  <view class="result" wx:if="{{score}}">
    <view class="score-name" wx:if="{{score.name}}">{{score.name}}</view>

    <view class="score-section">
      <text class="section-title">荣和 (ロン)</text>
      <text class="score-value">{{isParent ? score.parentRon : score.childRon}} 点</text>
    </view>

    <view class="score-section">
      <text class="section-title">自摸 (ツモ)</text>
      <block wx:if="{{isParent}}">
        <text class="score-value">各家 {{score.parentTsumo}} 点</text>
      </block>
      <block wx:else>
        <text class="score-value">亲家 {{score.childTsumo[0]}} / 子家 {{score.childTsumo[1]}} 点</text>
      </block>
    </view>
  </view>
</view>
```

`miniprogram/pages/score/score.wxss`:
```css
.container {
  padding: 30rpx;
}

.selector {
  background: #fff;
  border-radius: 12rpx;
  padding: 24rpx;
  margin-bottom: 30rpx;
}

.selector-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16rpx 0;
  border-bottom: 1rpx solid #f0f0f0;
}

.selector-row:last-child {
  border-bottom: none;
}

.label {
  font-size: 30rpx;
  color: #333;
}

.picker {
  font-size: 30rpx;
  color: #1a1a2e;
  font-weight: 500;
  padding: 8rpx 20rpx;
  background: #f5f5f5;
  border-radius: 8rpx;
}

.result {
  background: #fff;
  border-radius: 12rpx;
  padding: 32rpx;
}

.score-name {
  text-align: center;
  font-size: 36rpx;
  font-weight: bold;
  color: #e74c3c;
  margin-bottom: 24rpx;
}

.score-section {
  padding: 16rpx 0;
  border-bottom: 1rpx solid #f0f0f0;
}

.score-section:last-child {
  border-bottom: none;
}

.section-title {
  display: block;
  font-size: 26rpx;
  color: #999;
  margin-bottom: 8rpx;
}

.score-value {
  display: block;
  font-size: 36rpx;
  font-weight: 600;
  color: #333;
}
```

`miniprogram/pages/score/score.js`:
```javascript
const scoreTable = require('../../data/score.js')

Page({
  data: {
    hanOptions: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13],
    fuOptions: [20, 25, 30, 40, 50, 60, 70, 80, 90, 100, 110],
    hanIndex: 0,
    fuIndex: 2,
    isParent: false,
    showFu: true,
    score: null
  },

  onLoad() {
    this.calculateScore()
  },

  onHanChange(e) {
    const hanIndex = parseInt(e.detail.value)
    const han = this.data.hanOptions[hanIndex]
    this.setData({
      hanIndex,
      showFu: han <= 4
    })
    this.calculateScore()
  },

  onFuChange(e) {
    this.setData({ fuIndex: parseInt(e.detail.value) })
    this.calculateScore()
  },

  onParentChange(e) {
    this.setData({ isParent: e.detail.value })
    this.calculateScore()
  },

  calculateScore() {
    const han = this.data.hanOptions[this.data.hanIndex]
    const fu = this.data.fuOptions[this.data.fuIndex]

    const hanEntry = scoreTable[han]
    if (!hanEntry) {
      this.setData({ score: null })
      return
    }

    let score
    if (han >= 5) {
      score = hanEntry
    } else {
      score = hanEntry[fu]
    }

    this.setData({ score: score || null })
  }
})
```

- [ ] **Step 3: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add miniprogram/data/score.js miniprogram/pages/score/
git commit -m "feat(miniprogram): add score lookup page with han/fu selector"
```

---

## Task 13: Mini Program — Camera Page

**Files:**
- Create: `miniprogram/pages/camera/camera.wxml`
- Create: `miniprogram/pages/camera/camera.wxss`
- Create: `miniprogram/pages/camera/camera.js`
- Create: `miniprogram/pages/camera/camera.json`

- [ ] **Step 1: Create camera page**

`miniprogram/pages/camera/camera.json`:
```json
{
  "navigationBarTitleText": "拍照识别"
}
```

`miniprogram/pages/camera/camera.wxml`:
```xml
<view class="container">
  <view class="upload-area" wx:if="{{!imagePath}}">
    <view class="upload-btn" bindtap="chooseImage">
      <text class="upload-icon">📷</text>
      <text class="upload-text">拍照或选择图片</text>
      <text class="upload-hint">将手牌拍照上传，自动识别并计算番型</text>
    </view>
  </view>

  <view class="preview" wx:if="{{imagePath}}">
    <image class="preview-image" src="{{imagePath}}" mode="aspectFit" />
    <view class="action-bar">
      <button class="btn btn-secondary" bindtap="chooseImage">重新选择</button>
      <button class="btn btn-primary" bindtap="recognize" loading="{{loading}}">开始识别</button>
    </view>
  </view>

  <view class="result" wx:if="{{result}}">
    <view class="result-section">
      <text class="section-title">识别结果</text>
      <view class="tiles-display">
        <text class="tile-item" wx:for="{{result.tiles}}" wx:key="*this">{{item}}</text>
      </view>
    </view>

    <view class="result-section" wx:if="{{result.yaku.length > 0}}">
      <text class="section-title">番型</text>
      <view class="yaku-item" wx:for="{{result.yaku}}" wx:key="name">
        <text>{{item.name}}</text>
        <text class="yaku-han">{{item.han}}番</text>
      </view>
    </view>

    <view class="result-section">
      <text class="section-title">得分</text>
      <text class="total-score">{{result.total_han}}番 — {{result.score.RonTotal}}点</text>
    </view>
  </view>

  <view class="error" wx:if="{{error}}">
    <text>{{error}}</text>
  </view>
</view>
```

`miniprogram/pages/camera/camera.wxss`:
```css
.container {
  padding: 30rpx;
}

.upload-area {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 500rpx;
}

.upload-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 400rpx;
  background: #fff;
  border: 2rpx dashed #ccc;
  border-radius: 16rpx;
}

.upload-icon {
  font-size: 80rpx;
  margin-bottom: 16rpx;
}

.upload-text {
  font-size: 32rpx;
  color: #333;
  margin-bottom: 8rpx;
}

.upload-hint {
  font-size: 24rpx;
  color: #999;
}

.preview {
  margin-bottom: 30rpx;
}

.preview-image {
  width: 100%;
  height: 400rpx;
  border-radius: 12rpx;
  background: #eee;
}

.action-bar {
  display: flex;
  gap: 20rpx;
  margin-top: 20rpx;
}

.btn {
  flex: 1;
  border-radius: 12rpx;
  font-size: 28rpx;
  padding: 20rpx 0;
}

.btn-primary {
  background: #1a1a2e;
  color: #fff;
}

.btn-secondary {
  background: #f5f5f5;
  color: #333;
}

.result {
  background: #fff;
  border-radius: 12rpx;
  padding: 24rpx;
}

.result-section {
  padding: 16rpx 0;
  border-bottom: 1rpx solid #f0f0f0;
}

.result-section:last-child {
  border-bottom: none;
}

.section-title {
  display: block;
  font-size: 26rpx;
  color: #999;
  margin-bottom: 12rpx;
}

.tiles-display {
  display: flex;
  flex-wrap: wrap;
  gap: 8rpx;
}

.tile-item {
  background: #f5f5f5;
  padding: 8rpx 16rpx;
  border-radius: 6rpx;
  font-size: 28rpx;
  font-weight: 500;
}

.yaku-item {
  display: flex;
  justify-content: space-between;
  padding: 8rpx 0;
}

.yaku-han {
  color: #e74c3c;
  font-weight: 500;
}

.total-score {
  font-size: 36rpx;
  font-weight: bold;
  color: #1a1a2e;
}

.error {
  background: #fee;
  border-radius: 12rpx;
  padding: 24rpx;
  margin-top: 20rpx;
  color: #e74c3c;
  text-align: center;
}
```

`miniprogram/pages/camera/camera.js`:
```javascript
const app = getApp()

Page({
  data: {
    imagePath: '',
    loading: false,
    result: null,
    error: ''
  },

  chooseImage() {
    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      sourceType: ['album', 'camera'],
      success: (res) => {
        this.setData({
          imagePath: res.tempFiles[0].tempFilePath,
          result: null,
          error: ''
        })
      }
    })
  },

  recognize() {
    if (!this.data.imagePath) return

    this.setData({ loading: true, error: '' })

    wx.uploadFile({
      url: app.globalData.serverUrl + '/api/v1/recognize',
      filePath: this.data.imagePath,
      name: 'image',
      success: (res) => {
        try {
          const data = JSON.parse(res.data)
          if (res.statusCode === 200) {
            this.setData({ result: data })
          } else {
            this.setData({ error: data.error || '识别失败' })
          }
        } catch (e) {
          this.setData({ error: '解析响应失败' })
        }
      },
      fail: () => {
        this.setData({ error: '网络请求失败，请检查后端服务是否启动' })
      },
      complete: () => {
        this.setData({ loading: false })
      }
    })
  }
})
```

- [ ] **Step 2: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add miniprogram/pages/camera/
git commit -m "feat(miniprogram): add camera page for photo recognition"
```

---

## Task 14: CLAUDE.md & Final Integration

**Files:**
- Create: `CLAUDE.md`
- Create: `.gitignore`

- [ ] **Step 1: Create .gitignore**

`.gitignore`:
```
# Go
server/server
server/tmp/
*.exe

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
Thumbs.db

# Mini Program
miniprogram/node_modules/
miniprogram/miniprogram_npm/

# Config secrets
server/configs/config.local.yaml
```

- [ ] **Step 2: Create CLAUDE.md**

`CLAUDE.md`:
```markdown
# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MTiano is a Japanese Mahjong (日麻) tool collection WeChat Mini Program with a Go backend. It provides yaku/score table lookup and photo-based tile recognition with automatic scoring.

## Architecture

- **miniprogram/** — Native WeChat Mini Program frontend. Static yaku/score data is embedded locally for instant lookup. Camera page uploads images to the backend.
- **server/** — Go backend (Gin). Handles image recognition via AI vision service interface and scoring via the mahjong engine in `pkg/mahjong/`.
- Static data lives in the Mini Program (`miniprogram/data/`); the backend does NOT serve lookup data.

## Build & Run

### Go Backend

```bash
cd server
go run cmd/server/main.go          # run server (reads configs/config.yaml)
go build ./cmd/server/              # build binary
go test ./...                       # run all tests
go test ./pkg/mahjong/ -v           # run mahjong engine tests only
go test ./pkg/mahjong/ -run TestJudge -v  # run specific test
```

### Mini Program

Open `miniprogram/` directory in WeChat Developer Tools (微信开发者工具). No build step required for native mini programs.

## Key Modules

- `server/pkg/mahjong/` — Core mahjong logic (tile model, hand parsing, yaku judgment, score calculation). This package is independent and fully unit-tested.
- `server/internal/service/vision.go` — VisionService interface for AI tile recognition. Swap implementations by adding new files that implement the interface.
- `server/internal/handler/` — HTTP handlers. The recognize endpoint receives an image, calls vision service, runs the judge, and returns results.

## Tile Notation

Tiles use compact string notation: `1m`=一万, `5p`=五筒, `3s`=三索, `1z`=東, `5z`=白, `6z`=發, `7z`=中. `ParseTiles("123m456p789s11z")` parses this format.

## Adding New Yaku

1. Add constant to `server/pkg/mahjong/yaku.go` (YakuType enum + yakuInfo map)
2. Add judgment logic in `server/pkg/mahjong/judge.go` (inside `judgeDecomposition`)
3. Add test case in `judge_test.go`
4. Add entry to `miniprogram/data/yaku.js` for frontend display

## Adding a New AI Vision Provider

1. Create `server/internal/service/vision_<provider>.go` implementing `VisionService`
2. Add provider case in `server/cmd/server/main.go` switch statement
3. Update `configs/config.yaml` with provider-specific settings
```

- [ ] **Step 3: Commit**

```bash
cd /Users/buchiyudan/Documents/mtiano
git add CLAUDE.md .gitignore
git commit -m "docs: add CLAUDE.md and .gitignore"
```

---

Plan complete and saved to `docs/superpowers/plans/2026-05-01-mtiano-mahjong-tool.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
