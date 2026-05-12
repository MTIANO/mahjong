# 股票推荐评分与候选池优化设计

**日期:** 2026-05-12
**状态:** 待 review
**目标:** 每次批量跑 cron 后,`buy_score ≥ 8` 的推荐条数稳定达到 3–5 条,同时修复"某些股票重复执行后记录不生成"的问题。

---

## 1. 现状与问题

### 1.1 链路快照
- **数据层:** Python 微服务 `server/stock_service/app.py`
  - `/api/stock/hot` 走新浪 `Market_Center`,按成交额降序
  - `/api/stock/detail` 走腾讯 `qt.gtimg.cn`,拉实时行情字段
- **调度层:** `server/internal/cron/stock_cron.go` 工作日 9:30 / 10:30 / 11:30 / 13:00 / 14:00 / 14:30 各执行一次 `analyzeWatchlistStocks` + `analyzeHotStocks`
- **筛选层:** `preFilterStocks` 对热榜做硬过滤(涨幅 1–7%、换手 3–15%、量比 ≥0.8、流通市值 30–300 亿、PE 0–200)
- **评分层:** `stock_analyzer.go` 把通过筛的股票逐条送大模型,按 `systemPrompt` 中 1–10 刻度打分
- **存储层:** `stock_store.go` 以 `(stock_code, analysis_date)` 为唯一键 upsert 到 `stock_recommendations` 表

### 1.2 三类问题
1. **分数偏低**:`systemPrompt` 要求"三层共振"(盘口 + 题材 + 量价)才给 8–10 分,实际合格短线票通常只能拿到 4–6 分,用户看到的推荐缺少可入场标的。
2. **推荐记录丢失**:AI 偶尔返回自然语言(例如"数据不足,无法判断"),`parseAnalysisResult` 解析失败 → `Analyze()` 返回 error → `UpsertRecommendation` 跳过,用户列表里就看不到这只股票。
3. **候选池太窄**:只用成交额 Top30,偏向大盘蓝筹,短线爆点票难进入候选;pre-filter 再砍一刀,送到 AI 的通常只有 6–10 只。

---

## 2. 方案概览

选用方案 C(多源候选池 + 放宽筛选 + Prompt 重标定 + AI 容错重试 + 规则 fallback)。改动覆盖 5 个模块:

| 模块 | 文件 | 改动类型 |
|---|---|---|
| 数据层 | `stock_service/app.py` | 新增 2 条 endpoint,抽通用函数 |
| 调度层 | `stock_data.go`, `stock_cron.go` | 新增 2 个拉数据方法,重写候选池拼装,放宽 pre-filter |
| 评分层 | `stock_analyzer.go` | 重写 `systemPrompt`、简化 `buildAnalysisPrompt`、实现三级容错链与 `fallbackScore` |
| 存储层 | `stock_recommendations` 表、`stock_store.go`、`model/recommendation.go` | 新增 `is_fallback` 列,查询与写入透传 |
| 前端 | `miniprogram/pages/stock/*` | 可选,卡片上展示"兜底"角标(延后) |

---

## 3. 详细设计

### 3.1 数据层:Python 侧候选池多源化

**目标:** 新浪 `Market_Center` 同一接口换 `sort` 参数即可拿不同榜单,复用现有解析逻辑。

**抽通用函数** `fetch_sina(count, sort, extra_filter=None)`,替代现 `fetch_hot_sina`。三个路由都调它:

| Endpoint | sort 参数 | 附加过滤 |
|---|---|---|
| `/api/stock/hot?count=N`(已有) | `amount` | 无 |
| `/api/stock/gainers?count=N`(新增) | `changepercent` | `changepercent > 0` |
| `/api/stock/active?count=N`(新增) | `turnoverratio` | 无 |

**错误处理:** 任一路由失败返回 500 + 空 `stocks` 数组,异常本地 log,**不抛到 Go 侧**(Go 侧做 fallback)。

**不做的事:**
- 不引入 akshare / tushare 新依赖
- 不加题材榜(免费接口拿不到可靠数据,题材判断交给 prompt 里 AI 按股票名推断)

### 3.2 调度层:候选池合并 + pre-filter 放宽

**新增两个拉数据方法** 在 `server/internal/service/stock_data.go`,镜像 `GetHotStocks`:

```go
func (s *StockDataService) GetGainers(count int) ([]StockInfo, error)
func (s *StockDataService) GetActive(count int) ([]StockInfo, error)
```

**重写 `analyzeHotStocks` 候选池逻辑** 在 `stock_cron.go`:

```
hot      := GetHotStocks(20)
gainers  := GetGainers(20)
active   := GetActive(20)
merged   := dedupByCode([hot, gainers, active])   // 去重后目标 40–55
filtered := preFilterStocks(merged)               // 目标 ≥15
topN     := filtered[:min(len(filtered), 20)]     // 上限 10 → 20
analyzeAndStore(topN, "hot")
```

