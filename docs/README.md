# MTiano 麻雀 — 日麻计番工具

## 项目大纲

微信小程序 + Go 后端的日本麻将（日麻）工具集，提供番型/点数查表、拍照识别、手动算番功能。

### 技术栈

| 层级 | 技术 |
|------|------|
| 前端 | 微信小程序原生（WXML/WXSS/JS） |
| 后端 | Go + Gin |
| AI 识别 | YOLOv11 目标检测（Python 微服务）+ 千问视觉备选 |
| 部署 | 宝塔面板 + systemd + Nginx 反代 |
| 域名 | https://mahjong.czw-mtiano.cn |

### 目录结构

```
mahjong/
├── miniprogram/                # 微信小程序前端
│   ├── app.json                # 页面与 TabBar 配置
│   ├── app.js                  # 全局配置（serverUrl）
│   ├── data/                   # 静态番型/点数数据
│   │   ├── yaku.js             # 全番型列表
│   │   └── score.js            # 点数表
│   └── pages/
│       ├── index/              # 首页（菜单导航）
│       ├── yaku/               # 番型表查询
│       ├── score/              # 点数表查询
│       ├── camera/             # 拍照识别算番
│       └── manual/             # 手动选牌算番
├── server/                     # Go 后端
│   ├── cmd/server/main.go      # 入口 & 路由注册
│   ├── configs/config.yaml     # 配置文件
│   ├── internal/
│   │   ├── config/             # 配置加载
│   │   ├── handler/            # HTTP 处理器
│   │   │   ├── recognize.go    # POST /api/v1/recognize
│   │   │   └── calculate.go    # POST /api/v1/calculate
│   │   └── service/            # AI 视觉服务
│   │       ├── vision.go       # VisionService 接口
│   │       ├── vision_yolo.go  # YOLO 推理服务调用
│   │       └── vision_qwen.go  # 千问备选实现
│   ├── yolo_service/           # Python YOLO 推理微服务
│   │   ├── app.py              # Flask 服务（端口 5000）
│   │   ├── convert_model.py    # .pt → .onnx 转换脚本
│   │   ├── download_model.py   # 模型下载脚本
│   │   ├── requirements.txt    # Python 依赖
│   │   └── models/             # 模型权重文件
│   │       └── yolo11s_best.onnx
│   └── pkg/mahjong/           # 核心麻将引擎
│       ├── tile.go             # 牌模型 & ParseTiles
│       ├── hand.go             # Hand/Meld 结构
│       ├── parser.go           # 面子分解算法
│       ├── yaku.go             # 番型定义
│       ├── judge.go            # 番型判定逻辑
│       └── calculator.go       # 得分计算
└── docs/                       # 文档
```

### API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/health` | 健康检查 |
| POST | `/api/v1/recognize` | 图片识别算番 |
| POST | `/api/v1/calculate` | 手动选牌算番 |

#### POST /api/v1/calculate 请求体

```json
{
  "tiles": "123m456p789s11z",
  "melds": [
    {"type": "pon", "tiles": "555z"}
  ],
  "dora": "3m",
  "is_tsumo": true,
  "is_parent": true,
  "seat_wind": 0,
  "round_wind": 0
}
```

- `tiles`: 门前手牌（紧凑记法）
- `melds`: 副露数组，type 可选 `chi`/`pon`/`open_kan`/`closed_kan`
- `dora`: 宝牌指示牌
- `seat_wind`: 自风（0=東, 1=南, 2=西, 3=北）
- `round_wind`: 场风
- `is_parent`: 是否庄家
- `is_tsumo`: 是否自摸

---

## 胡牌番数计算流程

### 前提条件

1. 手牌必须能组成合法牌型（4面子+1雀头 或 七对子 或 国士无双）
2. **必须有至少一个役**（宝牌不算役），无役不能和牌

### 计算步骤

```
输入手牌 → 面子分解 → 番型判定 → 累加番数 → 宝牌加番 → 查得分表
```

#### Step 1: 面子分解

将手牌分解为所有可能的组合方式：

- **普通形**: 4面子（顺子或刻子）+ 1雀头
- **七对子**: 7个对子（门前限定）
- **国士无双**: 13种幺九牌各一张 + 任意一张幺九牌重复

有副露时，副露面子已固定，只需分解剩余门前手牌。

#### Step 2: 番型判定

对每种分解方式依次检查所有番型，取番数最高的结果。

##### 役满（13番）

