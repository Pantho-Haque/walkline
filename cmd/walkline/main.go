package main

import (
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
	root.AddCommand(cli.ShellwrapCmd())
	root.Execute()
}