**关键规则:**
- 子榜失败不阻断:每榜单独 try,任一失败 log + 继续,只要 `merged` 非空就往后跑
- `dedupByCode` 按遍历顺序保留首次出现项(hot 优先,gainers 次之,active 最后),因为成交额榜最能反映主力关注度
- 送 AI 上限 10 → 20,直接翻倍高分票产出基数

**pre-filter 放宽(`preFilterStocks`):**

| 指标 | 旧值 | 新值 |
|---|---|---|
| `ChangePct` | 1 ~ 7 | **0.5 ~ 9.5** |
| `TurnoverRate`(>0 时) | 3 ~ 15 | **2 ~ 20** |
| `VolumeRatio`(>0 时) | ≥0.8 | **≥0.6** |
| `FloatMarketCap`(>0 时) | 30 ~ 300 亿 | **20 ~ 500 亿** |
| `PERatio` | 0 ~ 200 | **-1000 ~ 300** |

**新增安全边界:** 股票名包含 `ST` / `*ST` / `退` 的直接跳过(现在没做,放宽后这类票会混进来)。

### 3.3 评分层:Prompt 重标定

**重写 `systemPrompt`** 核心刻度:

```
8–10 分(强烈推荐,预期占通过 pre-filter 的 30–40%):
  盘口 + 量价两层达标 + 无明显陷阱
  加分项(满足任一冲 9–10):清晰题材催化 / 量能显著放大 / 价接近日内高点

6–7 分(可观察,占 30–40%):
  盘口达标但量价或题材有一项平淡

4–5 分(弱势,占 20%):
  信号矛盾或轻度陷阱

1–3 分(规避,占 5–10%):
  明显陷阱 / 多项硬指标不及格
```

核心变化:原 prompt "三层共振"才给 8–10,现在"两层达标 + 无陷阱"即可 —— 通过 pre-filter 已意味着盘口过关,AI 只需判断量价 + 题材 + 陷阱。

**数据缺失规则(prompt 中新增):**
```
若 turnover_rate / volume_ratio / pe_ratio / float_market_cap 为 0 或异常,
视为"数据缺失",按可得字段打分,不额外扣分;
在 key_signals 末尾附 "[数据部分缺失]"。
```

**输出契约硬约束(`buildAnalysisPrompt` 末尾新增):**
```
只输出 JSON 对象,不要前后文字、不要代码块标记。
无法判断也必须返回有效 JSON,score 填 3,trap_warning 写原因。
```

**简化分析要求:** 原"逐层分析要求"4 条列表容易被 AI 误解为要显式复述过程,改为一句话 → 更聚焦在出 JSON。

**不改的部分:**
- `AnalysisResult` 结构体字段不变(DB schema 不变、前端不改字段)
- 三大陷阱识别保留(秒拉诱多 / 缩量拉升 / 弱转强),但 system prompt 里精简陈述

### 3.4 评分层:AI 容错重试 + 规则 fallback

**三级容错链** 在 `Analyze()`:

```
Level 1: 正常调用 + parseAnalysisResult
   ↓ 非 JSON / 字段缺失 / HTTP 5xx / 超时
Level 2: Retry 一次,user prompt 末尾加
         "上一次返回的不是合法 JSON,请严格只输出 JSON 对象"
   ↓ 仍失败
Level 3: fallbackScore(),写入 DB,is_fallback=true
```

- Level 2 只重试一次(避免长尾卡死批量任务)
- HTTP 4xx(key 错 / 余额不足)直接跳 Level 3
- 每级 log 清楚,便于后续统计失败率

**规则 fallback 打分器(新增 `fallbackScore` 函数):** 基于 pre-filter 已有的 5 个指标,每项 0–2 分,合计 0–10 分。

| 指标 | 2 分 | 1 分 | 0 分 |
|---|---|---|---|
| ChangePct | 3 ~ 5% | 0.5 ~ 3% 或 5 ~ 9.5% | 其它 |
| VolumeRatio | ≥1.5 | 1.0 ~ 1.5 | <1.0 或 0 |
| TurnoverRate | 5 ~ 10% | 2 ~ 5% 或 10 ~ 20% | 其它 |
| FloatMarketCap | 50 ~ 200 亿 | 20 ~ 50 或 200 ~ 500 亿 | 其它 |
| Amplitude | 3 ~ 8% | 1 ~ 3% 或 8 ~ 12% | 其它 |

