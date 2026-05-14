# 涨停股流动性警告设计

**日期:** 2026-05-14
**状态:** 待 review
**目标:** 让推荐列表中涨停板的股票自动带上"流动性差买不进 / 追高风险"的 `trap_warning`,前端可据此做差异化呈现;**buy_score / tail_score 保持不变**(用户决策:用户希望靠 trap_warning 文字区分,不破坏 AI 主评分)。

---

## 1. 背景

A 股短线策略"尾盘买入次日早盘卖出"对涨停板股票有两个隐性问题:

1. **开盘一字涨停** — 全天没有买点,流动性差,即便 AI 打 9 分也买不进
2. **盘中/尾盘涨停** — 追高接力风险大,可能次日开盘破板回落

当前 AI 评分逻辑(prompt 里"接近日内高点 → 9-10 分加分项")会**主动把涨停股推到 9 分**,而 trap_warning 字段不一定包含流动性风险(取决于 AI 自觉)。需要一个**确定性的后置规则**给涨停股贴上警告标签。

---

## 2. 方案 A:`Analyze()` 出口统一注入

**核心思路:** AI 评分 / 规则 fallback 都通过 `Analyze` 返回结果,在 `Analyze` 函数 3 个 return 路径(L1 成功 / L2 retry 成功 / L3 fallback)统一调用一个 helper,把涨停警告 **prefix 到 `result.TrapWarning`**。

### 2.1 新增工具函数(`server/internal/service/limit_up.go`)

```go
package service

import "strings"

// LimitUpWarning 返回涨停股的流动性风险警告文字;
// 非涨停股返回 ""。
//
// 涨停阈值按板块区分:
//   - 创业板(300 开头)/ 科创板(688 开头):20% 涨停 → 阈值 19.95%
//   - 主板 / 中小板 / 其它:10% 涨停 → 阈值 9.95%
//
// 阈值留 0.05% 容差,覆盖交易所"涨停价四舍五入到分"导致的 ChangePct
// 实际落在 9.99%-10.00% 的浮点误差;9.5% 这种"还差 0.5%"的非涨停股不会被误报。
//
// 区分两种状态(开盘一字优先,因为流动性更差):
//   - 开盘一字涨停:`Open/PrevClose - 1 ≥ 阈值` → "开盘一字涨停,流动性差买不进"
//   - 盘中已涨停:`ChangePct ≥ 阈值` → "当前已涨停,追高风险"
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
```

### 2.2 注入 helper(同文件)

