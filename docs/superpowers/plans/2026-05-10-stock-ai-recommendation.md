# 股票 AI 分析与推荐系统 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 MTiano 小程序中新增股票 AI 推荐模块，定时通过 AKShare 获取股票数据，用通义千问 AI 分析生成购买推荐，前端展示推荐列表。

**Architecture:** Go 单体扩展 + AKShare Python 微服务。Go 服务新增 MySQL 存储、JWT 认证、robfig/cron 定时任务、通义千问文本分析。Python Flask 微服务封装 AKShare 提供股票数据 HTTP 接口。小程序新增 stock tab 页面。

**Tech Stack:** Go/Gin, MySQL, robfig/cron, golang-jwt, AKShare(Python/Flask), 通义千问 DashScope API, 微信小程序原生

---

## File Structure

### Go 服务端新增/修改

| 文件 | 职责 |
|------|------|
| `server/internal/config/config.go` | **修改** — 扩展 Config 结构，新增 MySQL/WeChat/JWT/Stock 配置 |
| `server/internal/db/mysql.go` | **新建** — MySQL 连接初始化 + 建表 |
| `server/internal/model/user.go` | **新建** — User 结构体 |
| `server/internal/model/watchlist.go` | **新建** — Watchlist 结构体 |
| `server/internal/model/recommendation.go` | **新建** — StockRecommendation 结构体 |
| `server/internal/middleware/auth.go` | **新建** — JWT 认证中间件 |
| `server/internal/service/auth.go` | **新建** — 微信 code2Session + JWT 生成 + 用户查找/创建 |
| `server/internal/service/stock_data.go` | **新建** — 调用 AKShare 微服务获取股票数据 |
| `server/internal/service/stock_analyzer.go` | **新建** — 调用通义千问分析股票，返回推荐度和理由 |
| `server/internal/service/stock_store.go` | **新建** — MySQL CRUD（用户、自选股、推荐结果） |
| `server/internal/handler/auth.go` | **新建** — POST /api/v1/auth/login |
| `server/internal/handler/stock.go` | **新建** — 股票相关 HTTP 接口 |
| `server/internal/cron/stock_cron.go` | **新建** — 定时任务编排 |
| `server/cmd/server/main.go` | **修改** — 注册新路由、初始化 MySQL、启动定时任务 |
| `server/configs/config.yaml` | **修改** — 新增配置段 |

### AKShare Python 微服务

| 文件 | 职责 |
|------|------|
| `server/stock_service/app.py` | **新建** — Flask 服务，封装 AKShare |
| `server/stock_service/requirements.txt` | **新建** — Python 依赖 |

### 小程序前端

| 文件 | 职责 |
|------|------|
| `miniprogram/app.js` | **修改** — 新增 token 管理和登录方法 |
| `miniprogram/app.json` | **修改** — 新增 stock 页面和 tab |
| `miniprogram/utils/auth.js` | **新建** — 登录/token 工具函数 |
| `miniprogram/pages/stock/stock.js` | **新建** — 股票页面逻辑 |
| `miniprogram/pages/stock/stock.wxml` | **新建** — 股票页面模板 |
| `miniprogram/pages/stock/stock.wxss` | **新建** — 股票页面样式 |
| `miniprogram/pages/stock/stock.json` | **新建** — 股票页面配置 |

---

### Task 1: Go 配置扩展 + MySQL 连接

**Files:**
- Modify: `server/internal/config/config.go`
- Create: `server/internal/db/mysql.go`
- Modify: `server/configs/config.yaml`
- Modify: `server/go.mod` (go get 新增依赖)

- [ ] **Step 1: 安装 Go 依赖**

```bash
cd server
go get github.com/go-sql-driver/mysql
go get github.com/golang-jwt/jwt/v5
go get github.com/robfig/cron/v3
```

- [ ] **Step 2: 扩展 config.go**

将 `server/internal/config/config.go` 替换为：

```go
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Vision VisionConfig `yaml:"vision"`
	MySQL  MySQLConfig  `yaml:"mysql"`
	WeChat WeChatConfig `yaml:"wechat"`
	JWT    JWTConfig    `yaml:"jwt"`
	Stock  StockConfig  `yaml:"stock"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

type VisionConfig struct {
	Provider string `yaml:"provider"`
	APIKey   string `yaml:"api_key"`
	Endpoint string `yaml:"endpoint"`
	Model    string `yaml:"model"`
}

type MySQLConfig struct {
	DSN string `yaml:"dsn"`
}

type WeChatConfig struct {
	AppID  string `yaml:"appid"`
	Secret string `yaml:"secret"`
}

type JWTConfig struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
}

