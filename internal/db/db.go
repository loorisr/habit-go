package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		dbPath = "data/habits.db"
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	dsn := dbPath
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// single connection for modernc sqlite to avoid busy errors and to keep pragmas per connection
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	// pragmas - must be set on the single connection
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA synchronous=NORMAL;`); err != nil {
		return nil, err
	}
	if err := Migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func Migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS habits (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			group_name TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL CHECK(type IN ('boolean','numerical')),
			goal_value REAL NOT NULL,
			goal_period TEXT NOT NULL CHECK(goal_period IN ('daily','weekly','monthly')),
			is_negative INTEGER NOT NULL DEFAULT 0,
			unit TEXT NOT NULL DEFAULT '',
			archived_at TEXT,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_habits_archived ON habits(archived_at);`,
		`CREATE INDEX IF NOT EXISTS idx_habits_group ON habits(group_name);`,
		`CREATE TABLE IF NOT EXISTS entries (
			id TEXT PRIMARY KEY,
			habit_id TEXT NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
			date TEXT NOT NULL,
			value REAL NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(habit_id, date)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_entries_habit_date ON entries(habit_id, date);`,
		`CREATE INDEX IF NOT EXISTS idx_entries_date ON entries(date);`,
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// migration: add unit column for existing DBs (idempotent)
	if _, err := db.Exec(`ALTER TABLE habits ADD COLUMN unit TEXT NOT NULL DEFAULT ''`); err != nil {
		if !isDuplicateColumnError(err) {
			return err
		}
	}
	return nil
}

func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists")
}
