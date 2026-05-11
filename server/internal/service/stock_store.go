package service

import (
	"database/sql"
	"time"

	"github.com/mtiano/server/internal/model"
)

type StockStore struct {
	db *sql.DB
}

func NewStockStore(db *sql.DB) *StockStore {
	return &StockStore{db: db}
}

func (s *StockStore) FindOrCreateUser(openid string) (*model.User, error) {
	var user model.User
	err := s.db.QueryRow("SELECT id, openid, nickname, created_at, updated_at FROM users WHERE openid = ?", openid).
		Scan(&user.ID, &user.OpenID, &user.Nickname, &user.CreatedAt, &user.UpdatedAt)
	if err == nil {
		return &user, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	result, err := s.db.Exec("INSERT INTO users (openid) VALUES (?)", openid)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &model.User{ID: id, OpenID: openid, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (s *StockStore) AddWatchlistItem(userID int64, code, name string) error {
	_, err := s.db.Exec(
		"INSERT IGNORE INTO watchlist (user_id, stock_code, stock_name) VALUES (?, ?, ?)",
		userID, code, name,
	)
	return err
}

func (s *StockStore) RemoveWatchlistItem(userID int64, code string) error {
	_, err := s.db.Exec("DELETE FROM watchlist WHERE user_id = ? AND stock_code = ?", userID, code)
	return err
}

func (s *StockStore) GetWatchlist(userID int64) ([]model.WatchlistItem, error) {
	rows, err := s.db.Query(
		"SELECT id, user_id, stock_code, stock_name, created_at FROM watchlist WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.WatchlistItem
	for rows.Next() {
		var item model.WatchlistItem
		if err := rows.Scan(&item.ID, &item.UserID, &item.StockCode, &item.StockName, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *StockStore) GetAllWatchlistCodes() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT stock_code FROM watchlist")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}

func (s *StockStore) UpsertRecommendation(rec *model.StockRecommendation) error {
	_, err := s.db.Exec(`
		INSERT INTO stock_recommendations (stock_code, stock_name, source, buy_score, buy_reason, tail_score, tail_reason, key_signals, risk_level, trap_warning, analysis_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			stock_name = VALUES(stock_name),
			buy_score = VALUES(buy_score),
			buy_reason = VALUES(buy_reason),
			tail_score = VALUES(tail_score),
			tail_reason = VALUES(tail_reason),
			key_signals = VALUES(key_signals),
			risk_level = VALUES(risk_level),
			trap_warning = VALUES(trap_warning),
			updated_at = CURRENT_TIMESTAMP`,
		rec.StockCode, rec.StockName, rec.Source, rec.BuyScore, rec.BuyReason, rec.TailScore, rec.TailReason, rec.KeySignals, rec.RiskLevel, rec.TrapWarning, rec.AnalysisDate,
	)
	return err
}

func (s *StockStore) GetTodayRecommendations(source string, userID int64) ([]model.StockRecommendation, error) {
	today := time.Now().Format("2006-01-02")
	query := "SELECT id, stock_code, stock_name, source, buy_score, buy_reason, tail_score, tail_reason, COALESCE(key_signals, '') as key_signals, COALESCE(risk_level, 0) as risk_level, COALESCE(trap_warning, '') as trap_warning, analysis_date, created_at, updated_at FROM stock_recommendations WHERE analysis_date = ?"
	args := []any{today}

	if source != "" {
		query += " AND source = ?"
		args = append(args, source)
	}
	if source == "watchlist" && userID > 0 {
		query += " AND stock_code IN (SELECT stock_code FROM watchlist WHERE user_id = ?)"
		args = append(args, userID)
	}
	query += " ORDER BY buy_score DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recs []model.StockRecommendation
	for rows.Next() {
		var r model.StockRecommendation
		if err := rows.Scan(&r.ID, &r.StockCode, &r.StockName, &r.Source, &r.BuyScore, &r.BuyReason, &r.TailScore, &r.TailReason, &r.KeySignals, &r.RiskLevel, &r.TrapWarning, &r.AnalysisDate, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		recs = append(recs, r)
	}
	return recs, nil
}
