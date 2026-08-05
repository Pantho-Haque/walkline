package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const hookContent = `#!/bin/sh
# walkline post-commit hook
# Automatically installed by walkline
walkline log-commit
`

const prePushHookContent = `#!/bin/sh
# walkline pre-push hook
# Automatically installed by walkline
` + prePushHookBody

const prePushHookBody = `# walkline pre-push hook
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

const prePushMarker = "walkline pre-push hook"

func SetupTemplateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	templateDir := filepath.Join(home, ".git-templates", "hooks")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return "", fmt.Errorf("create template dir: %w", err)
	}

	postCommitPath := filepath.Join(templateDir, "post-commit")
	if err := os.WriteFile(postCommitPath, []byte(hookContent), 0755); err != nil {
		return "", fmt.Errorf("write post-commit hook: %w", err)
	}

	prePushPath := filepath.Join(templateDir, "pre-push")
	if err := os.WriteFile(prePushPath, []byte(prePushHookContent), 0755); err != nil {
		return "", fmt.Errorf("write pre-push hook: %w", err)
	}

	cmd := exec.Command("git", "config", "--global", "init.templateDir", filepath.Join(home, ".git-templates"))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git config: %w", err)
	}

	return templateDir, nil
}
