package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"walkline/internal/hooks"
	"walkline/internal/store"
)

// AutoSync scans the given directory and syncs all repos.
func AutoSync(home string) error {
	scanner := hooks.NewScanner(home, 2)
	results, err := scanner.Scan()
	if results == nil {
		return err
	}

	s, err := store.New()
	if err != nil {
		return err
	}
	defer s.Close()

	synced := 0
	skipped := 0

	fmt.Println("Syncing repos:")
	for _, repoPath := range results.Repos {
		if err := SyncRepo(s, repoPath); err != nil {
			skipped++
			continue
		}
		synced++
		fmt.Printf("  %s\n", repoPath)
	}

	if skipped > 0 {
		fmt.Printf("Auto-sync complete: %d synced, %d skipped (no upstream or permission)\n", synced, skipped)
	} else {
		fmt.Printf("Auto-sync complete: %d repos synced\n", synced)
	}
	return nil
}

// DefaultDataDir returns the default walkline data directory.
func DefaultDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".walkline")
}
