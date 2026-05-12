package cron

import (
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
