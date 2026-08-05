package store

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".walkline")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return NewStore(filepath.Join(dir, "commits.db"))
}

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS commits (
		hash TEXT PRIMARY KEY,
		project_name TEXT NOT NULL,
		project_path TEXT NOT NULL,
		remote_url TEXT,
		author_name TEXT,
		author_email TEXT,
		message TEXT,
		committed_at TEXT NOT NULL,
		pushed INTEGER NOT NULL DEFAULT 0,
		pushed_at TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_commits_project ON commits(project_name);
	CREATE INDEX IF NOT EXISTS idx_commits_pushed ON commits(pushed);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}
