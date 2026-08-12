package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"walkline/internal/constants"
)

func VersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(constants.Version)
			return nil
		},
	}
}
