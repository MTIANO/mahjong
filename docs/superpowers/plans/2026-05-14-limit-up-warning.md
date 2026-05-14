# 涨停股流动性警告 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让推荐列表中"开盘一字涨停"和"盘中已涨停"的股票自动带上流动性风险警告(trap_warning),buy_score / tail_score 完全不变。

**Architecture:** 新增 `LimitUpWarning(stock)` 纯函数 + `annotateLimitUp(result, stock)` helper,在 `StockAnalyzer.Analyze()` 三个 return 路径(L1 / L2 / L3 fallback)统一注入,前端通过 trap_warning 内容差异化呈现。

**Tech Stack:** Go 1.x + testing(单测无外部依赖)

**Spec 引用:** `docs/superpowers/specs/2026-05-14-limit-up-warning-design.md`

---

## 文件结构

**新增:**
| 文件 | 职责 |
|---|---|
| `server/internal/service/limit_up.go` | `LimitUpWarning(stock)` 工具函数 + `annotateLimitUp(result, stock)` helper |
| `server/internal/service/limit_up_test.go` | 11 个表格驱动单测覆盖板块阈值、开盘一字、边界值、空 PrevClose 等 |

**修改:**
| 文件 | 主要改动 |
|---|---|
| `server/internal/service/stock_analyzer.go` | `Analyze()` 函数 3 处 return 改为 `return annotateLimitUp(res, stock), nil` |
| `server/internal/service/stock_analyzer_test.go` | 追加 `Test_Analyze_annotates_limit_up` 集成测试 |

---

## Task 1: `LimitUpWarning` 工具函数

**Files:**
- Create: `server/internal/service/limit_up.go`
- Create: `server/internal/service/limit_up_test.go`

- [ ] **Step 1.1: 先写表格驱动失败测试**

创建 `server/internal/service/limit_up_test.go`,完整内容:

```go
package service

import "testing"

func TestLimitUpWarning(t *testing.T) {
	cases := []struct {
		name  string
		stock StockInfo
		want  string
	}{
		{
			"主板涨停 10%",
			StockInfo{Code: "600000", ChangePct: 10.0, PrevClose: 10.0, Open: 10.5},
			"当前已涨停,追高风险",
		},
		{
			"主板涨停容差 9.95",
			StockInfo{Code: "600000", ChangePct: 9.95, PrevClose: 10.0, Open: 10.0},
			"当前已涨停,追高风险",
		},
		{
			"主板 9.94 不警告(差 0.01)",
			StockInfo{Code: "600000", ChangePct: 9.94, PrevClose: 10.0, Open: 10.0},
			"",
		},
		{
			"主板接近涨停 9.5(还差 0.45)",
			StockInfo{Code: "600000", ChangePct: 9.5, PrevClose: 10.0, Open: 10.0},
			"",
		},
		{
			"创业板涨停 20%",
			StockInfo{Code: "300001", ChangePct: 20.0, PrevClose: 10.0, Open: 10.0},
			"当前已涨停,追高风险",
		},
		{
			"创业板容差 19.95",
			StockInfo{Code: "300001", ChangePct: 19.95, PrevClose: 10.0, Open: 10.0},
			"当前已涨停,追高风险",
		},
		{
			"创业板 19.5 不警告(20% 才涨停)",
			StockInfo{Code: "300001", ChangePct: 19.5, PrevClose: 10.0, Open: 10.0},
			"",
		},
		{
			"科创板涨停 20%",
			StockInfo{Code: "688256", ChangePct: 20.0, PrevClose: 10.0, Open: 10.0},
			"当前已涨停,追高风险",
		},
		{
			"主板开盘一字涨停(优先于盘中涨停)",
			StockInfo{Code: "600000", PrevClose: 10.0, Open: 11.0, ChangePct: 10.0},
			"开盘一字涨停,流动性差买不进",
		},
		{
			"创业板开盘一字",
			StockInfo{Code: "300001", PrevClose: 10.0, Open: 12.0, ChangePct: 20.0},
			"开盘一字涨停,流动性差买不进",
		},
		{
			"普通涨幅 3%",
			StockInfo{Code: "600000", ChangePct: 3.0, PrevClose: 10.0, Open: 10.1},
			"",
		},
		{
			"跌停 -10%",
			StockInfo{Code: "600000", ChangePct: -10.0, PrevClose: 10.0, Open: 9.5},
			"",
		},
		{
			"PrevClose=0 但已涨停(只看 ChangePct)",
			StockInfo{Code: "600000", PrevClose: 0, Open: 11.0, ChangePct: 10.0},
			"当前已涨停,追高风险",
		},
	}

	for _, c := range cases {
		got := LimitUpWarning(c.stock)
		if got != c.want {
			t.Errorf("%s: want %q, got %q", c.name, c.want, got)
		}
	}
}
```