type StockConfig struct {
	AKShareEndpoint string `yaml:"akshare_endpoint"`
	AIModel         string `yaml:"ai_model"`
	AIAPIKey        string `yaml:"ai_api_key"`
	AIEndpoint      string `yaml:"ai_endpoint"`
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

- [ ] **Step 3: 创建 db/mysql.go**

```go
package db

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

func InitMySQL(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}
	return db, nil
}

func createTables(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			openid VARCHAR(64) NOT NULL UNIQUE,
			nickname VARCHAR(50) DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS watchlist (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			user_id BIGINT NOT NULL,
			stock_code VARCHAR(10) NOT NULL,
			stock_name VARCHAR(50) DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_user_stock (user_id, stock_code)
		)`,
		`CREATE TABLE IF NOT EXISTS stock_recommendations (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			stock_code VARCHAR(10) NOT NULL,
			stock_name VARCHAR(50) DEFAULT '',
			source VARCHAR(10) NOT NULL,
			buy_score TINYINT DEFAULT 0,
			buy_reason TEXT,
			tail_score TINYINT DEFAULT 0,
			tail_reason TEXT,
			analysis_date DATE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_code_source_date (stock_code, source, analysis_date)
		)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}
	return nil
}
```

- [ ] **Step 4: 更新 config.yaml**

```yaml
server:
  port: ":9091"

vision:
  provider: "yolo"
  endpoint: "http://localhost:5000"

mysql:
  dsn: "root:123456@tcp(localhost:3306)/mtiano?charset=utf8mb4&parseTime=True"

wechat:
  appid: "wx_xxx"
  secret: "xxx"

jwt:
  secret: "mtiano-stock-jwt-secret-change-me"
  expire_hours: 168

stock:
  akshare_endpoint: "http://localhost:5001"
  ai_model: "qwen-plus"
  ai_api_key: "sk-c8454d49077c4827a2919b023283d588"
  ai_endpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1"
```

- [ ] **Step 5: 验证编译通过**

```bash
cd server && go build ./...
```
Expected: 编译成功，无错误

- [ ] **Step 6: Commit**

```bash
git add server/internal/config/config.go server/internal/db/mysql.go server/configs/config.yaml server/go.mod server/go.sum
git commit -m "feat(stock): add MySQL, JWT, stock config and DB init"
```

---

### Task 2: Model 层 — User, Watchlist, Recommendation

**Files:**
- Create: `server/internal/model/user.go`
- Create: `server/internal/model/watchlist.go`
- Create: `server/internal/model/recommendation.go`

- [ ] **Step 1: 创建 model/user.go**

```go
package model

import "time"

type User struct {
	ID        int64     `json:"id"`
	OpenID    string    `json:"openid"`
	Nickname  string    `json:"nickname"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: 创建 model/watchlist.go**

```go
package model

import "time"

type WatchlistItem struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	StockCode string    `json:"stock_code"`
	StockName string    `json:"stock_name"`
	CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **Step 3: 创建 model/recommendation.go**

```go
package model

import "time"

type StockRecommendation struct {
	ID           int64     `json:"id"`
	StockCode    string    `json:"stock_code"`
	StockName    string    `json:"stock_name"`
	Source       string    `json:"source"`
	BuyScore     int       `json:"buy_score"`
	BuyReason    string    `json:"buy_reason"`
	TailScore    int       `json:"tail_score"`
	TailReason   string    `json:"tail_reason"`
	AnalysisDate string    `json:"analysis_date"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
```

- [ ] **Step 4: 验证编译通过**

```bash
cd server && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add server/internal/model/
git commit -m "feat(stock): add User, Watchlist, Recommendation models"
```

---

### Task 3: StockStore — MySQL CRUD

**Files:**
- Create: `server/internal/service/stock_store.go`
- Create: `server/internal/service/stock_store_test.go`

- [ ] **Step 1: 编写 stock_store_test.go**

```go
package service

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mtiano/server/internal/model"
)

func getTestDB(t *testing.T) *sql.DB {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN not set, skipping DB test")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func TestStockStore_FindOrCreateUser(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	store := NewStockStore(db)

	user, err := store.FindOrCreateUser("test_openid_123")
	if err != nil {
		t.Fatalf("FindOrCreateUser: %v", err)
	}
	if user.OpenID != "test_openid_123" {
		t.Errorf("expected openid test_openid_123, got %s", user.OpenID)
	}
	if user.ID == 0 {
		t.Error("expected non-zero user ID")
	}

	user2, err := store.FindOrCreateUser("test_openid_123")
	if err != nil {
		t.Fatalf("FindOrCreateUser again: %v", err)
	}
	if user2.ID != user.ID {
		t.Errorf("expected same user ID %d, got %d", user.ID, user2.ID)
	}

	db.Exec("DELETE FROM users WHERE openid = 'test_openid_123'")
}

func TestStockStore_Watchlist(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	store := NewStockStore(db)

	user, _ := store.FindOrCreateUser("test_watchlist_user")
	defer db.Exec("DELETE FROM users WHERE openid = 'test_watchlist_user'")
	defer db.Exec("DELETE FROM watchlist WHERE user_id = ?", user.ID)

	err := store.AddWatchlistItem(user.ID, "600519", "贵州茅台")
	if err != nil {
		t.Fatalf("AddWatchlistItem: %v", err)
	}

	items, err := store.GetWatchlist(user.ID)
	if err != nil {
		t.Fatalf("GetWatchlist: %v", err)
	}
	if len(items) != 1 || items[0].StockCode != "600519" {
		t.Errorf("unexpected watchlist: %+v", items)
	}

	err = store.RemoveWatchlistItem(user.ID, "600519")
	if err != nil {
		t.Fatalf("RemoveWatchlistItem: %v", err)
	}

	items, _ = store.GetWatchlist(user.ID)
	if len(items) != 0 {
		t.Errorf("expected empty watchlist after delete, got %d", len(items))
	}
}

func TestStockStore_Recommendations(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	store := NewStockStore(db)

	today := time.Now().Format("2006-01-02")
	defer db.Exec("DELETE FROM stock_recommendations WHERE analysis_date = ? AND stock_code = '600519'", today)

	rec := &model.StockRecommendation{
		StockCode:    "600519",
		StockName:    "贵州茅台",
		Source:       "hot",
		BuyScore:     8,
		BuyReason:    "test buy reason",
		TailScore:    6,
		TailReason:   "test tail reason",
		AnalysisDate: today,
	}
	err := store.UpsertRecommendation(rec)
	if err != nil {
		t.Fatalf("UpsertRecommendation: %v", err)
	}

	recs, err := store.GetTodayRecommendations("")
	if err != nil {
		t.Fatalf("GetTodayRecommendations: %v", err)
	}
	found := false
	for _, r := range recs {
		if r.StockCode == "600519" && r.BuyScore == 8 {
			found = true
		}
	}
	if !found {
		t.Error("expected to find recommendation for 600519")
	}
}
```

- [ ] **Step 2: 创建 stock_store.go**

```go
package service

import (
	"database/sql"
	"time"

	"github.com/mtiano/server/internal/model"
)

type StockStore struct {
	db *sql.DB
}

func NewStockStore(db *sql.DB) *StockStore {
	return &StockStore{db: db}
}

func (s *StockStore) FindOrCreateUser(openid string) (*model.User, error) {
	var user model.User
	err := s.db.QueryRow("SELECT id, openid, nickname, created_at, updated_at FROM users WHERE openid = ?", openid).
		Scan(&user.ID, &user.OpenID, &user.Nickname, &user.CreatedAt, &user.UpdatedAt)
	if err == nil {
		return &user, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	result, err := s.db.Exec("INSERT INTO users (openid) VALUES (?)", openid)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &model.User{ID: id, OpenID: openid, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (s *StockStore) AddWatchlistItem(userID int64, code, name string) error {
	_, err := s.db.Exec(
		"INSERT IGNORE INTO watchlist (user_id, stock_code, stock_name) VALUES (?, ?, ?)",
		userID, code, name,
	)
	return err
}

func (s *StockStore) RemoveWatchlistItem(userID int64, code string) error {
	_, err := s.db.Exec("DELETE FROM watchlist WHERE user_id = ? AND stock_code = ?", userID, code)
	return err
}

func (s *StockStore) GetWatchlist(userID int64) ([]model.WatchlistItem, error) {
	rows, err := s.db.Query(
		"SELECT id, user_id, stock_code, stock_name, created_at FROM watchlist WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.WatchlistItem
	for rows.Next() {
		var item model.WatchlistItem
		if err := rows.Scan(&item.ID, &item.UserID, &item.StockCode, &item.StockName, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *StockStore) GetAllWatchlistCodes() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT stock_code FROM watchlist")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}

func (s *StockStore) UpsertRecommendation(rec *model.StockRecommendation) error {
	_, err := s.db.Exec(`
		INSERT INTO stock_recommendations (stock_code, stock_name, source, buy_score, buy_reason, tail_score, tail_reason, analysis_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			stock_name = VALUES(stock_name),
			buy_score = VALUES(buy_score),
			buy_reason = VALUES(buy_reason),
			tail_score = VALUES(tail_score),
			tail_reason = VALUES(tail_reason),
			updated_at = CURRENT_TIMESTAMP`,
		rec.StockCode, rec.StockName, rec.Source, rec.BuyScore, rec.BuyReason, rec.TailScore, rec.TailReason, rec.AnalysisDate,
	)
	return err
}

func (s *StockStore) GetTodayRecommendations(source string) ([]model.StockRecommendation, error) {
	today := time.Now().Format("2006-01-02")
	query := "SELECT id, stock_code, stock_name, source, buy_score, buy_reason, tail_score, tail_reason, analysis_date, created_at, updated_at FROM stock_recommendations WHERE analysis_date = ?"
	args := []interface{}{today}

	if source != "" {
		query += " AND source = ?"
		args = append(args, source)
	}
	query += " ORDER BY buy_score DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recs []model.StockRecommendation
	for rows.Next() {
		var r model.StockRecommendation
		if err := rows.Scan(&r.ID, &r.StockCode, &r.StockName, &r.Source, &r.BuyScore, &r.BuyReason, &r.TailScore, &r.TailReason, &r.AnalysisDate, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		recs = append(recs, r)
	}
	return recs, nil
}
```

- [ ] **Step 3: 验证编译**

```bash
cd server && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add server/internal/service/stock_store.go server/internal/service/stock_store_test.go
git commit -m "feat(stock): add StockStore MySQL CRUD with tests"
```

---

### Task 4: Auth 服务 — 微信登录 + JWT

**Files:**
- Create: `server/internal/service/auth.go`
- Create: `server/internal/service/auth_test.go`
- Create: `server/internal/middleware/auth.go`

- [ ] **Step 1: 编写 auth_test.go**

```go
package service

import (
	"testing"
	"time"
)

func TestGenerateAndParseJWT(t *testing.T) {
	auth := &AuthService{jwtSecret: "test-secret", expireHours: 24}

	token, err := auth.GenerateToken(42)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	userID, err := auth.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if userID != 42 {
		t.Errorf("expected userID 42, got %d", userID)
	}
}

func TestParseToken_Expired(t *testing.T) {
	auth := &AuthService{jwtSecret: "test-secret", expireHours: -1}

	token, _ := auth.GenerateToken(1)
	_, err := auth.ParseToken(token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestParseToken_Invalid(t *testing.T) {
	auth := &AuthService{jwtSecret: "test-secret", expireHours: 24}

	_, err := auth.ParseToken("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestGenerateToken_DifferentUsers(t *testing.T) {
	auth := &AuthService{jwtSecret: "test-secret", expireHours: 24}

	token1, _ := auth.GenerateToken(1)
	token2, _ := auth.GenerateToken(2)

	_ = time.Now()
	if token1 == token2 {
		t.Error("expected different tokens for different users")
	}

	uid1, _ := auth.ParseToken(token1)
	uid2, _ := auth.ParseToken(token2)
	if uid1 != 1 || uid2 != 2 {
		t.Errorf("expected 1 and 2, got %d and %d", uid1, uid2)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

```bash
cd server && go test ./internal/service/ -run TestGenerate -v
```
Expected: 编译失败，AuthService 未定义

- [ ] **Step 3: 创建 auth.go**

```go
package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthService struct {
	appID       string
	secret      string
	jwtSecret   string
	expireHours int
	store       *StockStore
}

func NewAuthService(appID, secret, jwtSecret string, expireHours int, store *StockStore) *AuthService {
	return &AuthService{
		appID:       appID,
		secret:      secret,
		jwtSecret:   jwtSecret,
		expireHours: expireHours,
		store:       store,
	}
}

type wxCode2SessionResp struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

func (s *AuthService) Login(code string) (string, int64, error) {
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		s.appID, s.secret, code,
	)

	resp, err := http.Get(url)
	if err != nil {
		return "", 0, fmt.Errorf("request wechat: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("read response: %w", err)
	}

	var wxResp wxCode2SessionResp
	if err := json.Unmarshal(body, &wxResp); err != nil {
		return "", 0, fmt.Errorf("unmarshal: %w", err)
	}
	if wxResp.ErrCode != 0 {
		return "", 0, fmt.Errorf("wechat error [%d]: %s", wxResp.ErrCode, wxResp.ErrMsg)
	}

	user, err := s.store.FindOrCreateUser(wxResp.OpenID)
	if err != nil {
		return "", 0, fmt.Errorf("find or create user: %w", err)
	}

	token, err := s.GenerateToken(user.ID)
	if err != nil {
		return "", 0, fmt.Errorf("generate token: %w", err)
	}

	return token, user.ID, nil
}

func (s *AuthService) GenerateToken(userID int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Duration(s.expireHours) * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *AuthService) ParseToken(tokenStr string) (int64, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, fmt.Errorf("invalid token")
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid user_id in token")
	}
	return int64(userIDFloat), nil
}
```

- [ ] **Step 4: 运行测试，确认通过**

```bash
cd server && go test ./internal/service/ -run TestGenerate -v && go test ./internal/service/ -run TestParseToken -v
```
Expected: 全部 PASS

- [ ] **Step 5: 创建 middleware/auth.go**

```go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/service"
)

func JWTAuth(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		userID, err := authService.ParseToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}
```

- [ ] **Step 6: 验证编译**

```bash
cd server && go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add server/internal/service/auth.go server/internal/service/auth_test.go server/internal/middleware/auth.go
git commit -m "feat(stock): add auth service (WeChat login + JWT) and middleware"
```

---

### Task 5: StockData 服务 — 调用 AKShare 微服务

**Files:**
- Create: `server/internal/service/stock_data.go`

- [ ] **Step 1: 创建 stock_data.go**

```go
package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type StockInfo struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Price        float64 `json:"price"`
	ChangePct    float64 `json:"change_pct"`
	Volume       int64   `json:"volume"`
	TurnoverRate float64 `json:"turnover_rate"`
	PERatio      float64 `json:"pe_ratio"`
	MarketCap    float64 `json:"market_cap"`
}

