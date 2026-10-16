package tui

import "github.com/charmbracelet/lipgloss"

// Palette uses fixed ANSI-256 indexes (no AdaptiveColor) so it reads the same on light and dark
// terminals and never needs to query the terminal for its background colour.
var (
	cAccent  = lipgloss.Color("105") // violet
	cText    = lipgloss.NoColor{}    // terminal default: readable on any background
	cDim     = lipgloss.Color("244") // mid grey works on light and dark
	cPushed  = lipgloss.Color("78")  // green
	cPending = lipgloss.Color("214") // amber
	cError   = lipgloss.Color("203") // red
	cSelBg   = lipgloss.Color("60")

	sDim    = lipgloss.NewStyle().Foreground(cDim)
	sAccent = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	sError  = lipgloss.NewStyle().Foreground(cError)
	sTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).Background(cAccent).Padding(0, 1)

	sPane      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cDim)
	sPaneFocus = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cAccent)
)

func fg(c lipgloss.TerminalColor, sel bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(c)
	if sel {
		s = s.Background(cSelBg).Bold(true)
		switch c {
		case cText:
			s = s.Foreground(lipgloss.Color("255"))
		case cDim:
			s = s.Foreground(lipgloss.Color("252"))
		}
	}
	return s
}
