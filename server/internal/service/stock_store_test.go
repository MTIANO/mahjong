package service

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mtiano/server/internal/model"
)

func getTestDB(t *testing.T) *sql.DB {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN not set, skipping DB test")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func TestStockStore_FindOrCreateUser(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	store := NewStockStore(db)

	user, err := store.FindOrCreateUser("test_openid_123")
	if err != nil {
		t.Fatalf("FindOrCreateUser: %v", err)
	}
	if user.OpenID != "test_openid_123" {
		t.Errorf("expected openid test_openid_123, got %s", user.OpenID)
	}
	if user.ID == 0 {
		t.Error("expected non-zero user ID")
	}

	user2, err := store.FindOrCreateUser("test_openid_123")
	if err != nil {
		t.Fatalf("FindOrCreateUser again: %v", err)
	}
	if user2.ID != user.ID {
		t.Errorf("expected same user ID %d, got %d", user.ID, user2.ID)
	}

	db.Exec("DELETE FROM users WHERE openid = 'test_openid_123'")
}

func TestStockStore_Watchlist(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	store := NewStockStore(db)

	user, _ := store.FindOrCreateUser("test_watchlist_user")
	defer db.Exec("DELETE FROM users WHERE openid = 'test_watchlist_user'")
	defer db.Exec("DELETE FROM watchlist WHERE user_id = ?", user.ID)

	err := store.AddWatchlistItem(user.ID, "600519", "贵州茅台")
	if err != nil {
		t.Fatalf("AddWatchlistItem: %v", err)
	}

	items, err := store.GetWatchlist(user.ID)
	if err != nil {
		t.Fatalf("GetWatchlist: %v", err)
	}
	if len(items) != 1 || items[0].StockCode != "600519" {
		t.Errorf("unexpected watchlist: %+v", items)
	}

	err = store.RemoveWatchlistItem(user.ID, "600519")
	if err != nil {
		t.Fatalf("RemoveWatchlistItem: %v", err)
	}

	items, _ = store.GetWatchlist(user.ID)
	if len(items) != 0 {
		t.Errorf("expected empty watchlist after delete, got %d", len(items))
	}
}

func TestStockStore_Recommendations(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	store := NewStockStore(db)

	today := time.Now().Format("2006-01-02")
	defer db.Exec("DELETE FROM stock_recommendations WHERE analysis_date = ? AND stock_code = '600519'", today)

	rec := &model.StockRecommendation{
		StockCode:    "600519",
		StockName:    "贵州茅台",
		Source:       "hot",
		BuyScore:     8,
		BuyReason:    "test buy reason",
		TailScore:    6,
		TailReason:   "test tail reason",
		AnalysisDate: today,
	}
	err := store.UpsertRecommendation(rec)
	if err != nil {
		t.Fatalf("UpsertRecommendation: %v", err)
	}

	recs, err := store.GetTodayRecommendations("", 0)
	if err != nil {
		t.Fatalf("GetTodayRecommendations: %v", err)
	}
	found := false
	for _, r := range recs {
		if r.StockCode == "600519" && r.BuyScore == 8 {
			found = true
		}
	}
	if !found {
		t.Error("expected to find recommendation for 600519")
	}
}
