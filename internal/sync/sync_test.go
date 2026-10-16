package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"walkline/internal/store"
)

func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, p := range []string{"AUTHOR", "COMMITTER"} {
		t.Setenv("GIT_"+p+"_NAME", "Tester")
		t.Setenv("GIT_"+p+"_EMAIL", "tester@example.com")
	}
	return home
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newRepo(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-q", "-b", "main")
	return dir
}

func commit(t *testing.T, dir, name string) string {
	t.Helper()
	os.WriteFile(filepath.Join(dir, name), []byte(name+dir), 0o644)
	git(t, dir, "add", name)
	git(t, dir, "commit", "-q", "-m", name)
	return git(t, dir, "rev-parse", "HEAD")
}

// withRemote creates a bare remote and wires repo's main to it.
func withRemote(t *testing.T, repo string) {
	t.Helper()
	bare := filepath.Join(t.TempDir(), "remote.git")
	git(t, repo, "init", "-q", "--bare", bare)
	git(t, repo, "remote", "add", "origin", bare)
}

func record(t *testing.T, s *store.Store, repo, hash string) {
	t.Helper()
	err := s.InsertCommit(&store.Commit{Hash: hash, ProjectName: filepath.Base(repo), ProjectPath: repo,
		AuthorName: "Tester", CommittedAt: "2026-01-01T00:00:00Z", Message: "m"})
	if err != nil {
		t.Fatal(err)
	}
}

func pushed(t *testing.T, s *store.Store, hash string) bool {
	t.Helper()
	c, err := s.GetCommitByHash(hash)
	if err != nil || c == nil {
		t.Fatalf("commit %s: %v %v", hash, c, err)
	}
	return c.Pushed
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.NewStore(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSyncRepo_NoUpstream(t *testing.T) {
	isolate(t)
	repo := newRepo(t, t.TempDir())
	h := commit(t, repo, "a")
	s := newStore(t)
	record(t, s, repo, h)

	err := SyncRepo(s, repo)
	if err == nil || !strings.Contains(err.Error(), "no upstream") {
		t.Fatalf("want no-upstream error, got %v", err)
	}
	if pushed(t, s, h) {
		t.Error("commit must stay pending without upstream")
	}
}

func TestSyncRepo_MarksOnlyPushedAncestors(t *testing.T) {
	isolate(t)
	repo := newRepo(t, t.TempDir())
	withRemote(t, repo)
	h1 := commit(t, repo, "a")
	h2 := commit(t, repo, "b")
	git(t, repo, "push", "-q", "-u", "origin", "main")
	h3 := commit(t, repo, "c") // local only

	s := newStore(t)
	for _, h := range []string{h1, h2, h3} {
		record(t, s, repo, h)
	}
	if err := SyncRepo(s, repo); err != nil {
		t.Fatal(err)
	}
	for h, want := range map[string]bool{h1: true, h2: true, h3: false} {
		if got := pushed(t, s, h); got != want {
			t.Errorf("%s pushed=%v want %v", h[:7], got, want)
		}
	}

	// push the rest, sync again, everything flips
	git(t, repo, "push", "-q")
	if err := SyncRepo(s, repo); err != nil {
		t.Fatal(err)
	}
	if !pushed(t, s, h3) {
		t.Error("h3 should be pushed after second sync")
	}
}

func TestSyncRepo_UnknownHashIsSkipped(t *testing.T) {
	isolate(t)
	repo := newRepo(t, t.TempDir())
	withRemote(t, repo)
	h := commit(t, repo, "a")
	git(t, repo, "push", "-q", "-u", "origin", "main")

	s := newStore(t)
	ghost := strings.Repeat("a", 40) // recorded, but not in this repo (e.g. rewritten history)
	record(t, s, repo, ghost)
	record(t, s, repo, h)
	if err := SyncRepo(s, repo); err != nil {
		t.Fatal(err)
	}
	if pushed(t, s, ghost) {
		t.Error("unknown commit must not be marked pushed")
	}
	if !pushed(t, s, h) {
		t.Error("real commit should still be marked")
	}
}

func TestSyncPending(t *testing.T) {
	isolate(t)
	s := newStore(t)

	ok := newRepo(t, t.TempDir())
	withRemote(t, ok)
	hok := commit(t, ok, "a")
	git(t, ok, "push", "-q", "-u", "origin", "main")
	record(t, s, ok, hok)

	noUp := newRepo(t, t.TempDir())
	record(t, s, noUp, commit(t, noUp, "a"))

	gone := filepath.Join(t.TempDir(), "deleted")
	record(t, s, gone, strings.Repeat("b", 40))

	synced, skipped, err := SyncPending(s)
	if err != nil {
		t.Fatal(err)
	}
	if synced != 1 || skipped != 2 {
		t.Errorf("synced=%d skipped=%d, want 1 and 2", synced, skipped)
	}
	if !pushed(t, s, hok) {
		t.Error("synced repo's commit should be pushed")
	}

	// nothing pending -> zero/zero
	s2 := newStore(t)
	if a, b, err := SyncPending(s2); err != nil || a != 0 || b != 0 {
		t.Errorf("empty store: %d %d %v", a, b, err)
	}
}

func TestAutoSync(t *testing.T) {
	home := isolate(t)
	repo := newRepo(t, filepath.Join(home, "proj"))
	withRemote(t, repo)
	h := commit(t, repo, "a")
	git(t, repo, "push", "-q", "-u", "origin", "main")
	noUp := newRepo(t, filepath.Join(home, "other"))
	commit(t, noUp, "a")
	newRepo(t, filepath.Join(home, "x")) // empty repo, no commits

	s, err := store.New() // honours HOME
	if err != nil {
		t.Fatal(err)
	}
	record(t, s, repo, h)
	s.Close()

	if err := AutoSync(home); err != nil {
		t.Fatal(err)
	}

	s, _ = store.New()
	defer s.Close()
	if !pushed(t, s, h) {
		t.Error("AutoSync should mark the pushed commit")
	}
	// hooks were installed into discovered repos
	if b, err := os.ReadFile(filepath.Join(repo, ".git", "hooks", "post-commit")); err != nil || !strings.Contains(string(b), "walkline log-commit") {
		t.Errorf("post-commit hook missing: %v", err)
	}
}

func TestDefaultDataDir(t *testing.T) {
	home := isolate(t)
	if got := DefaultDataDir(); got != filepath.Join(home, ".walkline") {
		t.Errorf("got %s", got)
	}
}
