import json
import re
import requests
from flask import Flask, request, jsonify

app = Flask(__name__)

HEADERS = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
    "Referer": "https://finance.sina.com.cn",
}


def stock_symbol_qq(code):
    code = code.strip().zfill(6)
    if code.startswith(("6", "9")):
        return "sh" + code
    return "sz" + code


def fetch_stocks_qq(codes):
    """腾讯财经实时行情 API，云服务器可用"""
    symbols = ",".join(stock_symbol_qq(c) for c in codes)
    url = f"http://qt.gtimg.cn/q={symbols}"
    resp = requests.get(url, headers=HEADERS, timeout=10)
    resp.encoding = "gbk"

    stocks = []
    for line in resp.text.strip().split("\n"):
        line = line.strip()
        if not line or '="' not in line:
            continue
        match = re.search(r'"(.+)"', line)
        if not match:
            continue
        fields = match.group(1).split("~")
        if len(fields) < 46 or not fields[3]:
            continue
        price = float(fields[3])
        if price == 0:
            continue
        stocks.append({
            "code": fields[2],
            "name": fields[1],
            "price": price,
            "change_pct": float(fields[32]) if fields[32] else 0,
            "volume": int(float(fields[6]) * 100) if fields[6] else 0,
            "turnover_rate": float(fields[38]) if fields[38] else 0,
            "pe_ratio": float(fields[39]) if fields[39] else 0,
            "market_cap": float(fields[45]) if fields[45] else 0,
        })
    return stocks


def fetch_hot_sina(count=10):
    """新浪财经成交额排行，云服务器可用"""
    url = (
        "https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/"
        "Market_Center.getHQNodeData"
    )
    params = {
        "page": 1,
        "num": count,
        "sort": "amount",
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
        stocks.append({
            "code": item["code"],
            "name": item["name"],
            "price": price,
            "change_pct": float(item.get("changepercent", 0) or 0),
            "volume": int(float(item.get("volume", 0) or 0)),
            "turnover_rate": float(item.get("turnoverratio", 0) or 0),
            "pe_ratio": float(item.get("per", 0) or 0),
            "market_cap": float(item.get("mktcap", 0) or 0) / 10000,
        })
    return stocks


@app.route("/api/stock/hot", methods=["GET"])
def get_hot_stocks():
    count = request.args.get("count", 10, type=int)
    try:
        stocks = fetch_hot_sina(count)
        return jsonify({"stocks": stocks})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/api/stock/detail", methods=["GET"])
def get_stock_detail():
    codes_param = request.args.get("codes", "")
    if not codes_param:
        return jsonify({"error": "codes parameter required"}), 400

    codes = [c.strip() for c in codes_param.split(",") if c.strip()]
    try:
        stocks = fetch_stocks_qq(codes)
        return jsonify({"stocks": stocks})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


def fetch_quote_qq(code):
    """腾讯财经详细行情，返回更多字段"""
    symbol = stock_symbol_qq(code)
    url = f"http://qt.gtimg.cn/q={symbol}"
    resp = requests.get(url, headers=HEADERS, timeout=10)
    resp.encoding = "gbk"

    line = resp.text.strip()
    match = re.search(r'"(.+)"', line)
    if not match:
        return None
    fields = match.group(1).split("~")
    if len(fields) < 50 or not fields[3]:
        return None
    price = float(fields[3])
    if price == 0:
        return None
    prev_close = float(fields[4]) if fields[4] else 0
    change = round(price - prev_close, 3) if prev_close else 0
    return {
        "code": fields[2],
        "name": fields[1],
        "price": price,
        "change": change,
        "change_pct": float(fields[32]) if fields[32] else 0,
        "open": float(fields[5]) if fields[5] else 0,
        "prev_close": prev_close,
        "high": float(fields[33]) if fields[33] else 0,
        "low": float(fields[34]) if fields[34] else 0,
        "volume": int(float(fields[6]) * 100) if fields[6] else 0,
        "amount": float(fields[37]) if fields[37] else 0,
        "turnover_rate": float(fields[38]) if fields[38] else 0,
        "pe_ratio": float(fields[39]) if fields[39] else 0,
        "pb_ratio": float(fields[46]) if len(fields) > 46 and fields[46] else 0,
        "market_cap": float(fields[45]) if fields[45] else 0,
        "float_market_cap": float(fields[44]) if fields[44] else 0,
        "amplitude": float(fields[43]) if fields[43] else 0,
        "volume_ratio": float(fields[49]) if len(fields) > 49 and fields[49] else 0,
    }


def fetch_daily_kline(code, count=60):
    """腾讯财经前复权日K线"""
    symbol = stock_symbol_qq(code)
    url = f"http://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param={symbol},day,,,{count},qfq"
    resp = requests.get(url, headers=HEADERS, timeout=10)
    data = resp.json()

    qfq_key = "qfqday"
    day_key = "day"
    stock_data = data.get("data", {}).get(symbol, {})
    klines = stock_data.get(qfq_key) or stock_data.get(day_key, [])

    result = []
    for item in klines:
        if len(item) < 6:
            continue
        result.append({
            "date": item[0],
            "open": float(item[1]),
            "close": float(item[2]),
            "high": float(item[3]),
            "low": float(item[4]),
            "volume": float(item[5]),
        })
    return result


def fetch_minute_kline(code):
    """腾讯财经分时数据"""
    symbol = stock_symbol_qq(code)
    url = f"http://web.ifzq.gtimg.cn/appstock/app/minute/query?_var=min_data&code={symbol}"
    resp = requests.get(url, headers=HEADERS, timeout=10)
    text = resp.text.strip()

    json_match = re.search(r"=\s*({.*})\s*;?\s*$", text, re.DOTALL)
    if not json_match:
        return {"prev_close": 0, "data": []}

    data = json.loads(json_match.group(1))
    stock_data = data.get("data", {}).get(symbol, {})
    qt_data = stock_data.get("qt", {}).get(symbol, [])
    prev_close = float(qt_data[4]) if len(qt_data) > 4 and qt_data[4] else 0

    minutes = stock_data.get("data", {}).get("data", [])
    result = []
    total_volume = 0
    total_amount = 0
    for m in minutes:
        parts = m.split(" ")
        if len(parts) < 3:
            continue
        p = float(parts[1])
        v = int(parts[2])
        total_volume += v
        total_amount += p * v
        avg = round(total_amount / total_volume, 3) if total_volume > 0 else p
        result.append({
            "time": parts[0][:2] + ":" + parts[0][2:],
            "price": p,
            "volume": v,
            "avg_price": avg,
        })
    return {"prev_close": prev_close, "data": result}


@app.route("/api/stock/quote", methods=["GET"])
def get_stock_quote():
    code = request.args.get("code", "")
    if not code:
        return jsonify({"error": "code parameter required"}), 400
    try:
        quote = fetch_quote_qq(code)
        if not quote:
            return jsonify({"error": "stock not found"}), 404
        return jsonify({"quote": quote})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/api/stock/kline/daily", methods=["GET"])
def get_daily_kline():
    code = request.args.get("code", "")
    count = request.args.get("count", 60, type=int)
    if not code:
        return jsonify({"error": "code parameter required"}), 400
    try:
        klines = fetch_daily_kline(code, count)
        return jsonify({"klines": klines})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/api/stock/kline/minute", methods=["GET"])
def get_minute_kline():
    code = request.args.get("code", "")
    if not code:
        return jsonify({"error": "code parameter required"}), 400
    try:
        data = fetch_minute_kline(code)
        return jsonify(data)
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "ok"})


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5001)
