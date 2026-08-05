package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScannerFreshRepo(t *testing.T) {
	tmp := t.TempDir()

	repoDir := filepath.Join(tmp, "repo1")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git", "hooks"), 0755); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(tmp, 1)
	results, err := scanner.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if results.Total != 1 {
		t.Errorf("expected 1 total, got %d", results.Total)
	}
	if results.Fresh != 1 {
		t.Errorf("expected 1 fresh, got %d", results.Fresh)
	}
	if results.Merged != 0 {
		t.Errorf("expected 0 merged, got %d", results.Merged)
	}
	if results.NoOp != 0 {
		t.Errorf("expected 0 noop, got %d", results.NoOp)
	}
}

func TestScannerMergesExistingHook(t *testing.T) {
	tmp := t.TempDir()

	repoDir := filepath.Join(tmp, "repo2")
	hookDir := filepath.Join(repoDir, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		t.Fatal(err)
	}

	existingHook := `#!/bin/sh
echo "existing hook"
`
	if err := os.WriteFile(filepath.Join(hookDir, "post-commit"), []byte(existingHook), 0755); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(tmp, 1)
	results, err := scanner.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if results.Merged != 1 {
		t.Errorf("expected 1 merged, got %d", results.Merged)
	}
	if results.Fresh != 0 {
		t.Errorf("expected 0 fresh, got %d", results.Fresh)
	}
}

func TestScannerNoOpForAlreadyInstalled(t *testing.T) {
	tmp := t.TempDir()

	repoDir := filepath.Join(tmp, "repo3")
	hookDir := filepath.Join(repoDir, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		t.Fatal(err)
	}

	existingHook := `#!/bin/sh
walkline log-commit
`
	if err := os.WriteFile(filepath.Join(hookDir, "post-commit"), []byte(existingHook), 0755); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(tmp, 1)
	results, err := scanner.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if results.NoOp != 1 {
		t.Errorf("expected 1 noop, got %d", results.NoOp)
	}
	if results.Fresh != 0 {
		t.Errorf("expected 0 fresh, got %d", results.Fresh)
	}
	if results.Merged != 0 {
		t.Errorf("expected 0 merged, got %d", results.Merged)
	}
}

func TestScannerIdempotent(t *testing.T) {
	tmp := t.TempDir()

	repoDir := filepath.Join(tmp, "repo4")
	hookDir := filepath.Join(repoDir, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(tmp, 1)

	results1, err := scanner.Scan()
	if err != nil {
		t.Fatal(err)
	}

	results2, err := scanner.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if results1.Total != results2.Total {
		t.Errorf("total repos should stay same: first=%d, second=%d", results1.Total, results2.Total)
	}
	if results1.Fresh+results2.Fresh != 1 {
		t.Errorf("should only install hook once: first_fresh=%d, second_fresh=%d", results1.Fresh, results2.Fresh)
	}
	if results2.NoOp != 1 {
		t.Errorf("second scan should mark as noop: got %d", results2.NoOp)
	}
}

func TestScannerSkipsNonRepos(t *testing.T) {
	tmp := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmp, "not-a-repo"), 0755); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(tmp, 1)
	results, err := scanner.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if results.Total != 0 {
		t.Errorf("expected 0 repos, got %d", results.Total)
	}
}
