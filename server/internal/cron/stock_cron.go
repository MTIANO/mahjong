package cron

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/mtiano/server/internal/model"
	"github.com/mtiano/server/internal/service"
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
	sc.scheduler.AddFunc("@every 1h", sc.analyzeWatchlistStocks)
	sc.scheduler.AddFunc("@every 1h", sc.analyzeHotStocks)
	sc.scheduler.Start()
	log.Println("[Cron] stock analysis cron started")

	go sc.analyzeWatchlistStocks()
	go sc.analyzeHotStocks()
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
	log.Println("[Cron] starting hot stock analysis")
	stocks, err := sc.stockData.GetHotStocks(10)
	if err != nil {
		log.Printf("[Cron] get hot stocks: %v", err)
		return
	}

	sc.analyzeAndStore(stocks, "hot")
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
			AnalysisDate: today,
		}

		if err := sc.store.UpsertRecommendation(rec); err != nil {
			log.Printf("[Cron] upsert recommendation %s: %v", stock.Code, err)
			continue
		}
		log.Printf("[Cron] analyzed %s(%s): buy=%d tail=%d", stock.Name, stock.Code, result.BuyScore, result.TailScore)
	}
}