- [ ] **Step 1.2: 跑测试确认失败**

```bash
cd /Users/buchiyudan/mtiano/mahjong/server
go test ./internal/service/ -run TestLimitUpWarning -v
```

期望:FAIL,`LimitUpWarning` undefined(test 文件 build 失败)。

- [ ] **Step 1.3: 实现 `LimitUpWarning`**

创建 `server/internal/service/limit_up.go`,完整内容:

```go
package service

import "strings"

// LimitUpWarning 返回涨停股的流动性风险警告文字;非涨停股返回 ""。
//
// 涨停阈值按板块区分(留 0.05% 容差覆盖"涨停价四舍五入到分"的浮点误差):
//   - 创业板(300 开头)/ 科创板(688 开头):20% 涨停 → 阈值 19.95%
//   - 主板 / 中小板 / 其它:10% 涨停 → 阈值 9.95%
//
// 优先级:开盘一字涨停 > 盘中已涨停。
func LimitUpWarning(stock StockInfo) string {
	threshold := 9.95
	if strings.HasPrefix(stock.Code, "300") || strings.HasPrefix(stock.Code, "688") {
		threshold = 19.95
	}

	openLimitUp := stock.PrevClose > 0 &&
		(stock.Open/stock.PrevClose-1)*100 >= threshold
	nowLimitUp := stock.ChangePct >= threshold

	switch {
	case openLimitUp:
		return "开盘一字涨停,流动性差买不进"
	case nowLimitUp:
		return "当前已涨停,追高风险"
	default:
		return ""
	}
}

// annotateLimitUp 把涨停警告 prefix 到 result.TrapWarning;
// 无警告时原样返回 result。涨停警告优先级高于 AI 推断的陷阱,
// 用 "; " 分隔保留原 trap_warning(若有)。
func annotateLimitUp(result *AnalysisResult, stock StockInfo) *AnalysisResult {
	warning := LimitUpWarning(stock)
	if warning == "" {
		return result
	}
	if result.TrapWarning == "" {
		result.TrapWarning = warning
	} else {
		result.TrapWarning = warning + "; " + result.TrapWarning
	}
	return result
}
```

- [ ] **Step 1.4: 跑测试确认通过**

```bash
cd /Users/buchiyudan/mtiano/mahjong/server
go test ./internal/service/ -run TestLimitUpWarning -v
go build ./...
```

期望:`TestLimitUpWarning` PASS(13 个 sub-case 全过),`go build` 无错。

- [ ] **Step 1.5: 提交**

```bash
cd /Users/buchiyudan/mtiano/mahjong
git add server/internal/service/limit_up.go server/internal/service/limit_up_test.go
git commit -m "feat(stock): add LimitUpWarning helper for liquidity-risk annotation"
```

---

## Task 2: 在 `Analyze()` 三个 return 路径注入 + 集成测试

**Files:**
- Modify: `server/internal/service/stock_analyzer.go`(Analyze 函数 3 处 return)
- Modify: `server/internal/service/stock_analyzer_test.go`(追加 1 个集成测试)

- [ ] **Step 2.1: 先写集成失败测试**

在 `server/internal/service/stock_analyzer_test.go` 文件末尾追加:

```go
func Test_Analyze_annotates_limit_up(t *testing.T) {
	// mock AI 返回正常 8 分但空 trap_warning
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"buy_score\":8,\"buy_reason\":\"放量突破\",\"tail_score\":8,\"tail_reason\":\"尾盘强势\",\"key_signals\":\"涨停接力\",\"risk_level\":2,\"trap_warning\":\"\"}"}}]}`))
	}))
	defer ts.Close()

	a := NewStockAnalyzer("test-key", ts.URL, "test-model")

	// 主板涨停 10% 主板股
	res, err := a.Analyze(context.Background(), StockInfo{
		Code: "600000", Name: "测试主板", ChangePct: 10.0, PrevClose: 10.0, Open: 10.5,
	}, nil)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if res.BuyScore != 8 {
		t.Errorf("buy_score should not change, want 8 got %d", res.BuyScore)
	}
	if res.TrapWarning != "当前已涨停,追高风险" {
		t.Errorf("expected limit-up warning prefix, got %q", res.TrapWarning)
	}

	// AI 返回非空 trap_warning 时,涨停警告 prefix + "; " + 原 warning
	tsWithWarning := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"buy_score\":7,\"buy_reason\":\"放量\",\"tail_score\":7,\"tail_reason\":\"尾盘\",\"key_signals\":\"\",\"risk_level\":3,\"trap_warning\":\"秒拉诱多\"}"}}]}`))
	}))
	defer tsWithWarning.Close()

	a2 := NewStockAnalyzer("test-key", tsWithWarning.URL, "test-model")
	res2, err := a2.Analyze(context.Background(), StockInfo{
		Code: "300001", Name: "测试创业", ChangePct: 20.0, PrevClose: 10.0, Open: 12.0,
	}, nil)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if res2.TrapWarning != "开盘一字涨停,流动性差买不进; 秒拉诱多" {
		t.Errorf("expected merged warning, got %q", res2.TrapWarning)
	}

	// 非涨停股不应被 annotate
	res3, err := a.Analyze(context.Background(), StockInfo{
		Code: "600000", Name: "正常股", ChangePct: 3.0, PrevClose: 10.0, Open: 10.1,
	}, nil)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if res3.TrapWarning != "" {
		t.Errorf("non-limit-up should keep trap_warning empty, got %q", res3.TrapWarning)
	}
}
```

- [ ] **Step 2.2: 跑测试确认失败**

```bash
cd /Users/buchiyudan/mtiano/mahjong/server
go test ./internal/service/ -run Test_Analyze_annotates_limit_up -v
```

期望:FAIL — `res.TrapWarning` 是空字符串(因为 Analyze 还没接 annotate),case 1 报 `expected limit-up warning prefix, got ""`。

- [ ] **Step 2.3: 在 `Analyze` 三个 return 路径接入 `annotateLimitUp`**

修改 `server/internal/service/stock_analyzer.go` 的 `Analyze` 函数:

把:
```go
func (a *StockAnalyzer) Analyze(ctx context.Context, stock StockInfo, themes []string) (*AnalysisResult, error) {
	// Level 1
	res, err := a.callOnce(ctx, stock, themes, "")
	if err == nil {
		return res, nil
	}
	log.Printf("[StockAnalyzer] %s(%s) L1 failed: %v, retrying", stock.Name, stock.Code, err)

	// Level 2: 仅对 parse 失败 / 非 4xx 的错误重试
	if isNonRetryable(err) {
		log.Printf("[StockAnalyzer] %s(%s) non-retryable, falling back", stock.Name, stock.Code)
	} else {
		res, err = a.callOnce(ctx, stock, themes, "\n\n⚠️ 上一次返回的不是合法 JSON,请严格只输出 JSON 对象,不要任何前后缀。")
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
```

替换为(3 处 `return` 都用 `annotateLimitUp` 包一层):