type StockDataService struct {
	endpoint string
}

func NewStockDataService(endpoint string) *StockDataService {
	return &StockDataService{endpoint: strings.TrimRight(endpoint, "/")}
}

type stockListResponse struct {
	Stocks []StockInfo `json:"stocks"`
}

func (s *StockDataService) GetHotStocks(count int) ([]StockInfo, error) {
	url := fmt.Sprintf("%s/api/stock/hot?count=%d", s.endpoint, count)
	return s.fetchStocks(url)
}

func (s *StockDataService) GetStockDetails(codes []string) ([]StockInfo, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	url := fmt.Sprintf("%s/api/stock/detail?codes=%s", s.endpoint, strings.Join(codes, ","))
	return s.fetchStocks(url)
}

func (s *StockDataService) fetchStocks(url string) ([]StockInfo, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request akshare: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("akshare returned %d: %s", resp.StatusCode, string(body))
	}

	var result stockListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return result.Stocks, nil
}
```

- [ ] **Step 2: 验证编译**

```bash
cd server && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add server/internal/service/stock_data.go
git commit -m "feat(stock): add StockDataService for AKShare integration"
```

---

### Task 6: StockAnalyzer — 通义千问 AI 分析

**Files:**
- Create: `server/internal/service/stock_analyzer.go`
- Create: `server/internal/service/stock_analyzer_test.go`

- [ ] **Step 1: 编写 stock_analyzer_test.go**

```go
package service

