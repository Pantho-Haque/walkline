package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInsertIdempotency(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	commit := &Commit{
		Hash:        "abc123",
		ProjectName: "test-project",
		ProjectPath: "/path/to/project",
		RemoteURL:   "https://github.com/user/test-project.git",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		Message:     "Test commit",
		CommittedAt: "2024-01-01T12:00:00Z",
		Pushed:      false,
	}

	if err := db.InsertCommit(commit); err != nil {
		t.Fatal("first insert:", err)
	}

	if err := db.InsertCommit(commit); err != nil {
		t.Fatal("second insert:", err)
	}

	count, err := countRows(db)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 row after duplicate insert, got %d", count)
	}
}

func TestMarkPushed(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	commits := []Commit{
		{Hash: "hash1", ProjectName: "p1", ProjectPath: "/p1", CommittedAt: "2024-01-01T12:00:00Z"},
		{Hash: "hash2", ProjectName: "p1", ProjectPath: "/p1", CommittedAt: "2024-01-01T12:01:00Z"},
		{Hash: "hash3", ProjectName: "p1", ProjectPath: "/p1", CommittedAt: "2024-01-01T12:02:00Z"},
	}

	for _, c := range commits {
		if err := db.InsertCommit(&c); err != nil {
			t.Fatal(err)
		}
	}

	if err := db.MarkPushed([]string{"hash1", "hash2"}); err != nil {
		t.Fatal(err)
	}

	commits, err = db.QueryCommits(CommitFilter{})
	if err != nil {
		t.Fatal(err)
	}

	pushed := 0
	unpushed := 0
	for _, c := range commits {
		if c.Pushed {
			pushed++
		} else {
			unpushed++
		}
	}

	if pushed != 2 {
		t.Errorf("expected 2 pushed, got %d", pushed)
	}
	if unpushed != 1 {
		t.Errorf("expected 1 unpushed, got %d", unpushed)
	}
}

func TestMarkPushedOnlyAffectsSpecified(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	commits := []Commit{
		{Hash: "hash1", ProjectName: "p1", ProjectPath: "/p1", CommittedAt: "2024-01-01T12:00:00Z"},
		{Hash: "hash2", ProjectName: "p1", ProjectPath: "/p1", CommittedAt: "2024-01-01T12:01:00Z"},
	}

	for _, c := range commits {
		if err := db.InsertCommit(&c); err != nil {
			t.Fatal(err)
		}
	}

	if err := db.MarkPushed([]string{"hash1"}); err != nil {
		t.Fatal(err)
	}

	all, err := db.QueryCommits(CommitFilter{})
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range all {
		if c.Hash == "hash1" && !c.Pushed {
			t.Error("hash1 should be pushed")
		}
		if c.Hash == "hash2" && c.Pushed {
			t.Error("hash2 should not be pushed")
		}
	}
}

func openDB(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return NewStore(path)
}

func countRows(s *Store) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM commits").Scan(&count)
	return count, err
}
