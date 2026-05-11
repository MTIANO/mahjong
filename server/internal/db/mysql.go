package db

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

func InitMySQL(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}
	return db, nil
}

func createTables(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			openid VARCHAR(64) NOT NULL UNIQUE,
			nickname VARCHAR(50) DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS watchlist (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			user_id BIGINT NOT NULL,
			stock_code VARCHAR(10) NOT NULL,
			stock_name VARCHAR(50) DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_user_stock (user_id, stock_code)
		)`,
		`CREATE TABLE IF NOT EXISTS stock_recommendations (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			stock_code VARCHAR(10) NOT NULL,
			stock_name VARCHAR(50) DEFAULT '',
			source VARCHAR(10) NOT NULL,
			buy_score TINYINT DEFAULT 0,
			buy_reason TEXT,
			tail_score TINYINT DEFAULT 0,
			tail_reason TEXT,
			analysis_date DATE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_code_source_date (stock_code, source, analysis_date)
		)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}

	migrations := []string{
		"ALTER TABLE stock_recommendations ADD COLUMN key_signals VARCHAR(100) DEFAULT '' AFTER tail_reason",
		"ALTER TABLE stock_recommendations ADD COLUMN risk_level TINYINT DEFAULT 0 AFTER key_signals",
		"ALTER TABLE stock_recommendations ADD COLUMN trap_warning VARCHAR(100) DEFAULT '' AFTER risk_level",
	}
	for _, m := range migrations {
		db.Exec(m)
	}

	return nil
}
