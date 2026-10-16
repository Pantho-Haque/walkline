package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"walkline/internal/constants"
	"walkline/internal/sync"
	"walkline/internal/update"
)

func UpdateCmd() *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:     "update",
		Short:   "Update walkline to the latest release",
		Example: "walkline update [--check]",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Checking for latest release...")
			latest, err := update.Latest()
			if err != nil {
				return fmt.Errorf("could not check for updates: %w", err)
			}
			if !update.Newer(latest, constants.Version) {
				fmt.Printf("Already up to date (%s).\n", constants.Version)
				return nil
			}
			if check {
				fmt.Printf("Update available: %s -> %s\n", constants.Version, latest)
				return nil
			}
			if err := InstallRelease(latest); err != nil {
				return err
			}
			fmt.Printf("Updated walkline %s -> %s\n", constants.Version, latest)
			fmt.Println("\nRunning auto-sync...")
			return sync.AutoSync(constants.DataDir())
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Only report whether an update is available")
	return cmd
}

// InstallRelease replaces the running binary with the given release version.
func InstallRelease(version string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return update.Install(version, exe)
}

// CheckForUpdate prints a one-line hint (to stderr) when a newer release exists.
func CheckForUpdate() {
	latest, _ := update.Cached(constants.DataDir())
	if constants.Version != "dev" && update.Newer(latest, constants.Version) {
		fmt.Fprintf(os.Stderr, "walkline %s is available (you have %s) — run 'walkline update'\n", latest, constants.Version)
	}
}
