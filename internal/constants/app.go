package constants

import (
	"os"
	"path/filepath"
)

// Version is the current walkline version
var Version = "dev"

// AppName is the name of the application
const AppName = "walkline"

// GitHooksDir is the directory name for git hooks
const GitHooksDir = "hooks"

// WalklineDir is the directory name for walkline data
const WalklineDir = ".walkline"

// DBFileName is the database file name
const DBFileName = "commits.db"

// DefaultScanDepth is the default depth for scanning repos
const DefaultScanDepth = 1

// DataDir returns the walkline data directory path
func DataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, WalklineDir)
}

// DBPath returns the database file path
func DBPath() string {
	return filepath.Join(DataDir(), DBFileName)
}

// TemplateDir returns the git template directory path
func TemplateDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".git-templates")
}

// HooksMarker is the marker string used to detect walkline hooks
const HooksMarker = "walkline"

const (
	// PostCommitHookContent is the content for post-commit hooks
	PostCommitHookContent = `#!/bin/sh
# walkline post-commit hook
# Automatically installed by walkline
walkline log-commit
`

	// PrePushHookMarker is the marker for pre-push hooks
	PrePushHookMarker = "walkline pre-push hook"

	// PrePushHookBody is the body of the pre-push hook
	PrePushHookBody = `# walkline pre-push hook
# Resolve walkline binary
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
if [ -z "$WALKLINE" ]; then
    exit 0
fi

while read -r local_ref local_sha remote_ref remote_sha; do
    if [ "$remote_sha" = "0000000000000000000000000000000000000000" ]; then
        continue
    fi
    if [ "$local_sha" = "$remote_sha" ]; then
        continue
    fi
    "$WALKLINE" mark-pushed --range "$remote_sha..$local_sha" --remote-ref "$remote_ref" 2>/dev/null || true
done
exit 0
`
)
