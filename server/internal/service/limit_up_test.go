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
		{
			"创业板 301 开头涨停 20%",
			StockInfo{Code: "301001", ChangePct: 20.0, PrevClose: 10.0, Open: 10.0},
			"当前已涨停,追高风险",
		},
		{
			"北交所 8 开头涨停 30%",
			StockInfo{Code: "831001", ChangePct: 30.0, PrevClose: 10.0, Open: 10.0},
			"当前已涨停,追高风险",
		},
		{
			"北交所 12% 不警告(30% 才涨停)",
			StockInfo{Code: "831001", ChangePct: 12.0, PrevClose: 10.0, Open: 10.0},
			"",
		},
	}

	for _, c := range cases {
		got := limitUpWarning(c.stock)
		if got != c.want {
			t.Errorf("%s: want %q, got %q", c.name, c.want, got)
		}
	}
}
