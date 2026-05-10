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


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "ok"})


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5001)