| 番型 | 条件 |
|------|------|
| 国士无双 | 13种幺九各一 + 重复一张（门前限定） |
| 四暗刻 | 4个暗刻（门前限定） |
| 大三元 | 白發中各一刻 |
| 字一色 | 全部字牌 |
| 绿一色 | 仅 2/3/4/6/8s + 發 |
| 清老头 | 仅 1/9 数牌 |
| 四喜和 | 東南西北中三刻一雀头或四刻 |
| 九莲宝灯 | 同一花色 1112345678999+任意（门前限定） |
| 天和 | 庄家配牌即和（门前限定） |
| 地和 | 闲家第一巡自摸和（门前限定） |

##### 3番

| 番型 | 门前 | 副露 |
|------|------|------|
| 混一色 | 3番 | 2番 |
| 纯全带幺九 | 3番 | 2番 |
| 二杯口 | 3番 | — |

##### 2番

| 番型 | 门前 | 副露 |
|------|------|------|
| 七对子 | 2番 | — |
| 对对和 | 2番 | 2番 |
| 三暗刻 | 2番 | 2番 |
| 三色同顺 | 2番 | 1番 |
| 一气通贯 | 2番 | 1番 |
| 混全带幺九 | 2番 | 1番 |
| 三色同刻 | 2番 | 2番 |
| 小三元 | 2番 | 2番 |
| 混老头 | 2番 | 2番 |
| 双立直 | 2番 | — |

##### 1番

| 番型 | 门前 | 副露 |
|------|------|------|
| 立直 | 1番 | — |
| 门前清自摸和 | 1番 | — |
| 断幺九 | 1番 | 1番 |
| 平和 | 1番 | — |
| 一杯口 | 1番 | — |
| 自风牌（東南西北） | 1番 | 1番 |
| 场风牌（東南西北） | 1番 | 1番 |
| 三元牌（白發中） | 1番 | 1番 |
| 岭上开花 | 1番 | 1番 |
| 抢杠 | 1番 | 1番 |
| 海底摸月 | 1番 | 1番 |
| 河底捞鱼 | 1番 | 1番 |

##### 6番

| 番型 | 门前 | 副露 |
|------|------|------|
| 清一色 | 6番 | 5番 |

##### 连风牌

当自风=场风时（如東场庄家），該风牌刻子同时计自风+场风 = 2番。

#### Step 3: 宝牌加番

宝牌指示牌的下一张为实际宝牌，统计手牌（含副露）中命中的宝牌数，每张 +1番。

宝牌循环规则：
- 数牌: 1→2→...→9→1
- 风牌: 東→南→西→北→東
- 三元牌: 白→發→中→白

#### Step 4: 得分计算

```
基本点 = 符数 × 2^(番数+2)
```

当基本点 ≥ 2000 时封顶为满贯。

| 番数 | 等级 | 基本点 |
|------|------|--------|
| 5番（或基本点≥2000） | 满贯 | 2000 |
| 6-7番 | 跳满 | 3000 |
| 8-10番 | 倍满 | 4000 |
| 11-12番 | 三倍满 | 6000 |
| 13番+ | 役满 | 8000 |

##### 得分公式（基于基本点）

| 场景 | 庄家 | 闲家 |
|------|------|------|
| 荣和 | 基本点×6（向上取整百） | 基本点×4（向上取整百） |
| 自摸（每家付） | 闲家各付 基本点×2 | 庄家付 基本点×2，闲家各付 基本点×1 |

##### 得分总额

- 庄家荣和: `基本点 × 6`
- 闲家荣和: `基本点 × 4`
- 庄家自摸: `基本点 × 2 × 3` (三家各付)
- 闲家自摸: `基本点 × 2 + 基本点 × 1 × 2` (庄付2倍 + 闲各付1倍)

> 本项目统一使用30符简化计算。

---

## 构建与运行

```bash
# 后端
cd server
go run cmd/server/main.go
go build ./cmd/server/
go test ./...

systemctl status yolo-mahjong    # 查看状态
systemctl restart yolo-mahjong   # 重启
systemctl stop yolo-mahjong      # 停止
journalctl -u yolo-mahjong -f    # 查看实时日志
# 前端
# 用微信开发者工具打开 miniprogram/ 目录
```

## 牌面记法

| 花色 | 记号 | 示例 |
|------|------|------|
| 万子 | m | `1m`=一万, `9m`=九万 |
| 筒子 | p | `1p`=一筒, `5p`=五筒 |
| 索子 | s | `1s`=一索, `9s`=九索 |
| 字牌 | z | `1z`=東, `2z`=南, `3z`=西, `4z`=北, `5z`=白, `6z`=發, `7z`=中 |

紧凑写法：`123m456p789s11z` = 一二三万 + 四五六筒 + 七八九索 + 東東

---

