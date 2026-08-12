package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	if results.Fresh != 2 {
		t.Errorf("expected 2 fresh (post-commit + pre-push), got %d", results.Fresh)
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
	if results.Fresh != 1 {
		t.Errorf("expected 1 fresh (pre-push), got %d", results.Fresh)
	}
}

func TestScannerMergesExistingPrePushHook(t *testing.T) {
	tmp := t.TempDir()

	repoDir := filepath.Join(tmp, "repo-prepush")
	hookDir := filepath.Join(repoDir, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		t.Fatal(err)
	}

	existingHook := `#!/bin/sh
echo "my custom pre-push"
`
	if err := os.WriteFile(filepath.Join(hookDir, "pre-push"), []byte(existingHook), 0755); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(tmp, 1)
	results, err := scanner.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if results.Merged != 1 {
		t.Errorf("expected 1 merged (pre-push), got %d", results.Merged)
	}
	if results.Fresh != 1 {
		t.Errorf("expected 1 fresh (post-commit), got %d", results.Fresh)
	}

	content, err := os.ReadFile(filepath.Join(hookDir, "pre-push"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "my custom pre-push") {
		t.Error("existing custom hook content should be preserved")
	}
	if !strings.Contains(string(content), PrePushHookMarker()) {
		t.Error("walkline pre-push marker should be appended")
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

	prePushHook := "#!/bin/sh\n" + PrePushHook()
	if err := os.WriteFile(filepath.Join(hookDir, "pre-push"), []byte(prePushHook), 0755); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(tmp, 1)
	results, err := scanner.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if results.NoOp != 2 {
		t.Errorf("expected 2 noop (post-commit + pre-push), got %d", results.NoOp)
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
	if results1.Fresh+results2.Fresh != 2 {
		t.Errorf("should only install hooks once: first_fresh=%d, second_fresh=%d", results1.Fresh, results2.Fresh)
	}
	if results2.NoOp != 2 {
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

func TestPrePushHookStdinParsing(t *testing.T) {
	hookContent := `#!/bin/sh
WALKLINE=""
for dir in "$HOME/.local/bin" "/usr/local/bin" "/opt/homebrew/bin"; do
    if [ -x "$dir/walkline" ]; then
        WALKLINE="$dir/walkline"
        break
    fi
done
if [ -z "$WALKLINE" ]; then
    WALKLINE=$(command -v walkline 2>/dev/null)
fi

echo "START"
while read -r local_ref local_sha remote_ref remote_sha; do
    echo "REF:$local_ref:$local_sha:$remote_ref:$remote_sha"
    if [ "$remote_sha" = "0000000000000000000000000000000000000000" ]; then
        echo "SKIP_DELETE"
        continue
    fi
    if [ "$local_sha" = "$remote_sha" ]; then
        echo "SKIP_NOOP"
        continue
    fi
    echo "PUSH:$remote_sha..$local_sha:$remote_ref"
done
echo "END"
exit 0
`

	tmp := t.TempDir()
	hookPath := filepath.Join(tmp, "pre-push-test")
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name: "normal push with updated ref",
			input: "refs/heads/main abc123def refs/heads/main 0000000000000000000000000000000000000000\n",
			expect: []string{
				"START",
				"REF:refs/heads/main:abc123def:refs/heads/main:0000000000000000000000000000000000000000",
				"SKIP_DELETE",
				"END",
			},
		},
		{
			name: "no-op push (same sha)",
			input: "refs/heads/main abc123def refs/heads/main abc123def\n",
			expect: []string{
				"START",
				"REF:refs/heads/main:abc123def:refs/heads/main:abc123def",
				"SKIP_NOOP",
				"END",
			},
		},
		{
			name: "multiple refs mixed",
			input: "refs/heads/main aaa111 refs/heads/main bbb222\nrefs/heads/feature ccc333 refs/heads/feature 0000000000000000000000000000000000000000\nrefs/heads/dev ddd444 refs/heads/dev ddd444\n",
			expect: []string{
				"START",
				"REF:refs/heads/main:aaa111:refs/heads/main:bbb222",
				"PUSH:bbb222..aaa111:refs/heads/main",
				"REF:refs/heads/feature:ccc333:refs/heads/feature:0000000000000000000000000000000000000000",
				"SKIP_DELETE",
				"REF:refs/heads/dev:ddd444:refs/heads/dev:ddd444",
				"SKIP_NOOP",
				"END",
			},
		},
		{
			name:   "empty input (no refs)",
			input:  "",
			expect: []string{"START", "END"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("sh", hookPath)
			cmd.Stdin = strings.NewReader(tt.input)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("hook failed: %v\noutput: %s", err, out)
			}

			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) != len(tt.expect) {
				t.Fatalf("expected %d lines, got %d:\ngot: %v\nwant: %v", len(tt.expect), len(lines), lines, tt.expect)
			}
			for i, line := range lines {
				if line != tt.expect[i] {
					t.Errorf("line %d: got %q, want %q", i, line, tt.expect[i])
				}
			}
		})
	}
}
