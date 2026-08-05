package store

import (
	"os"
	"os/exec"
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

func TestMarkPushedIdempotent(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	commit := &Commit{Hash: "idem1", ProjectName: "p1", ProjectPath: "/p1", CommittedAt: "2024-01-01T12:00:00Z"}
	if err := db.InsertCommit(commit); err != nil {
		t.Fatal(err)
	}

	if err := db.MarkPushed([]string{"idem1"}); err != nil {
		t.Fatal("first mark-pushed:", err)
	}

	if err := db.MarkPushed([]string{"idem1"}); err != nil {
		t.Fatal("second mark-pushed:", err)
	}

	result, err := db.GetCommitByHash("idem1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pushed {
		t.Error("commit should be pushed after double mark-pushed call")
	}
	if result.PushedAt == "" {
		t.Error("pushed_at should be set")
	}
}

func TestGetPendingProjects(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	commits := []Commit{
		{Hash: "h1", ProjectName: "p1", ProjectPath: "/proj1", CommittedAt: "2024-01-01T12:00:00Z"},
		{Hash: "h2", ProjectName: "p1", ProjectPath: "/proj1", CommittedAt: "2024-01-01T12:01:00Z"},
		{Hash: "h3", ProjectName: "p2", ProjectPath: "/proj2", CommittedAt: "2024-01-01T12:02:00Z"},
	}
	for _, c := range commits {
		if err := db.InsertCommit(&c); err != nil {
			t.Fatal(err)
		}
	}

	if err := db.MarkPushed([]string{"h1"}); err != nil {
		t.Fatal(err)
	}

	projects, err := db.GetPendingProjects()
	if err != nil {
		t.Fatal(err)
	}

	if len(projects) != 2 {
		t.Errorf("expected 2 pending projects, got %d", len(projects))
	}
}

func TestGetPendingHashes(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	commits := []Commit{
		{Hash: "h1", ProjectName: "p1", ProjectPath: "/proj1", CommittedAt: "2024-01-01T12:00:00Z"},
		{Hash: "h2", ProjectName: "p1", ProjectPath: "/proj1", CommittedAt: "2024-01-01T12:01:00Z"},
		{Hash: "h3", ProjectName: "p1", ProjectPath: "/proj1", CommittedAt: "2024-01-01T12:02:00Z"},
	}
	for _, c := range commits {
		if err := db.InsertCommit(&c); err != nil {
			t.Fatal(err)
		}
	}

	if err := db.MarkPushed([]string{"h2"}); err != nil {
		t.Fatal(err)
	}

	hashes, err := db.GetPendingHashes("/proj1")
	if err != nil {
		t.Fatal(err)
	}

	if len(hashes) != 2 {
		t.Errorf("expected 2 pending hashes, got %d", len(hashes))
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

func TestSyncIntegration(t *testing.T) {
	tmp := t.TempDir()

	repoDir := filepath.Join(tmp, "myrepo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmds := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial commit"},
	}
	for _, args := range cmds {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = repoDir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %v\n%s", args, err, out)
		}
	}

	hashOut, err := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	commitHash := string(hashOut[:len(hashOut)-1])

	remoteRef := filepath.Join(repoDir, ".git", "refs", "remotes", "origin", "main")
	if err := os.MkdirAll(filepath.Dir(remoteRef), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(remoteRef, []byte(commitHash+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(tmp, "test.db")
	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	commit := &Commit{
		Hash:        commitHash,
		ProjectName: "myrepo",
		ProjectPath: repoDir,
		CommittedAt: "2024-01-01T12:00:00Z",
		Pushed:      false,
	}
	if err := db.InsertCommit(commit); err != nil {
		t.Fatal(err)
	}

	result, err := db.GetCommitByHash(commitHash)
	if err != nil {
		t.Fatal(err)
	}
	if result.Pushed {
		t.Fatal("commit should be pending before sync")
	}

	hashes, err := db.GetPendingHashes(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(hashes) != 1 {
		t.Fatalf("expected 1 pending hash, got %d", len(hashes))
	}

	var toMark []string
	for _, h := range hashes {
		cmd := exec.Command("git", "-C", repoDir, "merge-base", "--is-ancestor", h, "refs/remotes/origin/main")
		err := cmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				continue
			}
			t.Logf("merge-base check failed: %v", err)
			continue
		}
		toMark = append(toMark, h)
	}

	if len(toMark) != 1 {
		t.Fatalf("expected 1 hash to mark, got %d", len(toMark))
	}

	if err := db.MarkPushed(toMark); err != nil {
		t.Fatal(err)
	}

	result, err = db.GetCommitByHash(commitHash)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pushed {
		t.Error("commit should be pushed after sync")
	}
	if result.PushedAt == "" {
		t.Error("pushed_at should be set after sync")
	}
}
