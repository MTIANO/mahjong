package cron

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/mtiano/server/internal/model"
	"github.com/mtiano/server/internal/service"
	"github.com/robfig/cron/v3"
)

type StockCron struct {
	scheduler  *cron.Cron
	store      *service.StockStore
	stockData  *service.StockDataService
	analyzer   *service.StockAnalyzer
	themeScout *service.ThemeScout
}

func NewStockCron(store *service.StockStore, stockData *service.StockDataService, analyzer *service.StockAnalyzer, themeScout *service.ThemeScout) *StockCron {
	return &StockCron{
		scheduler:  cron.New(),
		store:      store,
		stockData:  stockData,
		analyzer:   analyzer,
		themeScout: themeScout,
	}
}

func (sc *StockCron) Start() {
	// 工作日交易时段:10:00 / 10:30 / 11:30 / 13:00 / 14:00 / 14:30
	// 9:30 开盘瞬间盘口数据不稳定(涨幅/换手未充分形成),改为 10:00 起跑
	schedules := []string{
		"0 10 * * 1-5",  // 10:00
		"30 10 * * 1-5", // 10:30
		"30 11 * * 1-5", // 11:30
		"0 13 * * 1-5",  // 13:00
		"0 14 * * 1-5",  // 14:00
		"30 14 * * 1-5", // 14:30 尾盘黄金窗口
	}
	for _, s := range schedules {
		sc.scheduler.AddFunc(s, sc.analyzeWatchlistStocks)
		sc.scheduler.AddFunc(s, sc.analyzeHotStocks)
	}
	sc.scheduler.Start()
	log.Println("[Cron] stock analysis cron started (10:00 / 10:30 / 11:30 / 13:00 / 14:00 / 14:30 weekdays)")
}

func (sc *StockCron) Stop() {
	sc.scheduler.Stop()
}

func (sc *StockCron) analyzeWatchlistStocks() {
	log.Println("[Cron] starting watchlist stock analysis")
	codes, err := sc.store.GetAllWatchlistCodes()
	if err != nil {
		log.Printf("[Cron] get watchlist codes: %v", err)
		return
	}
	if len(codes) == 0 {
		log.Println("[Cron] no watchlist stocks to analyze")
		return
	}

	if len(codes) > 10 {
		codes = codes[:10]
	}

	stocks, err := sc.stockData.GetStockDetails(codes)
	if err != nil {
		log.Printf("[Cron] get stock details: %v", err)
		return
	}

	themes := sc.themeScout.FetchThemes(context.Background())
	sc.analyzeAndStore(stocks, "watchlist", themes)
}

func (sc *StockCron) analyzeHotStocks() {
	log.Println("[Cron] starting hot stock analysis (3-source pool)")

	hot, hotErr := sc.stockData.GetHotStocks(20)
	if hotErr != nil {
		log.Printf("[Cron] fetch hot failed: %v", hotErr)
	}
	gainers, gainersErr := sc.stockData.GetGainers(20)
	if gainersErr != nil {
		log.Printf("[Cron] fetch gainers failed: %v", gainersErr)
	}
	active, activeErr := sc.stockData.GetActive(20)
	if activeErr != nil {
		log.Printf("[Cron] fetch active failed: %v", activeErr)
	}

	merged := mergeCandidates(hot, hotErr, gainers, gainersErr, active, activeErr)
	if len(merged) == 0 {
		log.Println("[Cron] all three sources failed, aborting hot analysis")
		return
	}

	// Sina list 字段稀薄(无 volume_ratio / amplitude / float_market_cap / high/low/open/prev_close),
	// 走腾讯 detail 接口 hydrate,让 pre-filter 和 AI 都看到完整盘口。
	hydrated := sc.hydrateStockDetails(merged)

	filtered := sc.preFilterStocks(hydrated)
	log.Printf("[Cron] candidate pool: hot=%d gainers=%d active=%d merged=%d hydrated=%d filtered=%d",
		len(hot), len(gainers), len(active), len(merged), len(hydrated), len(filtered))

	// 每次 cron 触发都调一次 Qwen 拿今日热点,作为 DeepSeek 打分的参考信号
	themes := sc.themeScout.FetchThemes(context.Background())

	if len(filtered) == 0 {
		log.Println("[Cron] no candidates passed pre-filter, analyzing top 5 of merged pool")
		if len(merged) > 5 {
			merged = merged[:5]
		}
		sc.analyzeAndStore(merged, "hot", themes)
		return
	}

	if len(filtered) > 20 {
		filtered = filtered[:20]
	}
	sc.analyzeAndStore(filtered, "hot", themes)
}

