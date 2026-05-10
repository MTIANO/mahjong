# 股票 AI 分析与推荐系统设计文档

## 概述

在 MTiano 小程序中新增「股票推荐」模块，提供基于 AI 的股票购买推荐。系统定时拉取用户自选股和热门股票数据，通过通义千问 AI 分析生成「今日购买」和「尾盘购买」的推荐度及理由，在小程序端展示。

## 架构选型

**Go 单体扩展 + AKShare Python 微服务**

- Go 服务：扩展现有 Gin 服务，新增定时任务（robfig/cron）、HTTP 接口、MySQL 存储
- AKShare 微服务：新建 Python Flask 服务，提供股票数据获取接口（类似现有 yolo_service 模式）
- AI 分析：复用通义千问（阿里云 DashScope），文本对话接口
- 前端：小程序新增 stock tab 页面

## 数据模型

### MySQL 表结构

#### users 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT AUTO_INCREMENT | 主键 |
| openid | VARCHAR(64) UNIQUE | 微信 openid |
| nickname | VARCHAR(50) | 昵称（可选） |
| created_at | DATETIME | 注册时间 |
| updated_at | DATETIME | 更新时间 |

#### watchlist 表（用户自选股）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT AUTO_INCREMENT | 主键 |
| user_id | BIGINT | 关联 users.id |
| stock_code | VARCHAR(10) | 股票代码，如 "600519" |
| stock_name | VARCHAR(50) | 股票名称，如 "贵州茅台" |
| created_at | DATETIME | 添加时间 |

唯一索引：`(user_id, stock_code)`

#### stock_recommendations 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT AUTO_INCREMENT | 主键 |
| stock_code | VARCHAR(10) | 股票代码 |
| stock_name | VARCHAR(50) | 股票名称 |
| source | VARCHAR(10) | 来源：watchlist / hot |
| buy_score | TINYINT | 今日购买推荐度 1-10 |
| buy_reason | TEXT | 今日购买推荐理由 |
| tail_score | TINYINT | 尾盘购买推荐度 1-10 |
| tail_reason | TEXT | 尾盘购买推荐理由 |
| analysis_date | DATE | 分析日期 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

联合唯一索引：`(stock_code, source, analysis_date)` — 同一天同来源的同一只股票 upsert 最新结果。

## 微信小程序授权登录

### 流程

1. 小程序启动 / 进入股票 tab 时检查本地是否有有效 token
2. 无 token 或已过期 → 调用 `wx.login()` 获取 code
3. 请求 `POST /api/v1/auth/login { code }` → Go 服务用 code 向微信服务器换 openid
4. Go 服务根据 openid 查找或创建用户，生成 JWT token 返回
5. 小程序存储 token（`wx.setStorageSync`），后续请求带 `Authorization: Bearer <token>`

### 访问控制

- 股票模块所有接口需要登录（JWT 中间件）
- 进入股票 tab 时若未登录，自动触发登录流程
- 现有的麻将功能不受影响，无需登录

## AKShare 微服务

路径：`server/stock_service/`（与 yolo_service 同级）

### 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/stock/hot?count=10 | 获取今日热门前 N 只股票 |
| GET | /api/stock/detail?codes=600519,000001 | 获取指定股票的实时数据 |

### 返回数据

```json
{
  "stocks": [
    {
      "code": "600519",
      "name": "贵州茅台",
      "price": 1680.50,
      "change_pct": 2.35,
      "volume": 12345678,
      "turnover_rate": 0.85,
      "pe_ratio": 28.5,
      "market_cap": 21100.0
    }
  ]
}
```

### AKShare 函数

- 热门股票：`ak.stock_hot_rank_em()` 获取东方财富热门排行
- 股票详情：`ak.stock_zh_a_spot_em()` 获取 A 股实时行情

## Go 服务端

### 新增文件结构

```
server/
├── internal/
│   ├── config/config.go           # 扩展配置结构
│   ├── model/
│   │   ├── user.go                # User 模型
│   │   ├── watchlist.go           # Watchlist 模型
│   │   └── recommendation.go     # StockRecommendation 模型
│   ├── handler/
│   │   ├── auth.go                # 登录接口
│   │   └── stock.go               # 股票相关接口
│   ├── middleware/
│   │   └── auth.go                # JWT 认证中间件
│   ├── service/
│   │   ├── auth.go                # 微信登录 + JWT 生成
│   │   ├── stock_data.go          # 调用 AKShare 微服务
│   │   ├── stock_analyzer.go      # 调用通义千问分析股票
│   │   └── stock_store.go         # MySQL 存取
│   └── cron/
│       └── stock_cron.go          # 定时任务编排
├── stock_service/
│   ├── app.py                     # AKShare Flask 服务
│   └── requirements.txt
```

