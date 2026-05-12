# 股票推荐评分与候选池优化 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重构股票推荐链路,把 "每批次 ≥3 只 buy_score ≥8" 和 "AI 拒答不丢股票" 两个目标落到代码。

**Architecture:** 多源候选池(新浪三榜合并去重)→ 放宽 pre-filter(保留 ST 过滤)→ 重标定的 prompt(盘口 + 量价两层即可上 8 分)→ AI 三级容错链(原调用 / retry / 规则 fallback 入库标记 `is_fallback`)。

**Tech Stack:** Go 1.x + Gin + database/sql + MySQL(后端);Python 3 + Flask + requests(数据微服务);现有调度用 `robfig/cron/v3`。

**Spec 引用:** `docs/superpowers/specs/2026-05-12-stock-rec-score-optimize-design.md`

---

## 文件结构

**新增:**
- `server/internal/cron/stock_cron_test.go` — 候选池合并、pre-filter、ST 过滤单测(纯 Go,无 DB 依赖)

**修改:**

| 文件 | 职责 | 主要改动 |
|---|---|---|
| `server/internal/db/mysql.go` | DB 初始化 + 迁移 | `migrations` 切片追加 `is_fallback` 列 |
| `server/internal/model/recommendation.go` | DTO | 加 `IsFallback bool` |
| `server/internal/service/stock_store.go` | 存取层 | UPSERT + 查询带 `is_fallback` |
| `server/internal/service/stock_store_test.go` | 存取层集成测试 | `is_fallback` 写入 / 读取用例 |
| `server/stock_service/app.py` | 行情微服务 | 抽 `fetch_sina`,新增 `/api/stock/gainers`、`/api/stock/active` |
| `server/internal/service/stock_data.go` | 行情客户端 | `GetGainers`、`GetActive` 两方法 |
| `server/internal/service/stock_analyzer.go` | AI 评分 | 重写 `systemPrompt`、简化 `buildAnalysisPrompt`、实现 `fallbackScore`、`Analyze` 三级容错 |
| `server/internal/service/stock_analyzer_test.go` | 评分单测 | retry / fallback / scoring 边界 |
| `server/internal/cron/stock_cron.go` | 调度 | `dedupByCode`、放宽 `preFilterStocks`、ST 过滤、重写 `analyzeHotStocks` 合并三榜 |

---

## Task 1: DB schema + model 加 `is_fallback`

**Files:**
- Modify: `server/internal/db/mysql.go:64-71`
- Modify: `server/internal/model/recommendation.go:5-20`
- Modify: `server/internal/service/stock_store.go:89-138`
- Modify: `server/internal/service/stock_store_test.go:85-121`

- [ ] **Step 1.1: 加 migration 到 `mysql.go`**

把 `migrations` 切片追加一行:

```go
migrations := []string{
    "ALTER TABLE stock_recommendations ADD COLUMN key_signals VARCHAR(100) DEFAULT '' AFTER tail_reason",
    "ALTER TABLE stock_recommendations ADD COLUMN risk_level TINYINT DEFAULT 0 AFTER key_signals",
    "ALTER TABLE stock_recommendations ADD COLUMN trap_warning VARCHAR(100) DEFAULT '' AFTER risk_level",
    "ALTER TABLE stock_recommendations ADD COLUMN is_fallback TINYINT(1) NOT NULL DEFAULT 0 AFTER trap_warning",
}
```

`createTables` 里 `db.Exec(m)` 忽略 "duplicate column" 错误,新增这条不影响已有部署。

- [ ] **Step 1.2: 给 model 加字段**

在 `server/internal/model/recommendation.go` 的 `StockRecommendation` 中,`UpdatedAt` 之前插一行:

```go
IsFallback   bool      `json:"is_fallback"`
```

- [ ] **Step 1.3: 改 `UpsertRecommendation`**

在 `stock_store.go:89-106` 改 SQL 字段与占位符:

