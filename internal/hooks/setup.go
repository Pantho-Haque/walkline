package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"walkline/internal/constants"
)

// SetupTemplateDir sets up the global git template directory with walkline hooks.
func SetupTemplateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	templateDir := filepath.Join(home, ".git-templates", constants.GitHooksDir)
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return "", fmt.Errorf("create template dir: %w", err)
	}

	postCommitPath := filepath.Join(templateDir, "post-commit")
	if err := os.WriteFile(postCommitPath, []byte(PostCommitHook()), 0755); err != nil {
		return "", fmt.Errorf("write post-commit hook: %w", err)
	}

	prePushPath := filepath.Join(templateDir, "pre-push")
	if err := os.WriteFile(prePushPath, []byte(PrePushHook()), 0755); err != nil {
		return "", fmt.Errorf("write pre-push hook: %w", err)
	}

	cmd := exec.Command("git", "config", "--global", "init.templateDir", filepath.Join(home, ".git-templates"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git config failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return templateDir, nil
}