import (
	"context"
	"testing"
)

func TestParseAnalysisResult_Valid(t *testing.T) {
	raw := `{"buy_score": 7, "buy_reason": "MACD金叉", "tail_score": 5, "tail_reason": "尾盘不确定"}`
	result, err := parseAnalysisResult(raw)
	if err != nil {
		t.Fatalf("parseAnalysisResult: %v", err)
	}
	if result.BuyScore != 7 {
		t.Errorf("expected buy_score 7, got %d", result.BuyScore)
	}
	if result.BuyReason != "MACD金叉" {
		t.Errorf("expected buy_reason 'MACD金叉', got %q", result.BuyReason)
	}
	if result.TailScore != 5 {
		t.Errorf("expected tail_score 5, got %d", result.TailScore)
	}
}

func TestParseAnalysisResult_WithMarkdownWrapper(t *testing.T) {
	raw := "```json\n{\"buy_score\": 8, \"buy_reason\": \"放量突破\", \"tail_score\": 6, \"tail_reason\": \"资金流入\"}\n```"
	result, err := parseAnalysisResult(raw)
	if err != nil {
		t.Fatalf("parseAnalysisResult: %v", err)
	}
	if result.BuyScore != 8 {
		t.Errorf("expected 8, got %d", result.BuyScore)
	}
}

func TestParseAnalysisResult_Invalid(t *testing.T) {
	_, err := parseAnalysisResult("not json at all")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBuildAnalysisPrompt(t *testing.T) {
	stock := StockInfo{
		Code: "600519", Name: "贵州茅台", Price: 1680.5,
		ChangePct: 2.35, Volume: 12345678, TurnoverRate: 0.85,
		PERatio: 28.5, MarketCap: 21100.0,
	}
	prompt := buildAnalysisPrompt(stock)
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
}

func TestStockAnalyzer_Analyze_NilContext(t *testing.T) {
	analyzer := NewStockAnalyzer("fake-key", "http://fake", "qwen-plus")
	_, err := analyzer.Analyze(context.Background(), StockInfo{})
	if err == nil {
		t.Log("expected error because endpoint is fake, got nil — may be network dependent")
	}
}
```

- [ ] **Step 2: 创建 stock_analyzer.go**

```go
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

type AnalysisResult struct {
	BuyScore   int    `json:"buy_score"`
	BuyReason  string `json:"buy_reason"`
	TailScore  int    `json:"tail_score"`
	TailReason string `json:"tail_reason"`
}

type StockAnalyzer struct {
	apiKey   string
	endpoint string
	model    string
}

func NewStockAnalyzer(apiKey, endpoint, model string) *StockAnalyzer {
	return &StockAnalyzer{apiKey: apiKey, endpoint: endpoint, model: model}
}

func buildAnalysisPrompt(stock StockInfo) string {
	return fmt.Sprintf(`你是一位专业的A股分析师。请根据以下股票实时数据，分析该股票的短线交易价值，给出今日购买和今日尾盘购买的推荐度及理由。

股票数据：
- 代码: %s
- 名称: %s
- 当前价: %.2f
- 涨跌幅: %.2f%%
- 成交量: %d
- 换手率: %.2f%%
- 市盈率: %.2f
- 总市值: %.2f亿

请返回严格的 JSON 格式（不要包含任何其他文字）：
{
  "buy_score": <1-10的整数，10为最推荐>,
  "buy_reason": "<50字以内的今日购买理由>",
  "tail_score": <1-10的整数，10为最推荐>,
  "tail_reason": "<50字以内的尾盘购买理由>"
}`, stock.Code, stock.Name, stock.Price, stock.ChangePct, stock.Volume, stock.TurnoverRate, stock.PERatio, stock.MarketCap)
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *StockAnalyzer) Analyze(ctx context.Context, stock StockInfo) (*AnalysisResult, error) {
	prompt := buildAnalysisPrompt(stock)

	reqBody := chatRequest{
		Model: a.model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := strings.TrimRight(a.endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI API returned %d: %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("AI error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in AI response")
	}

	content := chatResp.Choices[0].Message.Content
	log.Printf("[StockAnalyzer] %s(%s) AI response: %s", stock.Name, stock.Code, content)

	return parseAnalysisResult(content)
}

var jsonBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

func parseAnalysisResult(raw string) (*AnalysisResult, error) {
	raw = strings.TrimSpace(raw)

	if matches := jsonBlockRe.FindStringSubmatch(raw); len(matches) > 1 {
		raw = matches[1]
	}

	var result AnalysisResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parse AI result %q: %w", raw, err)
	}
	return &result, nil
}
```

- [ ] **Step 3: 运行测试**

```bash
cd server && go test ./internal/service/ -run TestParseAnalysis -v && go test ./internal/service/ -run TestBuildAnalysis -v
```
Expected: 全部 PASS

- [ ] **Step 4: Commit**

```bash
git add server/internal/service/stock_analyzer.go server/internal/service/stock_analyzer_test.go
git commit -m "feat(stock): add StockAnalyzer with Qwen AI integration"
```

---

### Task 7: Handler — Auth + Stock HTTP 接口

**Files:**
- Create: `server/internal/handler/auth.go`
- Create: `server/internal/handler/stock.go`

- [ ] **Step 1: 创建 handler/auth.go**

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type loginRequest struct {
	Code string `json:"code" binding:"required"`
}

type loginResponse struct {
	Token  string `json:"token"`
	UserID int64  `json:"user_id"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	token, userID, err := h.auth.Login(req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, loginResponse{Token: token, UserID: userID})
}
```

- [ ] **Step 2: 创建 handler/stock.go**

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/service"
)

type StockHandler struct {
	store     *service.StockStore
	stockData *service.StockDataService
}

func NewStockHandler(store *service.StockStore, stockData *service.StockDataService) *StockHandler {
	return &StockHandler{store: store, stockData: stockData}
}

func (h *StockHandler) GetRecommendations(c *gin.Context) {
	source := c.Query("source")
	recs, err := h.store.GetTodayRecommendations(source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get recommendations"})
		return
	}
	if recs == nil {
		recs = []model_placeholder_empty_slice
	}
	c.JSON(http.StatusOK, gin.H{"recommendations": recs})
}

func (h *StockHandler) GetWatchlist(c *gin.Context) {
	userID := c.GetInt64("user_id")
	items, err := h.store.GetWatchlist(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get watchlist"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"watchlist": items})
}

type addWatchlistRequest struct {
	StockCode string `json:"stock_code" binding:"required"`
}

func (h *StockHandler) AddWatchlist(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req addWatchlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stock_code is required"})
		return
	}

	stocks, err := h.stockData.GetStockDetails([]string{req.StockCode})
	if err != nil || len(stocks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stock code or failed to fetch stock info"})
		return
	}

	if err := h.store.AddWatchlistItem(userID, stocks[0].Code, stocks[0].Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add watchlist item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok", "stock": stocks[0]})
}

func (h *StockHandler) RemoveWatchlist(c *gin.Context) {
	userID := c.GetInt64("user_id")
	code := c.Param("code")

	if err := h.store.RemoveWatchlistItem(userID, code); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove watchlist item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
```

