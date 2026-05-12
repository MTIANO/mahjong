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
	scheduler *cron.Cron
	store     *service.StockStore
	stockData *service.StockDataService
	analyzer  *service.StockAnalyzer
}

func NewStockCron(store *service.StockStore, stockData *service.StockDataService, analyzer *service.StockAnalyzer) *StockCron {
	return &StockCron{
		scheduler: cron.New(),
		store:     store,
		stockData: stockData,
		analyzer:  analyzer,
	}
}

func (sc *StockCron) Start() {
	// 上午盘中分析 9:30, 10:30, 11:30
	sc.scheduler.AddFunc("30 9-11 * * 1-5", sc.analyzeWatchlistStocks)
	sc.scheduler.AddFunc("30 9-11 * * 1-5", sc.analyzeHotStocks)
	// 下午盘中分析 13:00, 14:00
	sc.scheduler.AddFunc("0 13-14 * * 1-5", sc.analyzeWatchlistStocks)
	sc.scheduler.AddFunc("0 13-14 * * 1-5", sc.analyzeHotStocks)
	// 尾盘黄金窗口 14:30（主力最终表态期，最关键的一次分析）
	sc.scheduler.AddFunc("30 14 * * 1-5", sc.analyzeWatchlistStocks)
	sc.scheduler.AddFunc("30 14 * * 1-5", sc.analyzeHotStocks)
	sc.scheduler.Start()
	log.Println("[Cron] stock analysis cron started (9:30-11:30, 13:00-14:00, 14:30 weekdays)")
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

	sc.analyzeAndStore(stocks, "watchlist")
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

	filtered := sc.preFilterStocks(merged)
	log.Printf("[Cron] candidate pool: hot=%d gainers=%d active=%d merged=%d filtered=%d",
		len(hot), len(gainers), len(active), len(merged), len(filtered))

	if len(filtered) == 0 {
		log.Println("[Cron] no candidates passed pre-filter, analyzing top 5 of merged pool")
		if len(merged) > 5 {
			merged = merged[:5]
		}
		sc.analyzeAndStore(merged, "hot")
		return
	}

	if len(filtered) > 20 {
		filtered = filtered[:20]
	}
	sc.analyzeAndStore(filtered, "hot")
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

func (sc *StockCron) analyzeAndStore(stocks []service.StockInfo, source string) {
	today := time.Now().Format("2006-01-02")
	ctx := context.Background()

	for _, stock := range stocks {
		result, err := sc.analyzer.Analyze(ctx, stock)
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
