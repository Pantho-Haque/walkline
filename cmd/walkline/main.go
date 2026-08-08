package main

import (
	"os"

	"github.com/spf13/cobra"
	"walkline/internal/cli"
)

func main() {
	root := &cobra.Command{Use: "walkline"}
	root.SetVersionTemplate("{{.Version}}\n")
	root.Version = cli.Version
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cli.CheckForUpdate()
		return nil
	}
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
