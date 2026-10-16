package main

import (
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"walkline/internal/cli"
	"walkline/internal/constants"
)

func main() {
	root := &cobra.Command{Use: "walkline"}
	root.SetVersionTemplate("{{.Version}}\n")
	root.Version = constants.Version
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if !isatty.IsTerminal(os.Stdout.Fd()) {
			return cmd.Help()
		}
		return cli.RunTUI()
	}
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Hooks run on every commit/push and must stay fast and silent; the TUI shows its own banner.
		switch cmd.Name() {
		case "log-commit", "mark-pushed", "walkline", "tui", "update", "completion", "__complete":
		default:
			cli.CheckForUpdate()
		}
		return nil
	}
	root.AddCommand(cli.TUICmd())
	root.AddCommand(cli.LogCommitCmd())
	root.AddCommand(cli.MarkPushedCmd())
	root.AddCommand(cli.ReportCmd())
	root.AddCommand(cli.ExportCmd())
	root.AddCommand(cli.InstallCmd())
	root.AddCommand(cli.ScanCmd())
	root.AddCommand(cli.UninstallCmd())
	root.AddCommand(cli.UpdateCmd())
	root.AddCommand(cli.SyncCmd())
	root.AddCommand(cli.VersionCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