## YOLO 麻将牌识别模块

### 概述

使用 YOLOv11 目标检测模型识别图片中的麻将牌，部署为独立 Python 微服务，Go 后端通过 HTTP 调用。

### 模型信息

| 项目 | 说明 |
|------|------|
| 来源 | [nikmomo/Mahjong-YOLO](https://github.com/nikmomo/Mahjong-YOLO) |
| 模型 | YOLOv11s (ONNX 格式, ~19MB) |
| 精度 | mAP50 = 0.881 |
| 类别 | 38 类麻将牌 |
| 推理框架 | ONNX Runtime (CPU) |

### 38 类标签

```
1m 2m 3m 4m 5m 6m 7m 8m 9m    (万子)
1p 2p 3p 4p 5p 6p 7p 8p 9p    (筒子)
1s 2s 3s 4s 5s 6s 7s 8s 9s    (索子)
1z 2z 3z 4z 5z 6z 7z           (字牌: 東南西北白發中)
0m 0p 0s                        (赤宝牌: 赤五万/筒/索)
UNKNOWN                          (未知，过滤掉)
```

### 架构

```
小程序 → Go 后端 (POST /api/v1/recognize)
              ↓
       vision_yolo.go (HTTP multipart POST)
              ↓
       Python Flask (POST /predict, 端口 5000)
              ↓
       ONNX Runtime 推理
              ↓
       返回 {"tiles": "123m456p...", "red_dora": 1}
```

### 推理流程

1. **预处理**: 图片缩放到 640×640，保持比例，灰色填充边缘
2. **推理**: ONNX Runtime 加载模型，输入 `[1, 3, 640, 640]`，输出 `[1, 42, 8400]`
3. **后处理**:
   - 转置输出为 `[8400, 42]`（8400个检测框 × (4坐标 + 38类别分数)）
   - 置信度过滤（阈值 0.5）
   - NMS 去重（IoU 阈值 0.5）
   - 按 x 坐标从左到右排序
   - 赤五（`0m/0p/0s`）映射为 `5m/5p/5s` 并标记为赤宝牌
4. **输出**: 拼接为紧凑记法字符串 + 赤宝牌数量

### 赤宝牌处理

- 模型类别 `0m/0p/0s` 代表赤五万/筒/索
- 识别后映射为普通 `5m/5p/5s`（牌面功能相同）
- 同时记录赤宝牌数量 `red_dora`，Go 后端自动计入加番

### 部署步骤

```bash
# 1. 本地转换模型（需要 ultralytics + torch，Mac/PC 上执行）
cd server/yolo_service
python3 -m venv venv && source venv/bin/activate
pip install ultralytics
python download_model.py      # 下载 .pt 权重
python convert_model.py       # 转换为 .onnx

# 2. 上传 .onnx 到服务器
scp models/yolo11s_best.onnx root@服务器:/www/wwwroot/mtiano/mahjong/server/yolo_service/models/

# 3. 服务器安装依赖（Python 3.8+）
python3.10 -m pip install flask onnxruntime pillow numpy -i https://mirrors.aliyun.com/pypi/simple/

# 4. 启动服务
python3.10 app.py
```

### systemd 服务管理

服务文件: `/etc/systemd/system/yolo-mahjong.service`

```ini
[Unit]
Description=Mahjong YOLO Detection Service
After=network.target

[Service]
Type=simple
WorkingDirectory=/www/wwwroot/mtiano/mahjong/server/yolo_service
ExecStart=/usr/local/bin/python3.10 app.py
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

管理命令:

```bash
systemctl start yolo-mahjong     # 启动
systemctl stop yolo-mahjong      # 停止
systemctl restart yolo-mahjong   # 重启
systemctl status yolo-mahjong    # 状态
journalctl -u yolo-mahjong -f   # 查看日志
```

### Go 后端配置

`server/configs/config.yaml`:

```yaml
vision:
  provider: "yolo"
  endpoint: "http://localhost:5000"
```

切换回千问（备选）:

```yaml
vision:
  provider: "qwen"
  api_key: "your-api-key"
  endpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1"
  model: "qwen3.6-plus-2026-04-02"
```

### API 测试

```bash
# 直接测试 Python 推理服务
curl -X POST -F "image=@test.jpg" http://localhost:5000/predict

# 测试 Go 后端完整流程
curl -X POST -F "image=@test.jpg" https://mahjong.czw-mtiano.cn/api/v1/recognize
```

### 已知限制

- 赤宝牌（赤五）识别率较低，与普通五外观差异小
- 牌面遮挡或倾斜角度大时精度下降
- 建议配合手动算番功能修正识别结果
