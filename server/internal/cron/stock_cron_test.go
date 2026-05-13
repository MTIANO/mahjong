package cron

import (
	"fmt"
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

func Test_preFilterStocks_relaxed(t *testing.T) {
	sc := &StockCron{}

	cases := []struct {
		name   string
		stock  service.StockInfo
		passed bool
	}{
		{"涨幅边界低(0.5%)通过", service.StockInfo{Code: "1", Name: "A", ChangePct: 0.5, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: 20}, true},
		{"涨停 10% 通过(允许涨停接力)", service.StockInfo{Code: "2", Name: "B", ChangePct: 10.0, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: 20}, true},
		{"涨幅边界高(10.5%)通过", service.StockInfo{Code: "2b", Name: "B2", ChangePct: 10.5, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: 20}, true},
		{"涨幅超界(11%)刷掉", service.StockInfo{Code: "3", Name: "C", ChangePct: 11, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: 20}, false},
		{"PE 负值通过(放弃硬刷)", service.StockInfo{Code: "4", Name: "D", ChangePct: 3, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: -50}, true},
		{"PE 1000 通过(放开短线估值上限)", service.StockInfo{Code: "4b", Name: "D2", ChangePct: 3, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: 1000}, true},
		{"PE 1100 刷掉", service.StockInfo{Code: "4c", Name: "D3", ChangePct: 3, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 50, PERatio: 1100}, false},
		{"流通 20 亿通过", service.StockInfo{Code: "5", Name: "E", ChangePct: 3, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 20, PERatio: 20}, true},
		{"流通 14000 亿(大盘 AI 龙头)通过", service.StockInfo{Code: "6", Name: "F", ChangePct: 3, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 14000, PERatio: 20}, true},
		{"流通 20001 亿刷掉(理论上限)", service.StockInfo{Code: "6b", Name: "F2", ChangePct: 3, TurnoverRate: 3, VolumeRatio: 1.0, FloatMarketCap: 20001, PERatio: 20}, false},
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
