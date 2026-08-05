package hooks

import (
	"os"
	"path/filepath"
	"strings"
)

type ScanResults struct {
	Total      int
	Fresh      int
	Merged     int
	NoOp       int
	MergedPaths []string
}

type Scanner struct {
	rootDir string
	depth   int
}

func NewScanner(rootDir string, depth int) *Scanner {
	return &Scanner{rootDir: rootDir, depth: depth}
}

func (s *Scanner) Scan() (*ScanResults, error) {
	var results ScanResults

	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(s.rootDir, path)
		depth := len(filepath.SplitList(rel))
		if depth > s.depth {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			return nil
		}

		gitDir := findGitDir(path, info)
		if gitDir == "" {
			return nil
		}

		results.Total++
		hookPath := filepath.Join(gitDir, "hooks", "post-commit")

		status, err := InstallHook(hookPath)
		if err != nil {
			return err
		}

		switch status {
		case HookFresh:
			results.Fresh++
		case HookMerged:
			results.Merged++
			results.MergedPaths = append(results.MergedPaths, path)
		case HookNoOp:
			results.NoOp++
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &results, nil
}

func findGitDir(path string, info os.FileInfo) string {
	gitFolder := filepath.Join(path, ".git")
	gitFile := filepath.Join(path, ".git")

	if fi, err := os.Stat(gitFolder); err == nil && fi.IsDir() {
		return gitFolder
	}

	if fi, err := os.Stat(gitFile); err == nil && !fi.IsDir() {
		content, err := os.ReadFile(gitFile)
		if err == nil {
			lines := string(content)
			if len(lines) > 7 && lines[:7] == "gitdir:" {
				return strings.TrimSpace(lines[7:])
			}
		}
	}

	return ""
}
