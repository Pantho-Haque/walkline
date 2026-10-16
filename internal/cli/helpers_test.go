package cli

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"walkline/internal/store"
)

// isolate points HOME at a temp dir and neutralises global git config so no test can
// touch the real ~/.walkline or fire the user's installed walkline hooks.
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

// newRepo creates a repo named name under a fresh temp dir; the path is symlink-resolved
// so it equals what `git rev-parse --show-toplevel` reports (macOS /var -> /private/var).
func newRepo(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, _ = filepath.EvalSymlinks(dir)
	git(t, dir, "init", "-q", "-b", "main")
	return dir
}

func commit(t *testing.T, dir, file, msg string) string {
	t.Helper()
	// content includes dir so same-second commits in different repos never collide on hash
	os.WriteFile(filepath.Join(dir, file), []byte(msg+dir), 0o644)
	git(t, dir, "add", file)
	git(t, dir, "commit", "-q", "-m", msg)
	return git(t, dir, "rev-parse", "HEAD")
}

func addRemote(t *testing.T, repo string) {
	t.Helper()
	bare := filepath.Join(t.TempDir(), "remote.git")
	git(t, repo, "init", "-q", "--bare", bare)
	git(t, repo, "remote", "add", "origin", bare)
}

// chdir moves into dir for the test (t.Chdir needs Go 1.24; CI pins 1.21).
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

// captureStdout runs fn and returns everything it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	func() {
		defer func() { os.Stdout = old; w.Close() }()
		fn()
	}()
	return <-done
}

// run executes cmd with args, returning cobra-routed output, os.Stdout output and the error.
func run(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	var err error
	std := captureStdout(t, func() { err = cmd.Execute() })
	return buf.String() + std, err
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New() // honours HOME
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func pushedState(t *testing.T, hash string) bool {
	t.Helper()
	s, err := store.New()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c, err := s.GetCommitByHash(hash)
	if err != nil || c == nil {
		t.Fatalf("commit %s not found: %v", hash, err)
	}
	return c.Pushed
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	func() { defer func() { os.Stderr = old; w.Close() }(); fn() }()
	return <-done
}
