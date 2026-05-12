package cron

import (
	"context"
	"log"
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
	log.Println("[Cron] starting hot stock analysis (with pre-filter)")
	stocks, err := sc.stockData.GetHotStocks(30)
	if err != nil {
		log.Printf("[Cron] get hot stocks: %v", err)
		return
	}

	filtered := sc.preFilterStocks(stocks)
	if len(filtered) == 0 {
		log.Println("[Cron] no hot stocks passed pre-filter, analyzing top 5 by volume")
		if len(stocks) > 5 {
			stocks = stocks[:5]
		}
		sc.analyzeAndStore(stocks, "hot")
		return
	}

	if len(filtered) > 10 {
		filtered = filtered[:10]
	}
	log.Printf("[Cron] %d/%d hot stocks passed pre-filter", len(filtered), len(stocks))
	sc.analyzeAndStore(filtered, "hot")
}

func (sc *StockCron) preFilterStocks(stocks []service.StockInfo) []service.StockInfo {
	var passed []service.StockInfo
	for _, s := range stocks {
		if s.ChangePct < 1 || s.ChangePct > 7 {
			continue
		}
		if s.TurnoverRate > 0 && (s.TurnoverRate < 3 || s.TurnoverRate > 15) {
			continue
		}
		if s.VolumeRatio > 0 && s.VolumeRatio < 0.8 {
			continue
		}
		if s.FloatMarketCap > 0 && (s.FloatMarketCap < 30 || s.FloatMarketCap > 300) {
			continue
		}
		if s.PERatio < 0 || s.PERatio > 200 {
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
