package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"walkline/internal/constants"
	"walkline/internal/store"
	"walkline/internal/sync"
	"walkline/internal/tui"
	"walkline/internal/update"
)

// RunTUI opens the interactive commit browser.
func RunTUI() error {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("the interactive browser needs a terminal; use 'walkline report' for plain output")
	}
	s, err := store.New()
	if err != nil {
		return err
	}
	defer s.Close()

	m := tui.New(tui.Options{
		Store:   s,
		Version: constants.Version,
		Sync: func() (int, error) {
			n, _, err := sync.SyncPending(s)
			return n, err
		},
		CheckUpdate: func() (string, error) { return update.Cached(constants.DataDir()) },
		Install:     InstallRelease,
	})
	// A plain termenv.Output stops bubbletea from querying the terminal for its colours on start-up,
	// which stalls for seconds on terminals that never answer (our palette is fixed anyway).
	_, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(termenv.NewOutput(os.Stdout))).Run()
	return err
}

func TUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Browse, search and filter your commits interactively",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return RunTUI() },
	}
}
