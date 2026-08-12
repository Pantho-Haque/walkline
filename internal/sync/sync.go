package sync

import (
	"fmt"
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

	var toMark []string
	for _, h := range hashes {
		isAncestor, err := shared.IsAncestor(repoPath, h, upstream)
		if err != nil {
			continue
		}
		if isAncestor {
			toMark = append(toMark, h)
		}
	}

	if len(toMark) > 0 {
		return s.MarkPushed(toMark)
	}
	return nil
}