```go
func (s *StockStore) UpsertRecommendation(rec *model.StockRecommendation) error {
	_, err := s.db.Exec(`
		INSERT INTO stock_recommendations (stock_code, stock_name, source, buy_score, buy_reason, tail_score, tail_reason, key_signals, risk_level, trap_warning, is_fallback, analysis_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			stock_name = VALUES(stock_name),
			buy_score = VALUES(buy_score),
			buy_reason = VALUES(buy_reason),
			tail_score = VALUES(tail_score),
			tail_reason = VALUES(tail_reason),
			key_signals = VALUES(key_signals),
			risk_level = VALUES(risk_level),
			trap_warning = VALUES(trap_warning),
			is_fallback = VALUES(is_fallback),
			updated_at = CURRENT_TIMESTAMP`,
		rec.StockCode, rec.StockName, rec.Source, rec.BuyScore, rec.BuyReason, rec.TailScore, rec.TailReason, rec.KeySignals, rec.RiskLevel, rec.TrapWarning, rec.IsFallback, rec.AnalysisDate,
	)
	return err
}
```

- [ ] **Step 1.4: 改 `GetTodayRecommendations`**

在 `stock_store.go:108-138`,把 SELECT 列与 scan 都扩:

```go
func (s *StockStore) GetTodayRecommendations(source string, userID int64) ([]model.StockRecommendation, error) {
	today := time.Now().Format("2006-01-02")
	query := "SELECT id, stock_code, stock_name, source, buy_score, buy_reason, tail_score, tail_reason, COALESCE(key_signals, '') as key_signals, COALESCE(risk_level, 0) as risk_level, COALESCE(trap_warning, '') as trap_warning, COALESCE(is_fallback, 0) as is_fallback, analysis_date, created_at, updated_at FROM stock_recommendations WHERE analysis_date = ?"
	args := []any{today}

	if source != "" {
		query += " AND source = ?"
		args = append(args, source)
	}
	if userID > 0 {
		query += " AND (source != 'watchlist' OR stock_code IN (SELECT stock_code FROM watchlist WHERE user_id = ?))"
		args = append(args, userID)
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
		if err := rows.Scan(&r.ID, &r.StockCode, &r.StockName, &r.Source, &r.BuyScore, &r.BuyReason, &r.TailScore, &r.TailReason, &r.KeySignals, &r.RiskLevel, &r.TrapWarning, &r.IsFallback, &r.AnalysisDate, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		recs = append(recs, r)
	}
	return recs, nil
}
```

- [ ] **Step 1.5: 扩 `TestStockStore_Recommendations` 覆盖 `is_fallback`**

改 `server/internal/service/stock_store_test.go:85-121`:

```go
func TestStockStore_Recommendations(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	store := NewStockStore(db)

	today := time.Now().Format("2006-01-02")
	defer db.Exec("DELETE FROM stock_recommendations WHERE analysis_date = ? AND stock_code IN ('600519','000001')", today)

	rec := &model.StockRecommendation{
		StockCode: "600519", StockName: "贵州茅台", Source: "hot",
		BuyScore: 8, BuyReason: "test buy reason",
		TailScore: 6, TailReason: "test tail reason",
		IsFallback: false, AnalysisDate: today,
	}
	if err := store.UpsertRecommendation(rec); err != nil {
		t.Fatalf("UpsertRecommendation(ai): %v", err)
	}

	fbRec := &model.StockRecommendation{
		StockCode: "000001", StockName: "平安银行", Source: "hot",
		BuyScore: 6, BuyReason: "AI 服务异常,规则兜底评分",
		TailScore: 6, TailReason: "AI 服务异常,规则兜底评分",
		IsFallback: true, AnalysisDate: today,
	}
	if err := store.UpsertRecommendation(fbRec); err != nil {
		t.Fatalf("UpsertRecommendation(fallback): %v", err)
	}

	recs, err := store.GetTodayRecommendations("hot", 0)
	if err != nil {
		t.Fatalf("GetTodayRecommendations: %v", err)
	}
	var found600519, found000001 bool
	for _, r := range recs {
		if r.StockCode == "600519" && r.BuyScore == 8 && r.IsFallback == false {
			found600519 = true
		}
		if r.StockCode == "000001" && r.BuyScore == 6 && r.IsFallback == true {
			found000001 = true
		}
	}
	if !found600519 {
		t.Error("expected AI rec for 600519 with IsFallback=false")
	}
	if !found000001 {
		t.Error("expected fallback rec for 000001 with IsFallback=true")
	}
}
```

- [ ] **Step 1.6: 本地编译 + 跑测试**

```bash
cd server
go build ./...
TEST_MYSQL_DSN="<dsn>" go test ./internal/service/ -run TestStockStore -v
```

期望:编译通过;如果本地无 MySQL 则 skip;有 DSN 则 PASS。

- [ ] **Step 1.7: 提交**

```bash
git add server/internal/db/mysql.go server/internal/model/recommendation.go server/internal/service/stock_store.go server/internal/service/stock_store_test.go
git commit -m "feat(stock): add is_fallback column for rule-based fallback scoring"
```

---

## Task 2: Python 侧多源榜单

**Files:**
- Modify: `server/stock_service/app.py:62-104`

- [ ] **Step 2.1: 抽 `fetch_sina` 通用函数**

替换 `fetch_hot_sina`(62-94 行)为通用函数 + 旧名字兼容包装:

```python
def fetch_sina(count=10, sort="amount", min_change_pct=None):
    """新浪 Market_Center 通用查询"""
    url = (
        "https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/"
        "Market_Center.getHQNodeData"
    )
    params = {
        "page": 1,
        "num": count,
        "sort": sort,
        "asc": 0,
        "node": "hs_a",
        "_s_r_a": "init",
    }
    resp = requests.get(url, headers=HEADERS, params=params, timeout=10)
    data = resp.json()

    stocks = []
    for item in data:
        price = float(item.get("trade", 0) or 0)
        if price == 0:
            continue
        change_pct = float(item.get("changepercent", 0) or 0)
        if min_change_pct is not None and change_pct < min_change_pct:
            continue
        stocks.append({
            "code": item["code"],
            "name": item["name"],
            "price": price,
            "change_pct": change_pct,
            "volume": int(float(item.get("volume", 0) or 0)),
            "turnover_rate": float(item.get("turnoverratio", 0) or 0),
            "pe_ratio": float(item.get("per", 0) or 0),
            "market_cap": float(item.get("mktcap", 0) or 0) / 10000,
        })
    return stocks


def fetch_hot_sina(count=10):
    """保留原名字用于成交额榜(向后兼容)"""
    return fetch_sina(count=count, sort="amount")
```

- [ ] **Step 2.2: 加两条新路由**

在 `get_hot_stocks` 之后,`get_stock_detail` 之前(104-107 行之间)插入:

```python
@app.route("/api/stock/gainers", methods=["GET"])
def get_gainers():
    count = request.args.get("count", 20, type=int)
    try:
        stocks = fetch_sina(count=count, sort="changepercent", min_change_pct=0.01)
        return jsonify({"stocks": stocks})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/api/stock/active", methods=["GET"])
def get_active():
    count = request.args.get("count", 20, type=int)
    try:
        stocks = fetch_sina(count=count, sort="turnoverratio")
        return jsonify({"stocks": stocks})
    except Exception as e:
        return jsonify({"error": str(e)}), 500
```

- [ ] **Step 2.3: 本地启动 Python 服务冒烟**

```bash
cd server/stock_service
source venv/bin/activate 2>/dev/null || true
python app.py &
PID=$!
sleep 2
curl -s 'http://localhost:5001/api/stock/hot?count=3' | head -c 500; echo
curl -s 'http://localhost:5001/api/stock/gainers?count=3' | head -c 500; echo
curl -s 'http://localhost:5001/api/stock/active?count=3' | head -c 500; echo
kill $PID
```

期望:三个 endpoint 都返回 `{"stocks": [...]}`,非空(非交易时段也有历史缓存数据);gainers 返回的所有股 `change_pct > 0`。

- [ ] **Step 2.4: 提交**

```bash
git add server/stock_service/app.py
git commit -m "feat(stock_service): add gainers and active endpoints with generic fetch_sina"
```

---

## Task 3: Go 数据层加两个客户端方法

**Files:**
- Modify: `server/internal/service/stock_data.go:41-44`

- [ ] **Step 3.1: 加 `GetGainers` 与 `GetActive`**

在 `GetHotStocks` 方法之后(44 行后)追加:

```go
func (s *StockDataService) GetGainers(count int) ([]StockInfo, error) {
	url := fmt.Sprintf("%s/api/stock/gainers?count=%d", s.endpoint, count)
	return s.fetchStocks(url)
}

func (s *StockDataService) GetActive(count int) ([]StockInfo, error) {
	url := fmt.Sprintf("%s/api/stock/active?count=%d", s.endpoint, count)
	return s.fetchStocks(url)
}
```

- [ ] **Step 3.2: 编译验证**

```bash
cd server
go build ./...
```

期望:无错误。

- [ ] **Step 3.3: 提交**

```bash
git add server/internal/service/stock_data.go
git commit -m "feat(stock): add GetGainers and GetActive client methods"
```

---

## Task 4: 重写 Prompt(system + user + 分析器)

**Files:**
- Modify: `server/internal/service/stock_analyzer.go:35-102`

- [ ] **Step 4.1: 替换 `systemPrompt` 常量**

把 `stock_analyzer.go:35-70` 的 `const systemPrompt = ...` 整段替换为:

```go
const systemPrompt = `你是一位专注 A 股尾盘短线的资深分析师。策略:尾盘买入、次日早盘卖出,持股 ~12 小时,利用 T+1 尾盘主力表态的信息优势。

## 评分刻度(1-10,按以下比例分布)

8-10 分(强烈推荐,预计占合格票 30-40%)
  盘口 + 量价两层达标 + 无明显陷阱。
  满足下列任一加分项冲 9-10:
    - 清晰题材催化(名称/行业属于当前热点板块,如 AI 算力、新能源、消费电子等)
    - 量能显著放大(量比 ≥ 1.5 或换手 ≥ 8%)
    - 当前价接近日内高点(抗跌性好)

6-7 分(可观察,30-40%)
  盘口达标但量价或题材其中一项平淡,无明显陷阱。

4-5 分(弱势,20%)
  信号矛盾或有轻度陷阱特征。

1-3 分(规避,5-10%)
  明显陷阱 / 多项硬指标不及格。

## 数据缺失处理

若 turnover_rate / volume_ratio / pe_ratio / float_market_cap 为 0 或明显异常,
视为"数据缺失",按可得字段打分,不额外扣分,并在 key_signals 末尾附 "[数据部分缺失]"。

## 三大陷阱(检测到一项即降到 1-3 分并写 trap_warning)

1. 秒拉诱多:涨幅集中在尾盘最后几分钟,全天走势平淡 → 大概率做收盘价
2. 缩量拉升:价涨量缩,量比 < 1,无增量资金进场 → 假拉升真诱多
3. 弱转强陷阱:全天运行在均价线下方,仅尾盘勉强翻红 → 弱势股反抽

## 风险等级(1-5,与评分反向)

1: 低风险(多信号共振,量价健康)
2: 较低(主要信号满足,个别瑕疵)
3: 中等(信号矛盾,需谨慎)
4: 较高(多个风险信号)
5: 高风险(明显陷阱特征)`
```

- [ ] **Step 4.2: 简化 `buildAnalysisPrompt`**

把 `stock_analyzer.go:72-102` 的 `buildAnalysisPrompt` 函数整个替换为:

```go
func buildAnalysisPrompt(stock StockInfo) string {
	return fmt.Sprintf(`分析以下 A 股的尾盘短线交易价值(尾盘买入,次日早盘卖出):

【盘口数据】
代码: %s | 名称: %s
当前价: %.2f | 涨跌幅: %.2f%%
开盘价: %.2f | 昨收价: %.2f
最高价: %.2f | 最低价: %.2f | 振幅: %.2f%%
成交量: %d手 | 换手率: %.2f%% | 量比: %.2f
市盈率: %.2f | 总市值: %.2f亿 | 流通市值: %.2f亿

请结合盘口数据、股票名称推断的题材属性、量价配合度,输出 JSON:
{
  "buy_score": <1-10整数>,
  "buy_reason": "<40字以内的即时买入理由>",
  "tail_score": <1-10整数>,
  "tail_reason": "<40字以内的尾盘买入理由>",
  "key_signals": "<30字以内的关键信号>",
  "risk_level": <1-5整数>,
  "trap_warning": "<如有陷阱风险写30字以内警告,无则写空字符串>"
}

⚠️ 只输出 JSON 对象,不要任何前后文字、不要 代码块标记、不要解释。
无法判断也必须返回有效 JSON,score 填 3,trap_warning 写明原因。`,
		stock.Code, stock.Name, stock.Price, stock.ChangePct,
		stock.Open, stock.PrevClose, stock.High, stock.Low, stock.Amplitude,
		stock.Volume, stock.TurnoverRate, stock.VolumeRatio,
		stock.PERatio, stock.MarketCap, stock.FloatMarketCap)
}
```

- [ ] **Step 4.3: 编译验证**

```bash
cd server
go build ./...
go test ./internal/service/ -run TestBuildAnalysisPrompt -v
go test ./internal/service/ -run TestParseAnalysisResult -v
```

期望:编译通过;`TestBuildAnalysisPrompt` PASS(只断言非空);`TestParseAnalysisResult_*` 仍 PASS(prompt 结构改了,parser 行为不变)。

- [ ] **Step 4.4: 提交**

```bash
git add server/internal/service/stock_analyzer.go
git commit -m "feat(stock): recalibrate AI scoring prompt for higher pass rate on 8+"
```

---

## Task 5: 规则 fallback 打分器

**Files:**
- Modify: `server/internal/service/stock_analyzer.go`(函数追加到文件末尾)
- Modify: `server/internal/service/stock_analyzer_test.go`(追加测试)

- [ ] **Step 5.1: 先写失败测试 — `Test_fallbackScore_bucketing`**

追加到 `stock_analyzer_test.go` 末尾:

```go
func Test_fallbackScore_bucketing(t *testing.T) {
	// 5 项指标全部命中 2 分区间 → 合计 10 → buy_score cap 至 7
	strong := StockInfo{
		Code: "000001", Name: "平安银行",
		ChangePct: 4.0, VolumeRatio: 1.8, TurnoverRate: 7.5,
		FloatMarketCap: 100, Amplitude: 5.0,
	}
	res := fallbackScore(strong)
	if res.BuyScore != 7 {
		t.Errorf("strong: buy_score want 7 (capped), got %d", res.BuyScore)
	}
	if res.TailScore != 7 {
		t.Errorf("strong: tail_score want 7, got %d", res.TailScore)
	}
	if res.RiskLevel != 1 {
		t.Errorf("strong: risk want 1, got %d", res.RiskLevel)
	}
	if res.BuyReason != "AI 服务异常,规则兜底评分" {
		t.Errorf("unexpected buy_reason: %q", res.BuyReason)
	}

	// 全 0 字段 → 合计 0 → buy_score 0 (最低档) → risk clamp 到 5
	empty := StockInfo{Code: "000002", Name: "test"}
	res = fallbackScore(empty)
	if res.BuyScore != 0 {
		t.Errorf("empty: buy_score want 0, got %d", res.BuyScore)
	}
	if res.RiskLevel != 5 {
		t.Errorf("empty: risk want 5, got %d", res.RiskLevel)
	}

	// 中等:涨幅 2%(1分)、量比 1.2(1分)、换手 3%(1分)、流通 40 亿(1分)、振幅 2%(1分) = 5
	mid := StockInfo{
		Code: "600000", Name: "浦发银行",
		ChangePct: 2.0, VolumeRatio: 1.2, TurnoverRate: 3.0,
		FloatMarketCap: 40, Amplitude: 2.0,
	}
	res = fallbackScore(mid)
	if res.BuyScore != 5 {
		t.Errorf("mid: buy_score want 5, got %d", res.BuyScore)
	}
	if res.RiskLevel != 4 {
		t.Errorf("mid: risk want 4 (total=5, 6 - 5/2(int) = 4), got %d", res.RiskLevel)
	}
}
```

- [ ] **Step 5.2: 跑测试确认失败**

```bash
cd server
go test ./internal/service/ -run Test_fallbackScore_bucketing -v
```

期望:FAIL,`fallbackScore` undefined。

- [ ] **Step 5.3: 实现 `fallbackScore`**

追加到 `stock_analyzer.go` 文件末尾:

```go
// fallbackScore 基于 pre-filter 可用指标做规则打分,用于 AI 不可用时兜底入库。
// 每项 0-2 分,合计 0-10;为避免规则分混入"强烈推荐"区,buy/tail_score 硬 cap 到 7。
func fallbackScore(stock StockInfo) *AnalysisResult {
	total := 0

	c := stock.ChangePct
	if c >= 3 && c <= 5 {
		total += 2
	} else if (c >= 0.5 && c < 3) || (c > 5 && c <= 9.5) {
		total += 1
	}

	if stock.VolumeRatio >= 1.5 {
		total += 2
	} else if stock.VolumeRatio >= 1.0 {
		total += 1
	}

	tr := stock.TurnoverRate
	if tr >= 5 && tr <= 10 {
		total += 2
	} else if (tr >= 2 && tr < 5) || (tr > 10 && tr <= 20) {
		total += 1
	}

	f := stock.FloatMarketCap
	if f >= 50 && f <= 200 {
		total += 2
	} else if (f >= 20 && f < 50) || (f > 200 && f <= 500) {
		total += 1
	}

	a := stock.Amplitude
	if a >= 3 && a <= 8 {
		total += 2
	} else if (a >= 1 && a < 3) || (a > 8 && a <= 12) {
		total += 1
	}

	capped := total
	if capped > 7 {
		capped = 7
	}
	risk := 6 - total/2
	if risk < 1 {
		risk = 1
	}
	if risk > 5 {
		risk = 5
	}

	signals := fmt.Sprintf("涨%.2f%% 量比%.2f 换手%.2f%% 流通%.0f亿", stock.ChangePct, stock.VolumeRatio, stock.TurnoverRate, stock.FloatMarketCap)

	return &AnalysisResult{
		BuyScore:    capped,
		BuyReason:   "AI 服务异常,规则兜底评分",
		TailScore:   capped,
		TailReason:  "AI 服务异常,规则兜底评分",
		KeySignals:  signals,
		RiskLevel:   risk,
		TrapWarning: "",
	}
}
```

- [ ] **Step 5.4: 跑测试确认通过**

```bash
cd server
go test ./internal/service/ -run Test_fallbackScore_bucketing -v
```

期望:PASS。如果 `mid` 用例失败,检查 risk 公式:total=5,6-5/2=6-2=4,clamp 到 4。

- [ ] **Step 5.5: 提交**

```bash
git add server/internal/service/stock_analyzer.go server/internal/service/stock_analyzer_test.go
git commit -m "feat(stock): add rule-based fallback scorer"
```

---

## Task 6: `Analyze()` 三级容错链

**Files:**
- Modify: `server/internal/service/stock_analyzer.go:125-181`
- Modify: `server/internal/service/stock_analyzer_test.go`(追加 retry / fallback 测试)

- [ ] **Step 6.1: 先写失败测试 — retry 成功路径**

**先把 `stock_analyzer_test.go` 顶部的 imports 改为:**

```go
package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)
```

**再在文件末尾追加测试:**

```go
func Test_Analyze_retry_on_non_json(t *testing.T) {
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body string
		if calls == 1 {
			body = `{"choices":[{"message":{"content":"抱歉,我无法判断。"}}]}`
		} else {
			body = `{"choices":[{"message":{"content":"{\"buy_score\":8,\"buy_reason\":\"放量突破\",\"tail_score\":7,\"tail_reason\":\"尾盘可入\",\"key_signals\":\"量价齐升\",\"risk_level\":2,\"trap_warning\":\"\"}"}}]}`
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer ts.Close()

	a := NewStockAnalyzer("test-key", ts.URL, "test-model")
	res, err := a.Analyze(context.Background(), StockInfo{Code: "600519", Name: "贵州茅台", ChangePct: 3.0})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if res.BuyScore != 8 {
		t.Errorf("want buy_score 8 from retry, got %d", res.BuyScore)
	}
	if calls != 2 {
		t.Errorf("want 2 calls (original + retry), got %d", calls)
	}
	if res.IsFallback {
		t.Error("retry success should not be marked as fallback")
	}
}

func Test_Analyze_fallback_on_double_failure(t *testing.T) {
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"两次都不是 JSON"}}]}`))
	}))
	defer ts.Close()

	a := NewStockAnalyzer("test-key", ts.URL, "test-model")
	res, err := a.Analyze(context.Background(), StockInfo{
		Code: "000001", Name: "平安银行",
		ChangePct: 4.0, VolumeRatio: 1.8, TurnoverRate: 7.5,
		FloatMarketCap: 100, Amplitude: 5.0,
	})
	if err != nil {
		t.Fatalf("Analyze should fall back, got err: %v", err)
	}
	if !res.IsFallback {
		t.Error("expected IsFallback=true after double failure")
	}
	if res.BuyScore != 7 {
		t.Errorf("expected cap-7 fallback, got %d", res.BuyScore)
	}
	if calls != 2 {
		t.Errorf("expected exactly 2 AI calls before fallback, got %d", calls)
	}
}
```

⚠️ 这里测试假设 `AnalysisResult` 加了 `IsFallback bool` 字段(下一步补)。

- [ ] **Step 6.2: 给 `AnalysisResult` 加 `IsFallback` 字段**

在 `stock_analyzer.go:15-23` 的 `AnalysisResult` 末尾加一行:

```go
type AnalysisResult struct {
	BuyScore    int    `json:"buy_score"`
	BuyReason   string `json:"buy_reason"`
	TailScore   int    `json:"tail_score"`
	TailReason  string `json:"tail_reason"`
	KeySignals  string `json:"key_signals"`
	RiskLevel   int    `json:"risk_level"`
	TrapWarning string `json:"trap_warning"`
	IsFallback  bool   `json:"is_fallback"`
}
```

- [ ] **Step 6.3: 跑测试确认失败**

```bash
cd server
go test ./internal/service/ -run Test_Analyze -v
```

期望:FAIL(`Analyze` 当前没有 retry / fallback 逻辑)。

- [ ] **Step 6.4: 改 `Analyze` 实现三级容错链**

把 `stock_analyzer.go:125-181` 的 `Analyze` 整个替换为:

```go
func (a *StockAnalyzer) Analyze(ctx context.Context, stock StockInfo) (*AnalysisResult, error) {
	// Level 1
	res, err := a.callOnce(ctx, stock, "")
	if err == nil {
		return res, nil
	}
	log.Printf("[StockAnalyzer] %s(%s) L1 failed: %v, retrying", stock.Name, stock.Code, err)

	// Level 2: 仅对 parse 失败 / 非 4xx 的错误重试
	if isNonRetryable(err) {
		log.Printf("[StockAnalyzer] %s(%s) non-retryable, falling back", stock.Name, stock.Code)
	} else {
		res, err = a.callOnce(ctx, stock, "\n\n⚠️ 上一次返回的不是合法 JSON,请严格只输出 JSON 对象,不要任何前后缀。")
		if err == nil {
			return res, nil
		}
		log.Printf("[StockAnalyzer] %s(%s) L2 failed: %v, falling back", stock.Name, stock.Code, err)
	}

	// Level 3
	fb := fallbackScore(stock)
	fb.IsFallback = true
	return fb, nil
}

