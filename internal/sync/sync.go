package sync

import (
	"fmt"
	"os"
	"strings"

	"walkline/internal/shared"
	"walkline/internal/store"
)

// SyncRepo marks commits as pushed if they are ancestors of the upstream branch.
func SyncRepo(s *store.Store, repoPath string) error {
	upstream, err := shared.RunGit(repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return fmt.Errorf("no upstream configured")
	}
	upstream = strings.TrimSpace(upstream)

	hashes, err := s.GetPendingHashes(repoPath)
	if err != nil {
		return err
	}

	if len(hashes) == 0 {
		return nil
	}

	// One rev-list instead of one `merge-base --is-ancestor` process per pending commit.
	reachable, err := shared.RunGitLines(repoPath, "rev-list", upstream)
	if err != nil {
		return err
	}
	inUpstream := make(map[string]bool, len(reachable))
	for _, h := range reachable {
		inUpstream[h] = true
	}
	var toMark []string
	for _, h := range hashes {
		if inUpstream[h] {
			toMark = append(toMark, h)
		}
	}

	if len(toMark) > 0 {
		return s.MarkPushed(toMark)
	}
	return nil
}

// SyncPending reconciles every project that still has pending commits, without printing.
// It returns how many repos were reconciled and how many were skipped (missing or no upstream).
func SyncPending(s *store.Store) (synced, skipped int, err error) {
	paths, err := s.GetPendingProjects()
	if err != nil {
		return 0, 0, err
	}
	for _, p := range paths {
		if _, statErr := os.Stat(p); statErr != nil {
			skipped++
			continue
		}
		if SyncRepo(s, p) != nil {
			skipped++
			continue
		}
		synced++
	}
	return synced, skipped, nil
}