```go
func (a *StockAnalyzer) Analyze(ctx context.Context, stock StockInfo, themes []string) (*AnalysisResult, error) {
	// Level 1
	res, err := a.callOnce(ctx, stock, themes, "")
	if err == nil {
		return annotateLimitUp(res, stock), nil
	}
	log.Printf("[StockAnalyzer] %s(%s) L1 failed: %v, retrying", stock.Name, stock.Code, err)

	// Level 2: 仅对 parse 失败 / 非 4xx 的错误重试
	if isNonRetryable(err) {
		log.Printf("[StockAnalyzer] %s(%s) non-retryable, falling back", stock.Name, stock.Code)
	} else {
		res, err = a.callOnce(ctx, stock, themes, "\n\n⚠️ 上一次返回的不是合法 JSON,请严格只输出 JSON 对象,不要任何前后缀。")
		if err == nil {
			return annotateLimitUp(res, stock), nil
		}
		log.Printf("[StockAnalyzer] %s(%s) L2 failed: %v, falling back", stock.Name, stock.Code, err)
	}

	// Level 3
	fb := fallbackScore(stock)
	fb.IsFallback = true
	return annotateLimitUp(fb, stock), nil
}
```

- [ ] **Step 2.4: 跑测试确认通过 + 全量回归**

```bash
cd /Users/buchiyudan/mtiano/mahjong/server
go test ./internal/service/ -run Test_Analyze_annotates_limit_up -v
go test ./... -count=1
```

期望:
- `Test_Analyze_annotates_limit_up` 三个 sub-case 全 PASS
- 全量测试无回归:`internal/cron`、`internal/handler`、`internal/service`、`pkg/mahjong` 全 `ok`
- `TestLimitUpWarning` 仍 PASS
- 原有 `Test_Analyze_retry_on_non_json` / `Test_Analyze_fallback_on_double_failure` / `Test_Analyze_no_retry_on_4xx` 仍 PASS(它们传的 StockInfo 都不是涨停股,annotate 不会改变结果)

- [ ] **Step 2.5: 提交**

```bash
cd /Users/buchiyudan/mtiano/mahjong
git add server/internal/service/stock_analyzer.go server/internal/service/stock_analyzer_test.go
git commit -m "feat(stock): annotate limit-up risk in Analyze return paths"
```

---

## 执行约束

1. **顺序依赖:** Task 1 → Task 2(Task 2 用到 Task 1 的 `annotateLimitUp`)
2. **TDD 严格执行:** 每个 Task 内"测试先行 → 跑红 → 实现 → 跑绿 → commit"
3. **不动其他文件:** cron / handler / config / DB schema / prompt / 前端,本次完全不碰
4. **保持 buy_score / tail_score 不变** 是核心约束(spec §1),不要顺手做"涨停股 cap 5 分"等扩展

---

## 验证(部署后,首个交易日跑完一次 cron 后)

涨停股的盘口数据(ChangePct)不在 `stock_recommendations` 表里,所以**纯 SQL 无法 join 验证**。改成"日志侧 + DB 侧"两步:

```bash
# 1. 从行情看板挑一只当日涨停股,假设代码是 <CODE>;
#    从 Go 日志看 AI 看到的盘口 + 入库后的 trap_warning
grep -E "<CODE>" /www/wwwlogs/go/mahjong.log | tail -20

mysql -h 8.134.104.226 -u mtian_o -p'PBFHBNYrABBnd37C' mtian_o -e \
"SELECT stock_code, stock_name, buy_score, tail_score, trap_warning
 FROM stock_recommendations
 WHERE stock_code = '<CODE>' AND analysis_date = CURDATE();"
```

# 2. 整体上看,所有 buy_score >= 7 的票里,trap_warning 包含"涨停"的比例
mysql -h ... -e \
"SELECT
   SUM(trap_warning LIKE '%涨停%') AS limit_up_tagged,
   COUNT(*) AS total_high_score
 FROM stock_recommendations
 WHERE analysis_date = CURDATE() AND buy_score >= 7;"

**成功标准:**
- 当日已涨停股的 `trap_warning` **包含**"开盘一字涨停"或"当前已涨停"开头
- 涨停股 `buy_score` / `tail_score` **保持不变**(与改动前同等盘口数据的票分数一致,不应因本次改动下降)
- 非涨停股 `trap_warning` 不被影响(原 AI 写的内容不变,或仍为空)