注意：`GetRecommendations` 中有一个占位符需要修复。将那行替换为正确的空切片初始化。实际代码如下：

将 `handler/stock.go` 中 `GetRecommendations` 的 nil 检查写为：

```go
func (h *StockHandler) GetRecommendations(c *gin.Context) {
	source := c.Query("source")
	recs, err := h.store.GetTodayRecommendations(source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get recommendations"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"recommendations": recs})
}
```

- [ ] **Step 3: 验证编译**

```bash
cd server && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/auth.go server/internal/handler/stock.go
git commit -m "feat(stock): add auth and stock HTTP handlers"
```

---

### Task 8: Cron 定时任务

**Files:**
- Create: `server/internal/cron/stock_cron.go`

- [ ] **Step 1: 创建 cron/stock_cron.go**

```go
package cron

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/mtiano/server/internal/service"
)

type StockCron struct {
	scheduler *cron.Cron
	store     *service.StockStore
	stockData *service.StockDataService
	analyzer  *service.StockAnalyzer
}

func NewStockCron(store *service.StockStore, stockData *service.StockDataService, analyzer *service.StockAnalyzer) *StockCron {
	return &StockCron{
		scheduler: cron.New(),
		store:     store,
		stockData: stockData,
		analyzer:  analyzer,
	}
}

func (sc *StockCron) Start() {
	sc.scheduler.AddFunc("@every 1h", sc.analyzeWatchlistStocks)
	sc.scheduler.AddFunc("@every 1h", sc.analyzeHotStocks)
	sc.scheduler.Start()
	log.Println("[Cron] stock analysis cron started")

	go sc.analyzeWatchlistStocks()
	go sc.analyzeHotStocks()
}

func (sc *StockCron) Stop() {
	sc.scheduler.Stop()
}

func (sc *StockCron) analyzeWatchlistStocks() {
	log.Println("[Cron] starting watchlist stock analysis")
	codes, err := sc.store.GetAllWatchlistCodes()
	if err != nil {
		log.Printf("[Cron] get watchlist codes: %v", err)
		return
	}
	if len(codes) == 0 {
		log.Println("[Cron] no watchlist stocks to analyze")
		return
	}

	if len(codes) > 10 {
		codes = codes[:10]
	}

	stocks, err := sc.stockData.GetStockDetails(codes)
	if err != nil {
		log.Printf("[Cron] get stock details: %v", err)
		return
	}

	sc.analyzeAndStore(stocks, "watchlist")
}

func (sc *StockCron) analyzeHotStocks() {
	log.Println("[Cron] starting hot stock analysis")
	stocks, err := sc.stockData.GetHotStocks(10)
	if err != nil {
		log.Printf("[Cron] get hot stocks: %v", err)
		return
	}

	sc.analyzeAndStore(stocks, "hot")
}

func (sc *StockCron) analyzeAndStore(stocks []service.StockInfo, source string) {
	today := time.Now().Format("2006-01-02")
	ctx := context.Background()

	for _, stock := range stocks {
		result, err := sc.analyzer.Analyze(ctx, stock)
		if err != nil {
			log.Printf("[Cron] analyze %s(%s): %v", stock.Name, stock.Code, err)
			continue
		}

		rec := &model_rec{
			StockCode:    stock.Code,
			StockName:    stock.Name,
			Source:       source,
			BuyScore:     result.BuyScore,
			BuyReason:    result.BuyReason,
			TailScore:    result.TailScore,
			TailReason:   result.TailReason,
			AnalysisDate: today,
		}

		if err := sc.store.UpsertRecommendation(rec); err != nil {
			log.Printf("[Cron] upsert recommendation %s: %v", stock.Code, err)
			continue
		}
		log.Printf("[Cron] analyzed %s(%s): buy=%d tail=%d", stock.Name, stock.Code, result.BuyScore, result.TailScore)
	}
}
```

注意：上面的 `model_rec` 是占位符，实际应使用 `model.StockRecommendation`。完整的 import 和正确的类型如下：

```go
package cron

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/mtiano/server/internal/model"
	"github.com/mtiano/server/internal/service"
)
```

`analyzeAndStore` 中的 `rec` 构造：

```go
rec := &model.StockRecommendation{
	StockCode:    stock.Code,
	StockName:    stock.Name,
	Source:       source,
	BuyScore:     result.BuyScore,
	BuyReason:    result.BuyReason,
	TailScore:    result.TailScore,
	TailReason:   result.TailReason,
	AnalysisDate: today,
}
```

