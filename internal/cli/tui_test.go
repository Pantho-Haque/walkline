package cli

import (
	"strings"
	"testing"
)

func TestRunTUI_RequiresTerminal(t *testing.T) {
	isolate(t)
	// under `go test` stdout is a pipe, so this must fail fast instead of grabbing the tty
	err := RunTUI()
	if err == nil || !strings.Contains(err.Error(), "terminal") {
		t.Fatalf("want terminal error, got %v", err)
	}
	if _, err := run(t, TUICmd()); err == nil {
		t.Error("tui command should error without a terminal")
	}
	if _, err := run(t, TUICmd(), "extra"); err == nil {
		t.Error("tui takes no args")
	}
}