func isNonRetryable(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "returned 4") // HTTP 4xx
}

func (a *StockAnalyzer) callOnce(ctx context.Context, stock StockInfo, extraUserSuffix string) (*AnalysisResult, error) {
	prompt := buildAnalysisPrompt(stock) + extraUserSuffix

	reqBody := chatRequest{
		Model: a.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
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
```

- [ ] **Step 6.5: 跑测试确认通过**

```bash
cd server
go test ./internal/service/ -run Test_Analyze -v
go test ./internal/service/ -run TestParseAnalysisResult -v
go test ./internal/service/ -run Test_fallbackScore -v
```

期望:全部 PASS。

- [ ] **Step 6.6: 把 `IsFallback` 透传到 cron 与 handler**

Modify `server/internal/cron/stock_cron.go:131-143`,在 `rec := &model.StockRecommendation{...}` 块中追加一行 `IsFallback: result.IsFallback,`:

```go
rec := &model.StockRecommendation{
    StockCode:    stock.Code,
    StockName:    stock.Name,
    Source:       source,
    BuyScore:     result.BuyScore,
    BuyReason:    result.BuyReason,
    TailScore:    result.TailScore,
    TailReason:   result.TailReason,
    KeySignals:   result.KeySignals,
    RiskLevel:    result.RiskLevel,
    TrapWarning:  result.TrapWarning,
    IsFallback:   result.IsFallback,
    AnalysisDate: today,
}
```

Modify `server/internal/handler/stock.go:82-94`,同样在 `rec := &model.StockRecommendation{...}` 中追加 `IsFallback: result.IsFallback,`。

- [ ] **Step 6.7: 编译 + 跑全部单测**

```bash
cd server
go build ./...
go test ./... -count=1
```

期望:全部 PASS(`TestStockStore_*` 无 DSN 时 skip)。

- [ ] **Step 6.8: 提交**

```bash
git add server/internal/service/stock_analyzer.go server/internal/service/stock_analyzer_test.go server/internal/cron/stock_cron.go server/internal/handler/stock.go
git commit -m "feat(stock): add three-level AI fallback chain with retry and rule scorer"
```

---

## Task 7: `dedupByCode` 工具函数

**Files:**
- Create: `server/internal/cron/stock_cron_test.go`
- Modify: `server/internal/cron/stock_cron.go`(函数追加到文件末尾)

- [ ] **Step 7.1: 先写失败测试**

新建 `server/internal/cron/stock_cron_test.go`:

```go
package cron

import (
	"testing"

	"github.com/mtiano/server/internal/service"
)

func Test_dedupByCode_priority(t *testing.T) {
	hot := []service.StockInfo{
		{Code: "600519", Name: "贵州茅台-hot"},
		{Code: "000001", Name: "平安银行-hot"},
	}
	gainers := []service.StockInfo{
		{Code: "000001", Name: "平安银行-gainers"}, // 重复
		{Code: "300750", Name: "宁德时代-gainers"},
	}
	active := []service.StockInfo{
		{Code: "600519", Name: "贵州茅台-active"}, // 重复
		{Code: "002594", Name: "比亚迪-active"},
	}

	merged := dedupByCode(hot, gainers, active)
	if len(merged) != 4 {
		t.Fatalf("want 4 unique codes, got %d", len(merged))
	}

	byCode := map[string]string{}
	for _, s := range merged {
		byCode[s.Code] = s.Name
	}
	if byCode["600519"] != "贵州茅台-hot" {
		t.Errorf("600519 should keep hot version, got %q", byCode["600519"])
	}
	if byCode["000001"] != "平安银行-hot" {
		t.Errorf("000001 should keep hot version, got %q", byCode["000001"])
	}
	if byCode["300750"] != "宁德时代-gainers" {
		t.Errorf("300750 should come from gainers, got %q", byCode["300750"])
	}
	if byCode["002594"] != "比亚迪-active" {
		t.Errorf("002594 should come from active, got %q", byCode["002594"])
	}
}
```

- [ ] **Step 7.2: 跑测试确认失败**

```bash
cd server
go test ./internal/cron/ -run Test_dedupByCode_priority -v
```

期望:FAIL,`dedupByCode` undefined。

- [ ] **Step 7.3: 实现 `dedupByCode`**

追加到 `server/internal/cron/stock_cron.go` 文件末尾:

```go
// dedupByCode 按参数顺序合并多个榜单,按 code 去重,保留首次出现项(靠前榜单优先)。
func dedupByCode(lists ...[]service.StockInfo) []service.StockInfo {
	seen := map[string]struct{}{}
	var merged []service.StockInfo
	for _, list := range lists {
		for _, s := range list {
			if _, ok := seen[s.Code]; ok {
				continue
			}
			seen[s.Code] = struct{}{}
			merged = append(merged, s)
		}
	}
	return merged
}
```

- [ ] **Step 7.4: 跑测试确认通过**

```bash
cd server
go test ./internal/cron/ -run Test_dedupByCode_priority -v
```

期望:PASS。

- [ ] **Step 7.5: 提交**

```bash
git add server/internal/cron/stock_cron.go server/internal/cron/stock_cron_test.go
git commit -m "feat(stock): add dedupByCode helper for multi-source merge"
```

---

## Task 8: 放宽 `preFilterStocks` + ST 过滤

**Files:**
- Modify: `server/internal/cron/stock_cron.go:97-118`
- Modify: `server/internal/cron/stock_cron_test.go`

- [ ] **Step 8.1: 先写失败测试**

追加到 `stock_cron_test.go`:

```go
func Test_preFilterStocks_relaxed(t *testing.T) {
	sc := &StockCron{}

	cases := []struct {
		name   string
		stock  service.StockInfo
		passed bool
	}{
		{"涨幅边界低(0.5%)通过", service.StockInfo{Code: "1", Name: "A", ChangePct: 0.5, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: 20}, true},
		{"涨幅边界高(9.5%)通过", service.StockInfo{Code: "2", Name: "B", ChangePct: 9.5, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: 20}, true},
		{"涨幅超界(10%)刷掉", service.StockInfo{Code: "3", Name: "C", ChangePct: 10, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: 20}, false},
		{"PE 负值通过(放弃硬刷)", service.StockInfo{Code: "4", Name: "D", ChangePct: 3, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: -50}, true},
		{"流通 20 亿通过", service.StockInfo{Code: "5", Name: "E", ChangePct: 3, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 20, PERatio: 20}, true},
		{"流通 500 亿通过", service.StockInfo{Code: "6", Name: "F", ChangePct: 3, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 500, PERatio: 20}, true},
		{"ST 股名直接刷掉", service.StockInfo{Code: "7", Name: "ST 康美", ChangePct: 3, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: 20}, false},
		{"*ST 股名直接刷掉", service.StockInfo{Code: "8", Name: "*ST 海马", ChangePct: 3, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: 20}, false},
		{"退市股直接刷掉", service.StockInfo{Code: "9", Name: "XX退", ChangePct: 3, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: 20}, false},
		{"字段缺失(换手率 0)通过", service.StockInfo{Code: "10", Name: "G", ChangePct: 3, TurnoverRate: 0, VolumeRatio: 0, FloatMarketCap: 0, PERatio: 0}, true},
	}

	for _, c := range cases {
		got := sc.preFilterStocks([]service.StockInfo{c.stock})
		passed := len(got) == 1
		if passed != c.passed {
			t.Errorf("%s: want passed=%v, got %v", c.name, c.passed, passed)
		}
	}
}
```

- [ ] **Step 8.2: 跑测试确认(部分)失败**

```bash
cd server
go test ./internal/cron/ -run Test_preFilterStocks_relaxed -v
```

期望:FAIL 若干条(涨幅 0.5/9.5、PE 负、ST 过滤 尚未实现)。

- [ ] **Step 8.3: 替换 `preFilterStocks` 实现**

在 `stock_cron.go:97-118` 把 `preFilterStocks` 整个替换为:

```go
func (sc *StockCron) preFilterStocks(stocks []service.StockInfo) []service.StockInfo {
	var passed []service.StockInfo
	for _, s := range stocks {
		// ST / 退市 股直接刷掉
		if strings.Contains(s.Name, "ST") || strings.Contains(s.Name, "退") {
			continue
		}
		if s.ChangePct < 0.5 || s.ChangePct > 9.5 {
			continue
		}
		if s.TurnoverRate > 0 && (s.TurnoverRate < 2 || s.TurnoverRate > 20) {
			continue
		}
		if s.VolumeRatio > 0 && s.VolumeRatio < 0.6 {
			continue
		}
		if s.FloatMarketCap > 0 && (s.FloatMarketCap < 20 || s.FloatMarketCap > 500) {
			continue
		}
		if s.PERatio < -1000 || s.PERatio > 300 {
			continue
		}
		passed = append(passed, s)
	}
	return passed
}
```

同文件顶部 import 追加 `"strings"`(若 imports 里还没有)。

- [ ] **Step 8.4: 跑测试确认通过**

```bash
cd server
go test ./internal/cron/ -v
```

期望:全部 PASS。

- [ ] **Step 8.5: 提交**

```bash
git add server/internal/cron/stock_cron.go server/internal/cron/stock_cron_test.go
git commit -m "feat(stock): relax pre-filter and add ST exclusion"
```

---

## Task 9: `analyzeHotStocks` 合并三榜

**Files:**
- Modify: `server/internal/cron/stock_cron.go:72-95`
- Modify: `server/internal/cron/stock_cron_test.go`

- [ ] **Step 9.1: 先写失败测试 — 任一榜单失败时其它榜仍能进候选池**

**先把 `stock_cron_test.go` 顶部 imports 扩展为:**

```go
package cron

import (
	"fmt"
	"testing"

	"github.com/mtiano/server/internal/service"
)
```

**再在文件末尾追加测试:**

```go
var errBoom = fmt.Errorf("boom")

// Test_fetchCandidatePool_singleSourceFailure 验证三榜中两路失败,剩余一路仍能贡献候选
func Test_fetchCandidatePool_singleSourceFailure(t *testing.T) {
	hot := []service.StockInfo{{Code: "600519", Name: "贵州茅台"}}
	var gainersErr, activeErr error = errBoom, errBoom
	var gainersList, activeList []service.StockInfo

	merged := mergeCandidates(hot, nil, gainersList, gainersErr, activeList, activeErr)
	if len(merged) != 1 || merged[0].Code != "600519" {
		t.Fatalf("expected 1 candidate from hot-only, got %+v", merged)
	}
}
```

- [ ] **Step 9.2: 跑测试确认失败**

```bash
cd server
go test ./internal/cron/ -run Test_fetchCandidatePool_singleSourceFailure -v
```

期望:FAIL,`mergeCandidates` undefined。

- [ ] **Step 9.3: 实现 `mergeCandidates` 辅助函数**

追加到 `stock_cron.go` 文件末尾:

```go
// mergeCandidates 合并三榜,任一路的 err 非 nil 时跳过该路,返回去重后的候选池。
func mergeCandidates(hot []service.StockInfo, hotErr error, gainers []service.StockInfo, gainersErr error, active []service.StockInfo, activeErr error) []service.StockInfo {
	var lists [][]service.StockInfo
	if hotErr == nil && len(hot) > 0 {
		lists = append(lists, hot)
	}
	if gainersErr == nil && len(gainers) > 0 {
		lists = append(lists, gainers)
	}
	if activeErr == nil && len(active) > 0 {
		lists = append(lists, active)
	}
	return dedupByCode(lists...)
}
```

- [ ] **Step 9.4: 跑测试确认通过**

```bash
cd server
go test ./internal/cron/ -run Test_fetchCandidatePool_singleSourceFailure -v
```

期望:PASS。

- [ ] **Step 9.5: 重写 `analyzeHotStocks` 使用三榜 + 合并**

把 `stock_cron.go:72-95` 的 `analyzeHotStocks` 整个替换为:

```go
func (sc *StockCron) analyzeHotStocks() {
	log.Println("[Cron] starting hot stock analysis (3-source pool)")

	hot, hotErr := sc.stockData.GetHotStocks(20)
	if hotErr != nil {
		log.Printf("[Cron] fetch hot failed: %v", hotErr)
	}
	gainers, gainersErr := sc.stockData.GetGainers(20)
	if gainersErr != nil {
		log.Printf("[Cron] fetch gainers failed: %v", gainersErr)
	}
	active, activeErr := sc.stockData.GetActive(20)
	if activeErr != nil {
		log.Printf("[Cron] fetch active failed: %v", activeErr)
	}

	merged := mergeCandidates(hot, hotErr, gainers, gainersErr, active, activeErr)
	if len(merged) == 0 {
		log.Println("[Cron] all three sources failed, aborting hot analysis")
		return
	}

	filtered := sc.preFilterStocks(merged)
	log.Printf("[Cron] candidate pool: hot=%d gainers=%d active=%d merged=%d filtered=%d",
		len(hot), len(gainers), len(active), len(merged), len(filtered))

	if len(filtered) == 0 {
		log.Println("[Cron] no candidates passed pre-filter, analyzing top 5 of merged pool")
		if len(merged) > 5 {
			merged = merged[:5]
		}
		sc.analyzeAndStore(merged, "hot")
		return
	}

	if len(filtered) > 20 {
		filtered = filtered[:20]
	}
	sc.analyzeAndStore(filtered, "hot")
}
```

- [ ] **Step 9.6: 编译 + 跑全部单测**

```bash
cd server
go build ./...
go test ./... -count=1
```

期望:全部 PASS。

- [ ] **Step 9.7: 提交**

```bash
git add server/internal/cron/stock_cron.go server/internal/cron/stock_cron_test.go
git commit -m "feat(stock): analyze hot stocks from 3-source merged pool"
```

---

## Task 10: 冒烟验证 + 验收

**Files:** 无代码改动,仅手动验证 + log 记录。

- [ ] **Step 10.1: 启动 Python 微服务**

```bash
cd server/stock_service
source venv/bin/activate 2>/dev/null || true
python app.py
```

新开一个终端确认三个 endpoint 都可用:

```bash
curl -s 'http://localhost:5001/api/stock/hot?count=3' | python -m json.tool
curl -s 'http://localhost:5001/api/stock/gainers?count=3' | python -m json.tool
curl -s 'http://localhost:5001/api/stock/active?count=3' | python -m json.tool
```

- [ ] **Step 10.2: 启动 Go 后端**

```bash
cd server
go run cmd/server/main.go
```

观察启动日志确认 `is_fallback` migration 被执行(没有"duplicate column"报错即可)。

- [ ] **Step 10.3: 在开盘时段手动触发一次 hot 分析**

⚠️ cron 调度是工作日 9:30/10:30/11:30/13:00/14:00/14:30,若当前不在时间窗内,在 `main.go` 里临时加一行 `go stockCron.analyzeHotStocks()` 触发一次,**验证完后 revert 这行再部署**。

观察 Go 日志,期望出现:

```
[Cron] starting hot stock analysis (3-source pool)
[Cron] candidate pool: hot=20 gainers=20 active=20 merged=40-55 filtered=≥15
[StockAnalyzer] <stock>(<code>) AI response: {...}
```

- [ ] **Step 10.4: 查询 DB 验证验收标准**

```sql
SELECT COUNT(*) FROM stock_recommendations
  WHERE analysis_date = CURDATE() AND source = 'hot' AND buy_score >= 8;
```

期望 **≥ 3**(开盘时段)。

如果未达标,按顺序排查:
1. `SELECT COUNT(*) FROM stock_recommendations WHERE analysis_date = CURDATE() AND source='hot'` — 是否有足够样本(期望 15-20)
2. `SELECT buy_score, COUNT(*) FROM ... GROUP BY buy_score` — 看分数分布
3. `SELECT COUNT(*) FROM ... WHERE is_fallback = 1` — 有多少被 AI 拒答走了 fallback
4. 根据分布定位是候选池小了、还是 prompt 太严、还是 AI 拒答率高

- [ ] **Step 10.5: 非开盘时段回归**

晚上或周末再次手动触发 `analyzeHotStocks()`,期望:
- 链路不崩(字段大量为 0)
- 至少有记录入库(允许 `is_fallback=1`)

- [ ] **Step 10.6: AI 挂掉回归**

临时在 `config.yaml` 改 AI `api_key` 为错误值重启,触发一次分析,期望:
- 所有股票记录仍入库,`is_fallback=1`
- log 有 `L1 failed ... non-retryable ... falling back`(因为 4xx)

完成后恢复 `config.yaml`。

- [ ] **Step 10.7: 总结 + 关闭**

在 commit message 或 PR 描述里写下 Step 10.4 查询到的实际 ≥8 分条数,确认达标后关闭此计划。无需额外 commit。

---

## 执行约束

1. **顺序依赖:** Task 1 → Task 2 & 3(并行)→ Task 4 → Task 5 → Task 6(依赖 5)→ Task 7 → Task 8 → Task 9(依赖 7、8)→ Task 10。
2. **单测必须全绿才能进下一个 Task**(`go test ./... -count=1` 除 `TEST_MYSQL_DSN` skip 外应全 PASS)。
3. **每个 Task 结束必须 commit**,按 TDD 节奏(测试 → 跑红 → 实现 → 跑绿 → commit)。
4. **不动前端:** 本计划不包含 `miniprogram/**` 改动,"兜底" 角标延后观察一两天再做。
