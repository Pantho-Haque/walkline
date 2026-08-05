package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"walkline/internal/hooks"
)

func InstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "install",
		Short:   "Install global git template hooks",
		Example: "walkline install",
		RunE: func(cmd *cobra.Command, args []string) error {
			templateDir, err := hooks.SetupTemplateDir()
			if err != nil {
				return fmt.Errorf("setup template: %w", err)
			}
			fmt.Printf("Global git template installed at: %s\n", templateDir)
			fmt.Println("All NEW repos created after this will automatically have walkline's post-commit and pre-push hooks.")
			fmt.Println()

			// Install shell completion
			installCompletion()

			fmt.Println("NOTE: This does NOT affect existing repos. Run 'walkline scan <root>' to")
			fmt.Println("      instrument existing repos (see 'walkline scan --help' for details).")
			return nil
		},
	}
}

func installCompletion() {
	home, _ := os.UserHomeDir()

	// Detect shell
	sh := filepath.Base(os.Getenv("SHELL"))
	if sh == "" {
		fmt.Println("Shell completion: could not detect shell")
		return
	}

	switch sh {
	case "zsh":
		compDir := filepath.Join(home, ".zsh", "completions")
		os.MkdirAll(compDir, 0755)
		c := exec.Command("walkline", "completion", "zsh")
		out, err := c.Output()
		if err != nil {
			fmt.Println("Shell completion: failed to generate zsh completion")
			return
		}
		err = os.WriteFile(filepath.Join(compDir, "_walkline"), out, 0644)
		if err != nil {
			fmt.Println("Shell completion: failed to install zsh completion")
			return
		}

		// Add fpath to .zshrc if not already present
		zshrc := filepath.Join(home, ".zshrc")
		fpathLine := `fpath=($HOME/.zsh/completions $fpath)`
		content, _ := os.ReadFile(zshrc)
		if !strings.Contains(string(content), fpathLine) {
			f, _ := os.OpenFile(zshrc, os.O_WRONLY|os.O_APPEND, 0644)
			f.WriteString("\n" + fpathLine + "\n")
			f.Close()
		}

		fmt.Println("Zsh completion installed to ~/.zsh/completions/_walkline")
		fmt.Println("Restart shell or run: source ~/.zshrc")

	case "bash":
		compFile := filepath.Join(home, ".bash_completion.d", "walkline")
		os.MkdirAll(filepath.Dir(compFile), 0755)
		c := exec.Command("walkline", "completion", "bash")
		out, err := c.Output()
		if err != nil {
			fmt.Println("Shell completion: failed to generate bash completion")
			return
		}
		err = os.WriteFile(compFile, out, 0644)
		if err != nil {
			fmt.Println("Shell completion: failed to install bash completion")
			return
		}
		fmt.Println("Bash completion installed to ~/.bash_completion.d/walkline")

	case "fish":
		compDir := filepath.Join(home, ".config", "fish", "completions")
		os.MkdirAll(compDir, 0755)
		c := exec.Command("walkline", "completion", "fish")
		out, err := c.Output()
		if err != nil {
			fmt.Println("Shell completion: failed to generate fish completion")
			return
		}
		err = os.WriteFile(filepath.Join(compDir, "walkline.fish"), out, 0644)
		if err != nil {
			fmt.Println("Shell completion: failed to install fish completion")
			return
		}
		fmt.Println("Fish completion installed to ~/.config/fish/completions/walkline.fish")

	case "powershell", "pwsh":
		compDir := filepath.Join(home, "Documents", "PowerShell", "Completions")
		os.MkdirAll(compDir, 0755)
		c := exec.Command("walkline", "completion", "powershell")
		out, err := c.Output()
		if err != nil {
			fmt.Println("Shell completion: failed to generate powershell completion")
			return
		}
		err = os.WriteFile(filepath.Join(compDir, "walkline.ps1"), out, 0644)
		if err != nil {
			fmt.Println("Shell completion: failed to install powershell completion")
			return
		}
		fmt.Println("PowerShell completion installed to ~/Documents/PowerShell/Completions/walkline.ps1")

	default:
		fmt.Printf("Shell completion: unsupported shell '%s'. Supported: zsh, bash, fish, powershell\n", sh)
		fmt.Println("Run 'walkline completion <shell>' to generate the script manually.")
	}
}

func ScanCmd() *cobra.Command {
	var depth int

	cmd := &cobra.Command{
		Use:     "scan <root-dir>",
		Short:   "Scan directories for git repos and install hooks",
		Example: "walkline scan ~/projects --depth=1",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir := args[0]
			scanner := hooks.NewScanner(rootDir, depth)
			results, err := scanner.Scan()
			if err != nil {
				return err
			}

			fmt.Printf("\nScan complete:\n")
			fmt.Printf("  %d repos scanned\n", results.Total)
			fmt.Printf("  %d hooks freshly installed\n", results.Fresh)
			fmt.Printf("  %d merged with existing custom hook\n", results.Merged)
			if len(results.MergedPaths) > 0 {
				fmt.Println("  Merged paths (verify manually):")
				for _, p := range results.MergedPaths {
					fmt.Printf("    %s\n", p)
				}
			}
			fmt.Printf("  %d already had walkline (no-op)\n", results.NoOp)
			return nil
		},
	}
	cmd.Flags().IntVar(&depth, "depth", 1, "Directory depth to scan")
	return cmd
}