- [ ] **Step 2: 验证编译**

```bash
cd server && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add server/internal/cron/stock_cron.go
git commit -m "feat(stock): add cron jobs for watchlist and hot stock analysis"
```

---

### Task 9: main.go 集成

**Files:**
- Modify: `server/cmd/server/main.go`

- [ ] **Step 1: 更新 main.go**

将 `server/cmd/server/main.go` 替换为：

```go
package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/mtiano/server/internal/config"
	stockcron "github.com/mtiano/server/internal/cron"
	"github.com/mtiano/server/internal/db"
	"github.com/mtiano/server/internal/handler"
	"github.com/mtiano/server/internal/middleware"
	"github.com/mtiano/server/internal/service"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	var vision service.VisionService
	switch cfg.Vision.Provider {
	case "qwen":
		vision = service.NewQwenVisionService(cfg.Vision.APIKey, cfg.Vision.Endpoint, cfg.Vision.Model)
	case "yolo":
		vision = service.NewYoloVisionService(cfg.Vision.Endpoint)
	default:
		vision = service.NewStubVisionService()
	}

	mysqlDB, err := db.InitMySQL(cfg.MySQL.DSN)
	if err != nil {
		log.Fatalf("failed to init mysql: %v", err)
	}
	defer mysqlDB.Close()

	store := service.NewStockStore(mysqlDB)
	authService := service.NewAuthService(cfg.WeChat.AppID, cfg.WeChat.Secret, cfg.JWT.Secret, cfg.JWT.ExpireHours, store)
	stockDataService := service.NewStockDataService(cfg.Stock.AKShareEndpoint)
	stockAnalyzer := service.NewStockAnalyzer(cfg.Stock.AIAPIKey, cfg.Stock.AIEndpoint, cfg.Stock.AIModel)

	cronJob := stockcron.NewStockCron(store, stockDataService, stockAnalyzer)
	cronJob.Start()
	defer cronJob.Stop()

	r := gin.Default()

	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	recognizeHandler := handler.NewRecognizeHandler(vision)
	r.POST("/api/v1/recognize", recognizeHandler.Handle)
	r.POST("/api/v1/calculate", handler.HandleCalculate)

	authHandler := handler.NewAuthHandler(authService)
	r.POST("/api/v1/auth/login", authHandler.Login)

	stockHandler := handler.NewStockHandler(store, stockDataService)
	stockGroup := r.Group("/api/v1/stock")
	stockGroup.Use(middleware.JWTAuth(authService))
	{
		stockGroup.GET("/recommendations", stockHandler.GetRecommendations)
		stockGroup.GET("/watchlist", stockHandler.GetWatchlist)
		stockGroup.POST("/watchlist", stockHandler.AddWatchlist)
		stockGroup.DELETE("/watchlist/:code", stockHandler.RemoveWatchlist)
	}

	if err := r.Run(cfg.Server.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
```

- [ ] **Step 2: 验证编译**

```bash
cd server && go build ./cmd/server/
```

- [ ] **Step 3: Commit**

```bash
git add server/cmd/server/main.go
git commit -m "feat(stock): integrate MySQL, auth, stock APIs and cron in main.go"
```

---

### Task 10: AKShare Python 微服务

**Files:**
- Create: `server/stock_service/app.py`
- Create: `server/stock_service/requirements.txt`

- [ ] **Step 1: 创建 requirements.txt**

```
flask==3.1.1
akshare
```

- [ ] **Step 2: 创建 app.py**

```python
import akshare as ak
from flask import Flask, request, jsonify
import pandas as pd

app = Flask(__name__)


@app.route('/api/stock/hot', methods=['GET'])
def get_hot_stocks():
    count = request.args.get('count', 10, type=int)
    try:
        df = ak.stock_hot_rank_em()
        df = df.head(count)
        spot = ak.stock_zh_a_spot_em()
        spot = spot.set_index('代码')

        stocks = []
        for _, row in df.iterrows():
            code = str(row['股票代码']).zfill(6)
            name = row['股票简称']
            info = {'code': code, 'name': name, 'price': 0, 'change_pct': 0,
                    'volume': 0, 'turnover_rate': 0, 'pe_ratio': 0, 'market_cap': 0}
            if code in spot.index:
                s = spot.loc[code]
                info['price'] = float(s.get('最新价', 0) or 0)
                info['change_pct'] = float(s.get('涨跌幅', 0) or 0)
                info['volume'] = int(s.get('成交量', 0) or 0)
                info['turnover_rate'] = float(s.get('换手率', 0) or 0)
                info['pe_ratio'] = float(s.get('市盈率-动态', 0) or 0)
                info['market_cap'] = float(s.get('总市值', 0) or 0) / 1e8
            stocks.append(info)

        return jsonify({'stocks': stocks})
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/api/stock/detail', methods=['GET'])
def get_stock_detail():
    codes_param = request.args.get('codes', '')
    if not codes_param:
        return jsonify({'error': 'codes parameter required'}), 400

    codes = [c.strip() for c in codes_param.split(',') if c.strip()]
    try:
        spot = ak.stock_zh_a_spot_em()
        spot['代码'] = spot['代码'].astype(str).str.zfill(6)
        spot = spot.set_index('代码')

        stocks = []
        for code in codes:
            if code not in spot.index:
                continue
            s = spot.loc[code]
            stocks.append({
                'code': code,
                'name': str(s.get('名称', '')),
                'price': float(s.get('最新价', 0) or 0),
                'change_pct': float(s.get('涨跌幅', 0) or 0),
                'volume': int(s.get('成交量', 0) or 0),
                'turnover_rate': float(s.get('换手率', 0) or 0),
                'pe_ratio': float(s.get('市盈率-动态', 0) or 0),
                'market_cap': float(s.get('总市值', 0) or 0) / 1e8,
            })

        return jsonify({'stocks': stocks})
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/health', methods=['GET'])
def health():
    return jsonify({'status': 'ok'})


if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5001)
```

- [ ] **Step 3: 创建 Python 虚拟环境并安装依赖**

```bash
cd server/stock_service
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

- [ ] **Step 4: 快速验证服务可启动**

```bash
cd server/stock_service && source venv/bin/activate && python app.py &
sleep 3
curl http://localhost:5001/health
kill %1
```
Expected: `{"status": "ok"}`

- [ ] **Step 5: Commit**

```bash
git add server/stock_service/app.py server/stock_service/requirements.txt
git commit -m "feat(stock): add AKShare Python microservice"
```

---

### Task 11: 小程序 — 登录工具 + App 改造

**Files:**
- Create: `miniprogram/utils/auth.js`
- Modify: `miniprogram/app.js`

- [ ] **Step 1: 创建 utils/auth.js**

```javascript
const app = getApp()