func (sc *StockCron) preFilterStocks(stocks []service.StockInfo) []service.StockInfo {
	var passed []service.StockInfo
	for _, s := range stocks {
		// ST / 退市 股直接刷掉
		if strings.Contains(s.Name, "ST") || strings.Contains(s.Name, "退") {
			continue
		}
		if s.ChangePct < 0.5 || s.ChangePct > 9.5 {
			continue
		}
		if s.TurnoverRate > 0 && (s.TurnoverRate < 2 || s.TurnoverRate > 20) {
			continue
		}
		if s.VolumeRatio > 0 && s.VolumeRatio < 0.6 {
			continue
		}
		if s.FloatMarketCap > 0 && (s.FloatMarketCap < 20 || s.FloatMarketCap > 500) {
			continue
		}
		if s.PERatio < -1000 || s.PERatio > 300 {
			continue
		}
		passed = append(passed, s)
	}
	return passed
}

func (sc *StockCron) analyzeAndStore(stocks []service.StockInfo, source string, themes []string) {
	today := time.Now().Format("2006-01-02")
	ctx := context.Background()

	for _, stock := range stocks {
		result, err := sc.analyzer.Analyze(ctx, stock, themes)
		if err != nil {
			log.Printf("[Cron] analyze %s(%s): %v", stock.Name, stock.Code, err)
			continue
		}

		rec := &model.StockRecommendation{
			StockCode:    stock.Code,
			StockName:    stock.Name,
			Source:       source,
			BuyScore:     result.BuyScore,
			BuyReason:    result.BuyReason,
			TailScore:    result.TailScore,
			TailReason:   result.TailReason,
			KeySignals:   result.KeySignals,
			RiskLevel:    result.RiskLevel,
			TrapWarning:  result.TrapWarning,
			IsFallback:   result.IsFallback,
			AnalysisDate: today,
		}

		if err := sc.store.UpsertRecommendation(rec); err != nil {
			log.Printf("[Cron] upsert recommendation %s: %v", stock.Code, err)
			continue
		}
		log.Printf("[Cron] analyzed %s(%s): buy=%d tail=%d risk=%d", stock.Name, stock.Code, result.BuyScore, result.TailScore, result.RiskLevel)
	}
}

// dedupByCode 按参数顺序合并多个榜单,按 code 去重,保留首次出现项(靠前榜单优先)。
func dedupByCode(lists ...[]service.StockInfo) []service.StockInfo {
	seen := map[string]struct{}{}
	var merged []service.StockInfo
	for _, list := range lists {
		for _, s := range list {
			if _, ok := seen[s.Code]; ok {
				continue
			}
			seen[s.Code] = struct{}{}
			merged = append(merged, s)
		}
	}
	return merged
}

// hydrateStockDetails 用腾讯 detail 接口补全字段。
// Sina list 只返回 8 个字段(无 volume_ratio / amplitude / float_market_cap / high/low/open/prev_close);
// 腾讯 detail 返回 14 字段,让 pre-filter 和 AI prompt 看到完整盘口。
// 分批 25 只一次,避免 URL 过长触发腾讯接口限制。
// 任一批失败 log + 降级使用原数据;codes 顺序保留。
func (sc *StockCron) hydrateStockDetails(stocks []service.StockInfo) []service.StockInfo {
	if len(stocks) == 0 {
		return stocks
	}
	const batchSize = 25
	hydratedByCode := make(map[string]service.StockInfo, len(stocks))
	for i := 0; i < len(stocks); i += batchSize {
		end := i + batchSize
		if end > len(stocks) {
			end = len(stocks)
		}
		codes := make([]string, 0, end-i)
		for _, s := range stocks[i:end] {
			codes = append(codes, s.Code)
		}
		detail, err := sc.stockData.GetStockDetails(codes)
		if err != nil {
			log.Printf("[Cron] hydrate batch [%d,%d) failed: %v", i, end, err)
			continue
		}
		for _, d := range detail {
			hydratedByCode[d.Code] = d
		}
	}

	out := make([]service.StockInfo, 0, len(stocks))
	for _, s := range stocks {
		if d, ok := hydratedByCode[s.Code]; ok {
			out = append(out, d)
		} else {
			// 腾讯没拿到这只,保留 Sina 原数据(字段稀薄但至少有 code/name/price/change_pct)
			out = append(out, s)
		}
	}
	return out
}

// mergeCandidates 合并三榜,任一路的 err 非 nil 时跳过该路,返回去重后的候选池。
func mergeCandidates(hot []service.StockInfo, hotErr error, gainers []service.StockInfo, gainersErr error, active []service.StockInfo, activeErr error) []service.StockInfo {
	var lists [][]service.StockInfo
	if hotErr == nil && len(hot) > 0 {
		lists = append(lists, hot)
	}
	if gainersErr == nil && len(gainers) > 0 {
		lists = append(lists, gainers)
	}
	if activeErr == nil && len(active) > 0 {
		lists = append(lists, active)
	}
	return dedupByCode(lists...)
}
