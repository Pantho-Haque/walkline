package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"walkline/internal/store"
)

func SyncCmd() *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:     "sync [--project=<name>]",
		Short:   "Reconcile pending commits against upstream",
		Example: "walkline sync --project=myrepo",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.New()
			if err != nil {
				return err
			}
			defer s.Close()

			return runSync(s, project)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Sync only this project")
	return cmd
}

func runSync(s *store.Store, projectFilter string) error {
	var projectPaths []string
	if projectFilter != "" {
		projectPaths = []string{projectFilter}
	} else {
		paths, err := s.GetPendingProjects()
		if err != nil {
			return err
		}
		projectPaths = paths
	}

	synced := 0
	for _, projectPath := range projectPaths {
		n, err := syncProject(s, projectPath)
		if err != nil {
			fmt.Printf("  sync %s: %v (skipped)\n", projectPath, err)
			continue
		}
		synced += n
	}

	if synced > 0 {
		fmt.Printf("Synced %d commit(s) to pushed status\n", synced)
	}
	return nil
}

func syncProject(s *store.Store, projectPath string) (int, error) {
	if err := cleanupOrphans(s, projectPath); err != nil {
		fmt.Printf("  orphan cleanup warning for %s: %v\n", projectPath, err)
	}

	upstream, err := runGit(projectPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return 0, fmt.Errorf("no upstream configured")
	}
	upstream = strings.TrimSpace(upstream)

	hashes, err := s.GetPendingHashes(projectPath)
	if err != nil {
		return 0, err
	}
	if len(hashes) == 0 {
		return 0, nil
	}

	var toMark []string
	for _, h := range hashes {
		isAncestor, err := gitIsAncestor(projectPath, h, upstream)
		if err != nil {
			continue
		}
		if isAncestor {
			toMark = append(toMark, h)
		}
	}

	if len(toMark) > 0 {
		if err := s.MarkPushed(toMark); err != nil {
			return 0, err
		}
	}
	return len(toMark), nil
}

func gitIsAncestor(repoPath, commit, rev string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", commit, rev)
	cmd.Dir = repoPath
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