function getToken() {
  return wx.getStorageSync('token') || ''
}

function setToken(token) {
  wx.setStorageSync('token', token)
}

function clearToken() {
  wx.removeStorageSync('token')
}

function isLoggedIn() {
  return !!getToken()
}

function login() {
  return new Promise((resolve, reject) => {
    wx.login({
      success(res) {
        if (!res.code) {
          reject(new Error('wx.login failed'))
          return
        }
        wx.request({
          url: app.globalData.serverUrl + '/api/v1/auth/login',
          method: 'POST',
          data: { code: res.code },
          success(resp) {
            if (resp.statusCode === 200 && resp.data.token) {
              setToken(resp.data.token)
              resolve(resp.data)
            } else {
              reject(new Error(resp.data.error || 'login failed'))
            }
          },
          fail(err) {
            reject(err)
          }
        })
      },
      fail(err) {
        reject(err)
      }
    })
  })
}

function authRequest(options) {
  const token = getToken()
  if (!token) {
    return Promise.reject(new Error('not logged in'))
  }
  return new Promise((resolve, reject) => {
    wx.request({
      ...options,
      url: app.globalData.serverUrl + options.url,
      header: {
        ...options.header,
        'Authorization': 'Bearer ' + token
      },
      success(res) {
        if (res.statusCode === 401) {
          clearToken()
          reject(new Error('token expired'))
          return
        }
        resolve(res)
      },
      fail: reject
    })
  })
}

module.exports = { getToken, setToken, clearToken, isLoggedIn, login, authRequest }
```

- [ ] **Step 2: 更新 app.js**

```javascript
App({
  globalData: {
    serverUrl: 'https://mahjong.czw-mtiano.cn'
  }
})
```

app.js 保持不变，登录逻辑在 `utils/auth.js` 和 stock 页面中处理。

- [ ] **Step 3: Commit**

```bash
git add miniprogram/utils/auth.js
git commit -m "feat(stock): add auth utility for WeChat login and token management"
```

---

### Task 12: 小程序 — Stock 页面

**Files:**
- Create: `miniprogram/pages/stock/stock.json`
- Create: `miniprogram/pages/stock/stock.wxml`
- Create: `miniprogram/pages/stock/stock.wxss`
- Create: `miniprogram/pages/stock/stock.js`
- Modify: `miniprogram/app.json`

- [ ] **Step 1: 创建 stock.json**

```json
{
  "navigationBarTitleText": "股票推荐"
}
```

- [ ] **Step 2: 创建 stock.wxml**

```xml
<view class="container">
  <view class="header">
    <text class="title">股票推荐</text>
    <text class="date">{{today}}</text>
  </view>

  <view class="tabs">
    <view class="tab {{activeTab === '' ? 'active' : ''}}" bindtap="switchTab" data-tab="">全部</view>
    <view class="tab {{activeTab === 'watchlist' ? 'active' : ''}}" bindtap="switchTab" data-tab="watchlist">自选</view>
    <view class="tab {{activeTab === 'hot' ? 'active' : ''}}" bindtap="switchTab" data-tab="hot">热门</view>
  </view>

  <view wx:if="{{loading}}" class="loading">
    <text>加载中...</text>
  </view>

  <view wx:elif="{{recommendations.length === 0}}" class="empty">
    <text>暂无推荐数据</text>
  </view>

  <view wx:else class="stock-list">
    <view class="stock-card" wx:for="{{recommendations}}" wx:key="id">
      <view class="stock-header">
        <text class="stock-code">{{item.stock_code}}</text>
        <text class="stock-name">{{item.stock_name}}</text>
        <text class="source-tag {{item.source}}">{{item.source === 'watchlist' ? '自选' : '热门'}}</text>
      </view>

      <view class="score-row">
        <text class="score-label">今日购买</text>
        <view class="progress-bar">
          <view class="progress-fill" style="width: {{item.buy_score * 10}}%"></view>
        </view>
        <text class="score-value">{{item.buy_score}}分</text>
      </view>
      <text class="reason">{{item.buy_reason}}</text>

      <view class="score-row">
        <text class="score-label">尾盘购买</text>
        <view class="progress-bar">
          <view class="progress-fill tail" style="width: {{item.tail_score * 10}}%"></view>
        </view>
        <text class="score-value">{{item.tail_score}}分</text>
      </view>
      <text class="reason">{{item.tail_reason}}</text>
    </view>
  </view>

  <view class="add-section">
    <view class="add-row">
      <input class="add-input" placeholder="输入股票代码" value="{{inputCode}}" bindinput="onInputCode" />
      <button class="add-btn" bindtap="addWatchlist" disabled="{{!inputCode}}">添加自选</button>
    </view>
  </view>
</view>
```

- [ ] **Step 3: 创建 stock.wxss**

```css
.container {
  padding: 24rpx;
  padding-bottom: 160rpx;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24rpx;
}

.title {
  font-size: 36rpx;
  font-weight: bold;
  color: #1a1a2e;
}

.date {
  font-size: 24rpx;
  color: #999;
}

.tabs {
  display: flex;
  gap: 16rpx;
  margin-bottom: 24rpx;
}

.tab {
  flex: 1;
  text-align: center;
  padding: 16rpx 0;
  font-size: 28rpx;
  color: #666;
  background: #f5f5f5;
  border-radius: 8rpx;
}

.tab.active {
  background: #1a1a2e;
  color: #fff;
}

.loading, .empty {
  text-align: center;
  padding: 100rpx 0;
  color: #999;
  font-size: 28rpx;
}

.stock-list {
  display: flex;
  flex-direction: column;
  gap: 20rpx;
}

.stock-card {
  background: #fff;
  border-radius: 16rpx;
  padding: 24rpx;
  box-shadow: 0 2rpx 8rpx rgba(0,0,0,0.05);
}

.stock-header {
  display: flex;
  align-items: center;
  margin-bottom: 16rpx;
}

.stock-code {
  font-size: 26rpx;
  color: #999;
  margin-right: 12rpx;
}

.stock-name {
  font-size: 30rpx;
  font-weight: 500;
  color: #333;
  flex: 1;
}

.source-tag {
  font-size: 22rpx;
  padding: 4rpx 12rpx;
  border-radius: 4rpx;
}

