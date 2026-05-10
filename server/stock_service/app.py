import akshare as ak
from flask import Flask, request, jsonify

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
