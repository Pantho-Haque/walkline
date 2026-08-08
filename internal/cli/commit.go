package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"walkline/internal/store"
)

func LogCommitCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "log-commit",
		Short:   "Record a commit to the database",
		Example: "walkline log-commit",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.New()
			if err != nil {
				return err
			}
			defer s.Close()

			commit, err := getCommitInfo(".")
			if err != nil {
				return err
			}
			if err := s.InsertCommit(commit); err != nil {
				return err
			}
			return cleanupOrphans(s, commit.ProjectPath)
		},
	}
}

func getCommitInfo(repoPath string) (*store.Commit, error) {
	commit := &store.Commit{ProjectPath: repoPath}

	absPath, err := runGit(repoPath, "rev-parse", "--show-toplevel")
	if err == nil {
		commit.ProjectPath = strings.TrimSpace(absPath)
	}

	hash, err := runGit(repoPath, "log", "-1", "--format=%H")
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	commit.Hash = strings.TrimSpace(hash)

	authorName, err := runGit(repoPath, "log", "-1", "--format=%an")
	if err != nil {
		return nil, fmt.Errorf("git log author: %w", err)
	}
	commit.AuthorName = strings.TrimSpace(authorName)

	authorEmail, err := runGit(repoPath, "log", "-1", "--format=%ae")
	if err != nil {
		return nil, fmt.Errorf("git log email: %w", err)
	}
	commit.AuthorEmail = strings.TrimSpace(authorEmail)

	message, err := runGit(repoPath, "log", "-1", "--format=%B")
	if err != nil {
		return nil, fmt.Errorf("git log message: %w", err)
	}
	commit.Message = strings.TrimSpace(message)

	committedAt, err := runGit(repoPath, "log", "-1", "--format=%aI")
	if err != nil {
		return nil, fmt.Errorf("git log date: %w", err)
	}
	commit.CommittedAt = strings.TrimSpace(committedAt)

	remoteURL, _ := runGit(repoPath, "remote", "get-url", "origin")
	remoteURL = strings.TrimSpace(remoteURL)
	commit.RemoteURL = remoteURL

	if remoteURL != "" {
		parts := strings.Split(remoteURL, "/")
		name := parts[len(parts)-1]
		commit.ProjectName = strings.TrimSuffix(name, ".git")
	} else {
		repoRoot, _ := runGit(repoPath, "rev-parse", "--show-toplevel")
		repoRoot = strings.TrimSpace(repoRoot)
		commit.ProjectName = filepath.Base(repoRoot)
	}

	return commit, nil
}

func runGit(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func cleanupOrphans(s *store.Store, repoPath string) error {
	reachable, err := runGit(repoPath, "rev-list", "HEAD")
	if err != nil {
		return err
	}
	hashes := strings.Split(strings.TrimSpace(reachable), "\n")
	var valid []string
	for _, h := range hashes {
		h = strings.TrimSpace(h)
		if h != "" {
			valid = append(valid, h)
		}
	}
	deleted, err := s.DeleteOrphans(repoPath, valid)
	if err != nil {
		return err
	}
	if deleted > 0 {
		fmt.Printf("Cleaned up %d orphaned commit(s)\n", deleted)
	}
	return nil
}

func MarkPushedCmd() *cobra.Command {
	var flagRange string
	var remoteRef string

	cmd := &cobra.Command{
		Use:     "mark-pushed [ref]",
		Short:   "Mark commits as pushed",
		Example: "walkline mark-pushed (auto-detect) or walkline mark-pushed origin/main..HEAD",
		Args:    cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.New()
			if err != nil {
				return err
			}
			defer s.Close()

			var ref string
			if flagRange != "" {
				ref = flagRange
			} else if len(args) > 0 {
				ref = args[0]
			} else {
				ref = "HEAD"
				upstream, err := runGit(".", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
				if err == nil && strings.TrimSpace(upstream) != "" {
					ref = strings.TrimSpace(upstream) + "..HEAD"
				}
			}

			hashes, err := resolvePushRange(".", ref)
			if err != nil {
				return fmt.Errorf("resolve range: %w", err)
			}
			if len(hashes) == 0 {
				return nil
			}
			return s.MarkPushed(hashes)
		},
	}
	cmd.Flags().StringVar(&flagRange, "range", "", "Git rev-list range (e.g. abc123..def456)")
	cmd.Flags().StringVar(&remoteRef, "remote-ref", "", "Remote ref (informational, for logging)")
	return cmd
}

func resolvePushRange(repoPath, refSpec string) ([]string, error) {
	hashes, err := runGit(repoPath, "rev-list", refSpec)
	if err != nil {
		return nil, err
	}
	if hashes == "" {
		return []string{}, nil
	}
	lines := strings.Split(strings.TrimSpace(hashes), "\n")
	result := make([]string, 0, len(lines))
	for _, h := range lines {
		h = strings.TrimSpace(h)
		if h != "" {
			result = append(result, h)
		}
	}
	return result, nil
}
