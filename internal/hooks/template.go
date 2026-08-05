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

func SetupTemplateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	templateDir := filepath.Join(home, ".git-templates", "hooks")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return "", fmt.Errorf("create template dir: %w", err)
	}

	hookPath := filepath.Join(templateDir, "post-commit")
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return "", fmt.Errorf("write hook: %w", err)
	}

	cmd := exec.Command("git", "config", "--global", "init.templateDir", filepath.Join(home, ".git-templates"))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git config: %w", err)
	}

	return templateDir, nil
}