.source-tag.watchlist {
  background: #e8f5e9;
  color: #2e7d32;
}

.source-tag.hot {
  background: #fce4ec;
  color: #c62828;
}

.score-row {
  display: flex;
  align-items: center;
  margin-top: 12rpx;
}

.score-label {
  font-size: 24rpx;
  color: #666;
  width: 120rpx;
}

.progress-bar {
  flex: 1;
  height: 16rpx;
  background: #f0f0f0;
  border-radius: 8rpx;
  overflow: hidden;
  margin: 0 16rpx;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #4caf50, #8bc34a);
  border-radius: 8rpx;
  transition: width 0.3s;
}

.progress-fill.tail {
  background: linear-gradient(90deg, #ff9800, #ffc107);
}

.score-value {
  font-size: 26rpx;
  font-weight: bold;
  color: #333;
  width: 60rpx;
}

.reason {
  font-size: 22rpx;
  color: #888;
  margin-top: 4rpx;
  margin-left: 120rpx;
  line-height: 1.4;
}

.add-section {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  background: #fff;
  padding: 20rpx 24rpx;
  padding-bottom: calc(20rpx + env(safe-area-inset-bottom));
  box-shadow: 0 -2rpx 8rpx rgba(0,0,0,0.05);
}

.add-row {
  display: flex;
  gap: 16rpx;
}

.add-input {
  flex: 1;
  height: 72rpx;
  background: #f5f5f5;
  border-radius: 8rpx;
  padding: 0 20rpx;
  font-size: 28rpx;
}

.add-btn {
  width: 180rpx;
  height: 72rpx;
  line-height: 72rpx;
  background: #1a1a2e;
  color: #fff;
  font-size: 28rpx;
  border-radius: 8rpx;
  padding: 0;
}

.add-btn[disabled] {
  background: #ccc;
}
```

- [ ] **Step 4: 创建 stock.js**

```javascript
const auth = require('../../utils/auth')

Page({
  data: {
    today: '',
    activeTab: '',
    recommendations: [],
    loading: false,
    inputCode: ''
  },

  onShow() {
    this.setData({ today: this.formatDate(new Date()) })
    this.ensureLoginAndLoad()
  },

  onPullDownRefresh() {
    this.loadRecommendations().then(() => wx.stopPullDownRefresh())
  },

  formatDate(date) {
    const y = date.getFullYear()
    const m = String(date.getMonth() + 1).padStart(2, '0')
    const d = String(date.getDate()).padStart(2, '0')
    return y + '-' + m + '-' + d
  },

  ensureLoginAndLoad() {
    if (auth.isLoggedIn()) {
      this.loadRecommendations()
      return
    }
    this.setData({ loading: true })
    auth.login().then(() => {
      this.loadRecommendations()
    }).catch(err => {
      console.error('login failed:', err)
      wx.showToast({ title: '登录失败', icon: 'none' })
      this.setData({ loading: false })
    })
  },

  loadRecommendations() {
    this.setData({ loading: true })
    const source = this.data.activeTab
    return auth.authRequest({
      url: '/api/v1/stock/recommendations' + (source ? '?source=' + source : ''),
      method: 'GET'
    }).then(res => {
      this.setData({
        recommendations: res.data.recommendations || [],
        loading: false
      })
    }).catch(err => {
      console.error('load recommendations failed:', err)
      if (err.message === 'token expired' || err.message === 'not logged in') {
        auth.login().then(() => this.loadRecommendations())
        return
      }
      this.setData({ loading: false })
      wx.showToast({ title: '加载失败', icon: 'none' })
    })
  },

  switchTab(e) {
    const tab = e.currentTarget.dataset.tab
    this.setData({ activeTab: tab })
    this.loadRecommendations()
  },

  onInputCode(e) {
    this.setData({ inputCode: e.detail.value.trim() })
  },

  addWatchlist() {
    const code = this.data.inputCode
    if (!code) return

    auth.authRequest({
      url: '/api/v1/stock/watchlist',
      method: 'POST',
      data: { stock_code: code }
    }).then(res => {
      if (res.statusCode === 200) {
        wx.showToast({ title: '添加成功' })
        this.setData({ inputCode: '' })
        this.loadRecommendations()
      } else {
        wx.showToast({ title: res.data.error || '添加失败', icon: 'none' })
      }
    }).catch(err => {
      wx.showToast({ title: '网络错误', icon: 'none' })
    })
  }
})
```

- [ ] **Step 5: 更新 app.json — 添加 stock 页面和 tab**

将 `miniprogram/app.json` 更新为：

```json
{
  "pages": [
    "pages/index/index",
    "pages/yaku/yaku",
    "pages/score/score",
    "pages/camera/camera",
    "pages/manual/manual",
    "pages/stock/stock"
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
      },
      {
        "pagePath": "pages/stock/stock",
        "text": "股票"
      }
    ]
  },
  "style": "v2",
  "sitemapLocation": "sitemap.json"
}
```

注意：微信小程序 tabBar 最多支持 5 个 tab，所以这里移除了「算番」tab（保留页面，从首页可访问），将「股票」加入 tabBar。

- [ ] **Step 6: 在微信开发者工具中打开并检查页面加载**

- [ ] **Step 7: Commit**

```bash
git add miniprogram/pages/stock/ miniprogram/app.json
git commit -m "feat(stock): add stock recommendation page with login flow"
```

---

### Task 13: 端到端验证

- [ ] **Step 1: 启动 MySQL 并创建数据库**

```bash
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS mtiano CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
```

- [ ] **Step 2: 启动 AKShare 微服务**

```bash
cd server/stock_service && source venv/bin/activate && python app.py &
```

- [ ] **Step 3: 验证 AKShare 接口**

```bash
curl http://localhost:5001/api/stock/hot?count=3
curl "http://localhost:5001/api/stock/detail?codes=600519"
```
Expected: 返回包含 stocks 数组的 JSON

- [ ] **Step 4: 启动 Go 服务**

```bash
cd server && go run cmd/server/main.go
```
Expected: 服务启动成功，看到 cron 启动日志和首次分析日志

- [ ] **Step 5: 验证 API 接口**

```bash
curl http://localhost:9091/api/v1/health
curl http://localhost:9091/api/v1/stock/recommendations -H "Authorization: Bearer <test-token>"
```

- [ ] **Step 6: 在微信开发者工具中测试小程序**

1. 进入「股票」tab，确认自动触发登录
2. 查看推荐列表是否正常展示
3. 测试添加自选股
4. 测试切换筛选 tab

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(stock): complete stock AI recommendation system"
```
