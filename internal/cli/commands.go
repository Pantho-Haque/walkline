package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"walkline/internal/hooks"
	"walkline/internal/shellwrap"
)

func InstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "install",
		Short:   "Install global git template hooks for future repos",
		Example: "walkline install",
		RunE: func(cmd *cobra.Command, args []string) error {
			templateDir, err := hooks.SetupTemplateDir()
			if err != nil {
				return fmt.Errorf("setup template: %w", err)
			}
			fmt.Printf("Global git template installed at: %s\n", templateDir)
			fmt.Println("All NEW repos created after this will automatically have the walkline hook.")
			fmt.Println()
			fmt.Println("NOTE: This does NOT affect existing repos. Run 'walkline scan <root>' to")
			fmt.Println("      instrument existing repos (see 'walkline scan --help' for details).")
			fmt.Println()

			wrapper, err := shellwrap.Generate()
			if err != nil {
				return fmt.Errorf("generate wrapper: %w", err)
			}
			fmt.Println("=== Git push wrapper ===")
			fmt.Println(wrapper)
			fmt.Println("Append this to your shell rc file to enable push tracking.")
			fmt.Println()

			rcFiles := shellwrap.DetectShellRC()
			if len(rcFiles) == 0 {
				fmt.Println("Could not detect shell rc files. Please add the wrapper manually.")
			} else {
				fmt.Printf("Detected shell rc files: %v\n", rcFiles)
				fmt.Println("Run 'shellwrap' subcommand to get installation help.")
			}
			return nil
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

func ShellwrapCmd() *cobra.Command {
	var install bool

	cmd := &cobra.Command{
		Use:     "shellwrap",
		Short:   "Show git push wrapper and offer to install",
		Example: "walkline shellwrap --install",
		RunE: func(cmd *cobra.Command, args []string) error {
			wrapper, err := shellwrap.Generate()
			if err != nil {
				return err
			}
			fmt.Println(wrapper)
			if install {
				return shellwrap.Install(wrapper)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&install, "install", false, "Install wrapper to shell rc")
	return cmd
}
