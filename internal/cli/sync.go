package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"walkline/internal/shared"
	"walkline/internal/store"
	"walkline/internal/sync"
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
		paths, err := s.GetPathsByProjectName(projectFilter)
		if err != nil {
			return err
		}
		projectPaths = paths
	} else {
		paths, err := s.GetPendingProjects()
		if err != nil {
			return err
		}
		projectPaths = paths
	}

	synced := 0
	for _, projectPath := range projectPaths {
		if _, err := os.Stat(projectPath); os.IsNotExist(err) {
			continue
		}
		if err := cleanupOrphans(s, projectPath); err != nil {
			fmt.Printf("  orphan cleanup warning for %s: %v\n", projectPath, err)
		}
		if err := sync.SyncRepo(s, projectPath); err != nil {
			fmt.Printf("  sync %s: %v (skipped)\n", projectPath, err)
			continue
		}
		synced++
	}

	if synced > 0 {
		fmt.Printf("Synced %d commit(s) to pushed status\n", synced)
	}
	return nil
}

func cleanupOrphans(s *store.Store, repoPath string) error {
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil
	}
	reachable, err := shared.RunGitLines(repoPath, "rev-list", "HEAD")
	if err != nil {
		return err
	}
	deleted, err := s.DeleteOrphans(repoPath, reachable)
	if err != nil {
		return err
	}
	if deleted > 0 {
		fmt.Printf("Cleaned up %d orphaned commit(s)\n", deleted)
	}
	return nil
}
