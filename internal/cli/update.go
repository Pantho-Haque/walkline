package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"walkline/internal/hooks"
	"walkline/internal/store"
)

var (
	Version = "dev"
)

func UpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update",
		Short:   "Update walkline to the latest version",
		Example: "walkline update",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := doUpdate(); err != nil {
				return err
			}
			fmt.Println("\nRunning auto-sync...")
			return runAutoSync()
		},
	}
	return cmd
}

func doUpdate() error {
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not find current binary: %w", err)
	}

	fmt.Println("Checking for latest version...")
	latestVersion, err := fetchLatestVersion()
	if err != nil {
		fmt.Printf("Warning: could not check for updates: %v\n", err)
		latestVersion = ""
	}

	latestVersion = strings.TrimPrefix(latestVersion, "v")

	osName := strings.ToLower(runtime.GOOS)
	arch := strings.ToLower(runtime.GOARCH)
	if arch == "x86_64" {
		arch = "amd64"
	}
	if arch == "aarch64" || arch == "arm64" {
		arch = "arm64"
	}

	ext := ".tar.gz"
	if osName == "windows" {
		ext = ".zip"
	}
	archiveName := fmt.Sprintf("walkline_%s_%s_%s%s", latestVersion, osName, arch, ext)
	binaryName := "walkline"
	if osName == "windows" {
		binaryName = "walkline.exe"
	}

	tmpdir, err := os.MkdirTemp("", "walkline-update")
	if err != nil {
		return fmt.Errorf("could not create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpdir)

	downloadURL := fmt.Sprintf("https://github.com/Pantho-Haque/walkline/releases/download/v%s/%s", latestVersion, archiveName)
	fmt.Printf("Downloading %s...\n", downloadURL)
	c := exec.Command("curl", "-fsSL", "-o", filepath.Join(tmpdir, archiveName), downloadURL)
	if err := c.Run(); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	if ext == ".tar.gz" {
		c = exec.Command("tar", "xzf", archiveName, "-C", tmpdir)
	} else {
		c = exec.Command("unzip", "-o", archiveName, "-d", tmpdir)
	}
	c.Dir = tmpdir
	if err := c.Run(); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	newBinary := filepath.Join(tmpdir, binaryName)
	src, err := os.Open(newBinary)
	if err != nil {
		return fmt.Errorf("could not open new binary: %w", err)
	}
	defer src.Close()
	dst, err := os.OpenFile(currentPath, os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("could not open target binary: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	fmt.Printf("Updated walkline to v%s\n", latestVersion)
	return nil
}

func fetchLatestVersion() (string, error) {
	c := exec.Command("curl", "-fsSL", "-m", "10", "https://api.github.com/repos/Pantho-Haque/walkline/releases/latest")
	out, err := c.Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, `"tag_name"`) {
			parts := strings.Split(line, `"`)
			if len(parts) >= 4 {
				return parts[3], nil
			}
		}
	}
	return "", fmt.Errorf("could not parse version from response")
}

func runAutoSync() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not find home directory: %w", err)
	}

	scanner := hooks.NewScanner(home, 2)
	results, err := scanner.Scan()
	if err != nil {
		fmt.Printf("Auto-sync scan warning: %v\n", err)
	}

	synced := 0
	failed := 0

	for _, repoPath := range results.Repos {
		if err := syncRepo(repoPath); err != nil {
			failed++
			continue
		}
		synced++
	}

	fmt.Printf("Auto-sync complete: %d repos synced, %d failed (no upstream or permission)\n", synced, failed)
	return nil
}

func syncRepo(repoPath string) error {
	s, err := store.New()
	if err != nil {
		return err
	}
	defer s.Close()

	upstream, err := runGit(repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return err
	}
	upstream = strings.TrimSpace(upstream)

	hashes, err := s.GetPendingHashes(repoPath)
	if err != nil {
		return err
	}

	var toMark []string
	for _, h := range hashes {
		isAncestor, err := gitIsAncestor(repoPath, h, upstream)
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

func CheckForUpdate() {
	if Version == "dev" {
		return
	}

	latest, shouldFetch, err := getCachedLatestVersion()
	if err != nil {
		return
	}
	if !shouldFetch {
		return
	}

	latest = strings.TrimPrefix(latest, "v")
	if latest == "" || latest == Version {
		return
	}

	fmt.Printf("\n" +
		"╔══════════════════════════════════════════════════════════════╗\n" +
		"║  A new version '%s' is available.                          ║\n"+
		"║  You have '%s'.                                              ║\n"+
		"║  Run 'walkline update' to upgrade.                          ║\n"+
		"╚══════════════════════════════════════════════════════════════╝\n",
		latest, Version)
}

func getCachedLatestVersion() (string, bool, error) {
	configDir := filepath.Join(osUserHomeDir(), ".walkline")
	lastCheckFile := filepath.Join(configDir, ".last_update_check")
	versionFile := filepath.Join(configDir, ".latest_version")

	var lastCheck time.Time
	if data, err := os.ReadFile(lastCheckFile); err == nil {
		lastCheck, _ = time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	}

	if time.Since(lastCheck) < 24*time.Hour {
		if data, err := os.ReadFile(versionFile); err == nil {
			return strings.TrimSpace(string(data)), false, nil
		}
	}

	latest, err := fetchLatestVersion()
	if err != nil {
		return "", false, err
	}

	os.MkdirAll(configDir, 0755)
	os.WriteFile(lastCheckFile, []byte(time.Now().Format(time.RFC3339)), 0644)
	os.WriteFile(versionFile, []byte(latest), 0644)

	return latest, true, nil
}

func osUserHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}