- `buy_score = tail_score = min(合计, 7)` — **硬 cap 为 7**:规则分没法识别秒拉诱多之类陷阱,不让兜底分冲进"强烈推荐"区间;用户看到 fallback 票最多是"可观察"级别,配合前端角标足够提示
- `risk_level = clamp(1, 5, 6 − 总分/2)` — 总分 10 → 1(低风险),总分 0 → 5(高风险)
- `buy_reason = "AI 服务异常,规则兜底评分"`
- `key_signals` 拼装实际指标值(便于用户复盘)
- `trap_warning` 留空

**目标:** AI 全挂时用户仍能看到推荐,不再出现"列表空 / 股票丢失"。

### 3.5 存储层:DB schema 变更

**单条迁移:**
```sql
ALTER TABLE stock_recommendations
  ADD COLUMN is_fallback TINYINT(1) NOT NULL DEFAULT 0 AFTER trap_warning;
```

**代码配套:**
- `model.StockRecommendation` 加 `IsFallback bool \`json:"is_fallback"\``
- `UpsertRecommendation` INSERT 与 `ON DUPLICATE KEY UPDATE` 都带上 `is_fallback`
- `GetTodayRecommendations` 查询带 `COALESCE(is_fallback, 0)`,scan 到 struct

### 3.6 前端(可选,延后)

在 `miniprogram/pages/stock/stock.js` 渲染时,`is_fallback === true` 的卡片右上角加灰色 "兜底" 小标签。**先上线后端,观察一两天再决定是否做。**

---

## 4. 验证

### 4.1 单元测试

**`stock_cron_test.go`(新建):**
- `Test_dedupByCode_priority` — 三榜重复 code,保留 hot 来源
- `Test_preFilterStocks_relaxed` — 构造 6 组边界数据(涨幅 0.5/9.5、PE 负、流通 20/500、ST 名),验证放宽通过率与 ST 过滤
- `Test_analyzeHotStocks_single_source_failure` — mock 两榜 error,验证剩余一榜仍能送 AI

**`stock_analyzer_test.go`(扩展现有):**
- `Test_Analyze_retry_on_non_json` — mock HTTP,首次返回非 JSON,第二次返回合法 JSON,验证结果取第二次
- `Test_Analyze_fallback_on_double_failure` — 两次都返回非 JSON,验证走 `fallbackScore`,`IsFallback=true`
- `Test_fallbackScore_bucketing` — 5 指标 × 边界值表格化验证

**`stock_store_test.go`(扩展):**
- 验证 `UpsertRecommendation` 带 `is_fallback`
- 验证 `GetTodayRecommendations` 能 scan 出该字段

### 4.2 手动冒烟

```bash
# Python 侧
curl 'http://localhost:5001/api/stock/hot?count=5'
curl 'http://localhost:5001/api/stock/gainers?count=5'
curl 'http://localhost:5001/api/stock/active?count=5'

# Go 侧触发一次 cron(临时暴露一个 admin endpoint 手动触发)
# 观察 log 流:候选池去重后 ≥30;pre-filter 后 ≥15;送 AI 20 只;分数分布
```

### 4.3 验收标准

- **开盘时段** 手动触发一次 `analyzeHotStocks`,当日 `stock_recommendations` 表里 `source='hot' AND buy_score >= 8` 的条数 **≥ 3**
- **非开盘时段** 跑一次,字段大量为 0 的情况下,链路不崩,至少有记录入库(允许 `is_fallback=true`)
- **AI 全部挂掉**(人为制造,比如改 key 为错误值),watchlist + hot 任务仍有记录入库,全部标记 `is_fallback=true`

---

## 5. 兼容性与风险

### 5.1 兼容性
- DB 变更只加一列,带默认值,无需停服迁移
- cron 调度时间不变
- watchlist 流程复用同一个 `Analyze()`,自动享受容错 + fallback,直接修掉"数据不足"问题
- 前端不做功能改动(可选标签延后)

### 5.2 风险
- **新浪接口限流:** 同一 IP 连发三路 list 查询可能触发临流。缓解:三路串行调用(已是),各自 10s 超时,失败跳过
- **放宽 pre-filter 后 ST / 风险股混入:** 通过新加的名称过滤规则(`ST`/`*ST`/`退`)止损
- **fallback 分数虚高:** 规则打分没能识别秒拉诱多之类的陷阱,已在 3.4 节通过 `min(合计, 7)` 硬 cap 止损,配合前端 "兜底" 角标提示用户
- **腾讯详情接口抖动:** 详情拉不到(如字段为空)时 `StockInfo` 字段为 0 的票进 AI,由 prompt 中"数据缺失规则"兜底,不再低分拒答

---

## 6. 不在本次范围

- 不引入新的数据源依赖(akshare、tushare)
- 不改 cron 调度时间或频率
- 不改前端页面布局(仅可选地加一个角标)
- 不做"为什么当前板块是热点"的自动推断(AI 靠股票名 + 常识即可)
- 不在本次做概念/板块榜(免费接口拿不到可靠数据)
