package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"walkline/internal/constants"
	"walkline/internal/hooks"
	"walkline/internal/sync"
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

			installCompletion()

			fmt.Println("NOTE: This does NOT affect existing repos. Run 'walkline scan <root>' to")
			fmt.Println("      instrument existing repos (see 'walkline scan --help' for details).")
			fmt.Println("\nRunning auto-sync on existing repos...")
			return sync.AutoSync(constants.DataDir())
		},
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
				if err := os.RemoveAll(constants.DataDir()); err == nil {
					removed = append(removed, constants.DataDir())
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
