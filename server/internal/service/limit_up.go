package service

import "strings"

// LimitUpWarning 返回涨停股的流动性风险警告文字;非涨停股返回 ""。
//
// 涨停阈值按板块区分(留 0.05% 容差覆盖"涨停价四舍五入到分"的浮点误差):
//   - 创业板(300 开头)/ 科创板(688 开头):20% 涨停 → 阈值 19.95%
//   - 主板 / 中小板 / 其它:10% 涨停 → 阈值 9.95%
//
// 优先级:开盘一字涨停 > 盘中已涨停。
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

// annotateLimitUp 把涨停警告 prefix 到 result.TrapWarning;
// 无警告时原样返回 result。涨停警告优先级高于 AI 推断的陷阱,
// 用 "; " 分隔保留原 trap_warning(若有)。
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