func UninstallCmd() *cobra.Command {
	var includeDB bool

	cmd := &cobra.Command{
		Use:     "uninstall",
		Short:   "Remove walkline and all installed components",
		Example: "walkline uninstall [--include-db]",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			removed := []string{}

			binPaths := []string{
				filepath.Join(home, ".local", "bin", "walkline"),
				"/usr/local/bin/walkline",
			}
			for _, p := range binPaths {
				if err := os.Remove(p); err == nil {
					removed = append(removed, p)
				}
			}

			templateDir := filepath.Join(home, ".git-templates")
			if err := os.RemoveAll(templateDir); err == nil {
				removed = append(removed, templateDir)
			}

			sh := filepath.Base(os.Getenv("SHELL"))
			switch sh {
			case "zsh":
				os.Remove(filepath.Join(home, ".zsh", "completions", "_walkline"))
				os.Remove(filepath.Join(home, ".zsh", "completions", "_walkline.bak"))
			case "bash":
				os.Remove(filepath.Join(home, ".bash_completion.d", "walkline"))
			case "fish":
				os.Remove(filepath.Join(home, ".config", "fish", "completions", "walkline.fish"))
			case "powershell", "pwsh":
				os.Remove(filepath.Join(home, "Documents", "PowerShell", "Completions", "walkline.ps1"))
			}

			if includeDB {
				dbDir := filepath.Join(home, ".walkline")
				if err := os.RemoveAll(dbDir); err == nil {
					removed = append(removed, dbDir)
				}
			}

			fmt.Println("Removed:")
			for _, r := range removed {
				fmt.Printf("  - %s\n", r)
			}
			if !includeDB {
				fmt.Println("\nNote: Database preserved at ~/.walkline. Use --include-db to remove it.")
			}
			fmt.Println("\nNote: Hooks already installed in individual repos are left in place (they are harmless).")
			return nil
		},
	}
	cmd.Flags().BoolVar(&includeDB, "include-db", false, "Also remove the database at ~/.walkline")
	return cmd
}

func UpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update",
		Short:   "Update walkline to the latest version",
		Example: "walkline update",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Find current binary path
			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("could not find current binary: %w", err)
			}

			fmt.Println("Downloading latest release...")
			version := "latest"
			// Resolve latest version
			c := exec.Command("curl", "-fsSL", "https://api.github.com/repos/Pantho-Haque/walkline/releases/latest")
			out, err := c.Output()
			if err == nil {
				lines := strings.Split(string(out), "\n")
				for _, line := range lines {
					if strings.Contains(line, `"tag_name"`) {
						version = strings.Split(line, `"`)[3]
						break
					}
				}
			}

			// Detect OS and arch
			osName := strings.ToLower(func() string {
				out, _ := exec.Command("uname", "-s").Output()
				s := strings.TrimSpace(string(out))
				if s == "Darwin" {
					return "darwin"
				}
				return s
			}())

			arch := strings.ToLower(func() string {
				out, _ := exec.Command("uname", "-m").Output()
				s := strings.TrimSpace(string(out))
				if s == "x86_64" {
					return "amd64"
				}
				if s == "arm64" || s == "aarch64" {
					return "arm64"
				}
				return s
			}())

			ext := ".tar.gz"
			if osName == "windows" {
				ext = ".zip"
			}
			archiveName := fmt.Sprintf("walkline_%s_%s_%s%s", strings.TrimPrefix(version, "v"), osName, arch, ext)
			binaryName := "walkline"
			if osName == "windows" {
				binaryName = "walkline.exe"
			}

			tmpdir, err := os.MkdirTemp("", "walkline-update")
			if err != nil {
				return fmt.Errorf("could not create temp dir: %w", err)
			}
			defer os.RemoveAll(tmpdir)

			// Download
			downloadURL := fmt.Sprintf("https://github.com/Pantho-Haque/walkline/releases/download/%s/%s", version, archiveName)
			fmt.Printf("Downloading %s...\n", downloadURL)
			c = exec.Command("curl", "-fsSL", "-o", filepath.Join(tmpdir, archiveName), downloadURL)
			if err := c.Run(); err != nil {
				return fmt.Errorf("download failed: %w", err)
			}

			// Extract
			if ext == ".tar.gz" {
				c = exec.Command("tar", "xzf", archiveName, "-C", tmpdir)
			} else {
				c = exec.Command("unzip", "-o", archiveName, "-d", tmpdir)
			}
			c.Dir = tmpdir
			if err := c.Run(); err != nil {
				return fmt.Errorf("extraction failed: %w", err)
			}

			// Replace binary
			newBinary := filepath.Join(tmpdir, binaryName)
			src, err := os.Open(newBinary)
			if err != nil {
				return fmt.Errorf("could not open new binary: %w", err)
			}
			defer src.Close()
			dst, err := os.OpenFile(execPath, os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("could not open target binary: %w", err)
			}
			defer dst.Close()
			if _, err := io.Copy(dst, src); err != nil {
				return fmt.Errorf("failed to replace binary: %w", err)
			}

			fmt.Printf("Updated walkline to %s\n", version)
			return nil
		},
	}
	return cmd
}
