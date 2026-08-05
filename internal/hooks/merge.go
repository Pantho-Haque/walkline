package hooks

import (
	"os"
	"path/filepath"
	"strings"
)

type HookStatus int

const (
	HookFresh HookStatus = iota
	HookMerged
	HookNoOp
)

func InstallHook(hookPath string) (HookStatus, error) {
	dir := filepath.Dir(hookPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, err
	}

	fi, err := os.Stat(hookPath)
	if os.IsNotExist(err) {
		if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
			return 0, err
		}
		return HookFresh, nil
	}
	if err != nil {
		return 0, err
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		return 0, err
	}

	if strings.Contains(string(content), "walkline log-commit") {
		return HookNoOp, nil
	}

	return HookMerged, appendToHook(hookPath, string(content), fi.Mode())
}

func appendToHook(hookPath, existingContent string, mode os.FileMode) error {
	merged := existingContent + "\nwalkline log-commit\n"
	return os.WriteFile(hookPath, []byte(merged), mode)
}