```go
// annotateLimitUp 把涨停警告 prefix 到 result.TrapWarning 前;
// result.TrapWarning 原本为空则直接赋值,非空则用 "; " 分隔保留原警告。
// 涨停警告放在前面是因为它属于"硬性流动性风险",优先级高于 AI 推断的陷阱。
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

### 2.3 在 `Analyze` 三个 return 路径调用

`server/internal/service/stock_analyzer.go` 的 `Analyze` 函数:

```go
func (a *StockAnalyzer) Analyze(ctx context.Context, stock StockInfo, themes []string) (*AnalysisResult, error) {
    // Level 1
    res, err := a.callOnce(ctx, stock, themes, "")
    if err == nil {
        return annotateLimitUp(res, stock), nil   // ← 新增 annotate
    }
    log.Printf(...)

    // Level 2
    if isNonRetryable(err) {
        log.Printf(...)
    } else {
        res, err = a.callOnce(ctx, stock, themes, "...")
        if err == nil {
            return annotateLimitUp(res, stock), nil   // ← 新增 annotate
        }
        log.Printf(...)
    }

    // Level 3
    fb := fallbackScore(stock)
    fb.IsFallback = true
    return annotateLimitUp(fb, stock), nil   // ← 新增 annotate
}
```

**改动量:** 1 个新文件 + `Analyze` 函数 3 处 + 单测。

---

## 3. 失败模式与边界

| 场景 | 处理 |
|---|---|
| `PrevClose == 0`(数据缺失) | 跳过"开盘一字"检查,只看 `ChangePct` |
| `ChangePct = 9.0%`(接近但未到涨停) | 不警告(可控,9.5 留了 0.5% 容差) |
| 北交所 `8` 开头(30% 涨停) | 当前不在候选池(成交额榜过滤掉了),无需处理 |
| ST 股(5% 涨停) | pre-filter 阶段已经全部刷掉,不会进 Analyze |
| 跌停板 | 不警告(跌停反而是反弹机会的潜在信号,不属于本次策略) |
| AI 原本写了 `trap_warning`(如"秒拉诱多") | 涨停警告 prefix 在前,用 "; " 连接保留原文 |

---

## 4. 验证

### 4.1 单元测试(`limit_up_test.go`,新文件)

| 用例 | 输入 | 期望 |
|---|---|---|
| 主板涨停 10% | `Code:600000, ChangePct:10.0` | `"当前已涨停,追高风险"` |
| 主板接近涨停 9.5(还差 0.45%) | `Code:600000, ChangePct:9.5` | `""` |
| 主板涨停容差 9.95 | `Code:600000, ChangePct:9.95` | `"当前已涨停,追高风险"` |
| 主板 9.94(差 0.01) | `Code:600000, ChangePct:9.94` | `""` |
| 创业板涨停 20% | `Code:300001, ChangePct:20.0` | `"当前已涨停,追高风险"` |
| 创业板 19.5(还差 0.45) | `Code:300001, ChangePct:19.5` | `""`(20% 才涨停) |
| 创业板容差 19.95 | `Code:300001, ChangePct:19.95` | `"当前已涨停,追高风险"` |
| 科创板涨停 20% | `Code:688256, ChangePct:20.0` | `"当前已涨停,追高风险"` |
| 主板开盘一字涨停 | `Code:600000, PrevClose:10, Open:11.0, ChangePct:10.0` | `"开盘一字涨停,流动性差买不进"` |
| 创业板开盘一字 | `Code:300001, PrevClose:10, Open:12.0, ChangePct:20.0` | `"开盘一字涨停,流动性差买不进"` |
| 普通涨幅 3% | `ChangePct:3.0` | `""` |
| 跌停 -10% | `ChangePct:-10.0` | `""` |
| PrevClose=0 但已涨停 | `Code:600000, PrevClose:0, ChangePct:10.0` | `"当前已涨停,追高风险"`(只看 ChangePct) |

### 4.2 集成测试(扩展 `stock_analyzer_test.go`)

在现有 3 个 `Test_Analyze_*` 测试基础上加 1 个:

`Test_Analyze_annotates_limit_up` — mock AI 返回正常 8 分 + 空 trap_warning,输入一只 ChangePct=10 的主板股,验证最终 `result.TrapWarning == "当前已涨停,追高风险"`,且 `result.BuyScore` 仍是 8(分数不变)。

### 4.3 生产验收

部署后第一个交易日观察:
- 当日有涨停股进入推荐列表(buy_score ≥ 7)的话,`trap_warning` 字段必须包含 "涨停" / "追高" 关键字
- 通过 SQL 验证:
  ```sql
  SELECT stock_code, stock_name, buy_score, trap_warning
  FROM stock_recommendations
  WHERE analysis_date = CURDATE()
    AND buy_score >= 7
    AND trap_warning LIKE '%涨停%';
  ```

---

## 5. 兼容性

- ✅ DB schema 不变(trap_warning 一直存在)
- ✅ 前端不强制改动(用户已有 trap_warning 显示;后续可选地加图标差异化)
- ✅ AI prompt 完全不动
- ✅ 规则 fallback 路径也会带上 warning(annotateLimitUp 在三个 return 路径统一执行)
- ✅ 调用方(cron / handler)零改动

---

## 6. 不在本次范围

- 不修改 buy_score / tail_score(用户决策)
- 不针对"开盘涨停"做单独的硬性 filter(如果需要可以后续做)
- 不识别"涨停打开"或"跌停打开"(需要分时数据,当前不可得)
- 不识别北交所 30% 涨停(候选池里几乎不会出现)
- 不做"前端按 trap_warning 内容差异化呈现"(留给前端独立决定)
