package shared

import (
	"os/exec"
	"strings"
)

// RunGit runs a git command in the given directory and returns the output.
func RunGit(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// RunGitLines runs a git command and returns the output as a slice of non-empty lines.
func RunGitLines(repoPath string, args ...string) ([]string, error) {
	out, err := RunGit(repoPath, args...)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

// IsAncestor checks if commit is an ancestor of rev in the given repo.
func IsAncestor(repoPath, commit, rev string) (bool, error) {
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
