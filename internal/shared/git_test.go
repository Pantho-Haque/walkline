package shared

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// isolate keeps git away from the user's config/templates (which would fire the real walkline hook).
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, p := range []string{"AUTHOR", "COMMITTER"} {
		t.Setenv("GIT_"+p+"_NAME", "Tester")
		t.Setenv("GIT_"+p+"_EMAIL", "tester@example.com")
	}
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

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	return dir
}

func commit(t *testing.T, dir, name, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(msg+dir), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", name)
	git(t, dir, "commit", "-q", "-m", msg)
	return git(t, dir, "rev-parse", "HEAD")
}

func TestRunGit(t *testing.T) {
	isolate(t)
	dir := newRepo(t)
	h := commit(t, dir, "a", "first")

	out, err := RunGit(dir, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(out) != h {
		t.Fatalf("got %q, %v; want %s", out, err, h)
	}
	if _, err := RunGit(dir, "no-such-subcommand"); err == nil {
		t.Error("expected error for bad subcommand")
	}
	if _, err := RunGit(t.TempDir(), "rev-parse", "HEAD"); err == nil {
		t.Error("expected error outside a repo")
	}
	if _, err := RunGit(filepath.Join(dir, "missing"), "status"); err == nil {
		t.Error("expected error for missing dir")
	}
}

func TestRunGitLines(t *testing.T) {
	isolate(t)
	dir := newRepo(t)
	h1 := commit(t, dir, "a", "one")
	h2 := commit(t, dir, "b", "two")

	got, err := RunGitLines(dir, "rev-list", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != h2 || got[1] != h1 {
		t.Errorf("got %v", got)
	}

	// empty output -> no lines (no phantom empty entry)
	got, err = RunGitLines(dir, "rev-list", "HEAD..HEAD")
	if err != nil || len(got) != 0 {
		t.Errorf("empty range: got %v, %v", got, err)
	}

	if _, err := RunGitLines(dir, "rev-list", "nonexistent-ref"); err == nil {
		t.Error("expected error for bad ref")
	}
}

func TestIsAncestor(t *testing.T) {
	isolate(t)
	dir := newRepo(t)
	base := commit(t, dir, "a", "base")
	git(t, dir, "checkout", "-q", "-b", "side")
	side := commit(t, dir, "s", "side")
	git(t, dir, "checkout", "-q", "main")
	tip := commit(t, dir, "b", "tip")

	tests := []struct {
		name         string
		commit, rev  string
		want, wantEr bool
	}{
		{"parent of tip", base, tip, true, false},
		{"self", tip, tip, true, false},
		{"tip not ancestor of parent", tip, base, false, false},
		{"diverged branch", side, "main", false, false},
		{"base of side", base, "side", true, false},
		{"unknown commit", strings.Repeat("0", 40), tip, false, true},
		{"unknown rev", base, "no-such-branch", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsAncestor(dir, tt.commit, tt.rev)
			if (err != nil) != tt.wantEr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantEr)
			}
			if got != tt.want {
				t.Errorf("got %v want %v", got, tt.want)
			}
		})
	}

	if _, err := IsAncestor(t.TempDir(), base, tip); err == nil {
		t.Error("expected error outside a repo")
	}
}