### 定时任务

使用 `robfig/cron` 库，在 `main.go` 启动时注册：

1. **自选股分析**（每小时）：查询所有用户自选股（去重）→ 调 AKShare 获取实时数据 → 调 AI 分析 → upsert 到 stock_recommendations
2. **热门股分析**（每小时）：调 AKShare 获取热门前 10 → 调 AI 分析 → upsert 到 stock_recommendations

### HTTP 接口

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| POST | /api/v1/auth/login | 否 | 微信授权登录 |
| GET | /api/v1/stock/recommendations | 是 | 获取今日推荐列表（自选+热门，按 buy_score 降序） |
| GET | /api/v1/stock/watchlist | 是 | 获取当前用户自选股列表 |
| POST | /api/v1/stock/watchlist | 是 | 添加自选股（传股票代码） |
| DELETE | /api/v1/stock/watchlist/:code | 是 | 删除自选股 |

### AI 分析

调用通义千问文本对话接口（OpenAI 兼容格式），prompt 包含股票实时数据，要求返回结构化 JSON：

```json
{
  "buy_score": 7,
  "buy_reason": "该股近期放量突破年线，MACD金叉形成...",
  "tail_score": 8,
  "tail_reason": "午后资金净流入明显，尾盘有冲高可能..."
}
```

### 配置扩展

```yaml
server:
  port: ":9091"

vision:
  provider: "yolo"
  endpoint: "http://localhost:5000"

wechat:
  appid: "wx_xxx"
  secret: "xxx"

jwt:
  secret: "your-jwt-secret"
  expire_hours: 168

mysql:
  dsn: "user:password@tcp(localhost:3306)/mtiano?charset=utf8mb4&parseTime=True"

stock:
  akshare_endpoint: "http://localhost:5001"
  ai_model: "qwen-plus"
  ai_api_key: "sk-xxx"
  ai_endpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1"
```

## 小程序前端

### 页面

新增 `pages/stock/stock` 作为第 6 个 tab 页面。

### 访问控制

进入 stock 页面时：
1. 检查本地 token 是否存在且有效
2. 无效 → 调用 `wx.login()` + 后端登录接口 → 获取 token
3. 登录成功后加载数据

### 页面布局

```
┌──────────────────────────────┐
│  📈 股票推荐    2026-05-10    │  ← 标题 + 日期
├──────────────────────────────┤
│  [全部] [自选] [热门]         │  ← 筛选 tab
├──────────────────────────────┤
│  ┌─ 600519 贵州茅台 ────────┐ │
│  │ 今日购买: ████████░░ 8分  │ │
│  │ MACD金叉，放量突破年线    │ │
│  │ 尾盘购买: ██████░░░░ 6分  │ │
│  │ 尾盘资金面不确定          │ │
│  │                  [自选]   │ │  ← 来源标签
│  └──────────────────────────┘ │
│  ┌─ 000001 平安银行 ────────┐ │
│  │ ...                       │ │
│  └──────────────────────────┘ │
├──────────────────────────────┤
│  + 添加自选股 [输入股票代码]  │  ← 底部添加入口
└──────────────────────────────┘
```

### 功能要点

- 筛选 tab：全部 / 自选 / 热门
- 列表按 buy_score 降序排列
- 卡片展示：股票代码+名称、今日/尾盘推荐度（进度条+分数）、推荐理由、来源标签
- 底部输入框：输入股票代码添加自选
- 自选股支持左滑删除
- 下拉刷新获取最新数据

## 技术依赖

### Go 新增依赖

- `github.com/robfig/cron/v3` — 定时任务
- `github.com/go-sql-driver/mysql` — MySQL 驱动
- `github.com/golang-jwt/jwt/v5` — JWT 生成与验证

### Python (stock_service)

- `flask` — HTTP 服务
- `akshare` — 股票数据获取

## 测试策略

- Go 单元测试：stock_analyzer（mock AI 响应）、stock_store（测试 DB 操作）、auth（JWT 生成验证）
- AKShare 微服务：接口测试
- 小程序端：在微信开发者工具中手动测试登录流程和数据展示
