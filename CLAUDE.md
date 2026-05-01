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
