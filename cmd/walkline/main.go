package main

import (
	"os"

	"github.com/spf13/cobra"
	"walkline/internal/cli"
)

func main() {
	root := &cobra.Command{Use: "walkline"}
	root.AddCommand(cli.LogCommitCmd())
	root.AddCommand(cli.MarkPushedCmd())
	root.AddCommand(cli.ReportCmd())
	root.AddCommand(cli.ExportCmd())
	root.AddCommand(cli.InstallCmd())
	root.AddCommand(cli.ScanCmd())
	root.AddCommand(cli.UninstallCmd())
	root.AddCommand(cli.UpdateCmd())
	root.AddCommand(cli.SyncCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
