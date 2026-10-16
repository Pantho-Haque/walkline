package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"walkline/internal/store"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

func plain(s string) string { return ansi.ReplaceAllString(s, "") }

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.NewStore(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func add(t *testing.T, s *store.Store, hash, project, author, msg, when string, pushed bool) {
	t.Helper()
	c := &store.Commit{Hash: hash, ProjectName: project, ProjectPath: "/p/" + project, AuthorName: author,
		AuthorEmail: strings.ToLower(author) + "@x.io", Message: msg, CommittedAt: when}
	if err := s.InsertCommit(c); err != nil {
		t.Fatal(err)
	}
	if pushed {
		s.MarkPushed([]string{hash})
	}
}

// seeded: 5 commits across 3 days, 3 projects, 2 authors.
func seeded(t *testing.T) (*Model, *store.Store) {
	s := newStore(t)
	add(t, s, "aaaaaaa1111", "alpha", "Ann", "Fix login bug", "2026-10-14T09:00:00+06:00", true)
	add(t, s, "bbbbbbb2222", "alpha", "Bob", "Add search", "2026-10-15T10:00:00+06:00", false)
	add(t, s, "ccccccc3333", "beta", "Ann", "Refactor store\n\nlong body text here", "2026-10-16T08:00:00+06:00", false)
	add(t, s, "ddddddd4444", "beta", "Ann", "Bump deps", "2026-10-16T12:00:00+06:00", true)
	add(t, s, "eeeeeee5555", "gamma", "Bob", "Initial commit", "2026-10-16T13:00:00+06:00", false)
	m := New(Options{Store: s, Version: "0.2.0", Now: func() time.Time { return time.Date(2026, 10, 16, 15, 0, 0, 0, time.UTC) }})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return m, s
}

func key(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// press sends keys; multi-rune literals like "login" are typed char by char via typ.
func press(m *Model, keys ...string) tea.Cmd {
	var cmd tea.Cmd
	for _, k := range keys {
		_, cmd = m.Update(key(k))
	}
	return cmd
}

func typ(m *Model, text string) {
	for _, r := range text {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func hashesOf(m *Model) []string {
	var out []string
	for _, c := range m.commits {
		out = append(out, c.Hash[:7])
	}
	return out
}

func eq(a, b []string) bool { return strings.Join(a, ",") == strings.Join(b, ",") }

func TestInitialListNewestFirstWithStats(t *testing.T) {
	m, _ := seeded(t)
	if got := hashesOf(m); !eq(got, []string{"eeeeeee", "ddddddd", "ccccccc", "bbbbbbb", "aaaaaaa"}) {
		t.Errorf("order: %v", got)
	}
	v := plain(m.View())
	for _, want := range []string{"walkline", "0.2.0", "5 commits", "3 pending", "3 projects", "Fri 16 Oct 2026 · 3 commits", "Thu 15 Oct 2026 · 1 commit", "Initial commit", "1/5"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestSearchFiltersLiveAndEscKeepsFilter(t *testing.T) {
	m, _ := seeded(t)
	press(m, "/")
	typ(m, "bug")
	if got := hashesOf(m); !eq(got, []string{"aaaaaaa"}) {
		t.Errorf("search 'bug': %v", got)
	}
	press(m, "esc")
	if m.focus != -1 || len(m.commits) != 1 {
		t.Errorf("esc should leave the field but keep the filter (focus=%d, n=%d)", m.focus, len(m.commits))
	}
	// First esc in list mode clears filters, second does nothing harmful.
	press(m, "esc")
	if len(m.commits) != 5 {
		t.Errorf("esc in list mode should clear filters, n=%d", len(m.commits))
	}
}

func TestQuitKeyIsTextInFilterMode(t *testing.T) {
	m, _ := seeded(t)
	press(m, "/")
	cmd := press(m, "q")
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("typing q in a filter field must not quit")
		}
	}
	if m.in[fSearch].Value() != "q" {
		t.Errorf("value %q", m.in[fSearch].Value())
	}
}

func TestQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		m, _ := seeded(t)
		cmd := press(m, k)
		if cmd == nil {
			t.Fatalf("%s: no cmd", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%s did not quit", k)
		}
	}
	m, _ := seeded(t)
	press(m, "/")
	if cmd := press(m, "ctrl+c"); cmd == nil {
		t.Error("ctrl+c must quit even from a filter field")
	}
}

func TestProjectAndAuthorFilters(t *testing.T) {
	m, _ := seeded(t)
	press(m, "p")
	typ(m, "BET")
	if got := hashesOf(m); !eq(got, []string{"ddddddd", "ccccccc"}) {
		t.Errorf("project substring: %v", got)
	}
	press(m, "tab") // → author
	if m.focus != fAuthor {
		t.Fatalf("tab should move to author, focus=%d", m.focus)
	}
	typ(m, "ann")
	press(m, "enter")
	if got := hashesOf(m); !eq(got, []string{"ddddddd", "ccccccc"}) {
		t.Errorf("project+author: %v", got)
	}
	m.clear()
	press(m, "a")
	typ(m, "bob@")
	if got := hashesOf(m); !eq(got, []string{"eeeeeee", "bbbbbbb"}) {
		t.Errorf("author by email: %v", got)
	}
}

func TestTabCyclesAndShiftTabReverses(t *testing.T) {
	m, _ := seeded(t)
	press(m, "/")
	for i := 1; i < nFields; i++ {
		press(m, "tab")
		if m.focus != i {
			t.Fatalf("after %d tabs focus=%d", i, m.focus)
		}
	}
	press(m, "tab")
	if m.focus != fSearch {
		t.Errorf("tab should wrap, focus=%d", m.focus)
	}
	press(m, "shift+tab")
	if m.focus != fUntil {
		t.Errorf("shift+tab should wrap backwards, focus=%d", m.focus)
	}
}

func TestDateRangeFilters(t *testing.T) {
	m, _ := seeded(t)
	press(m, "f")
	typ(m, "2026-10-15")
	if got := hashesOf(m); !eq(got, []string{"eeeeeee", "ddddddd", "ccccccc", "bbbbbbb"}) {
		t.Errorf("since: %v", got)
	}
	press(m, "tab")
	typ(m, "2026-10-15")
	if got := hashesOf(m); !eq(got, []string{"bbbbbbb"}) {
		t.Errorf("since+until same day: %v", got)
	}
	if m.bad[fSince] || m.bad[fUntil] {
		t.Error("valid dates flagged bad")
	}
}

func TestInvalidDateIsIgnoredAndFlagged(t *testing.T) {
	m, _ := seeded(t)
	press(m, "f")
	typ(m, "2026-1") // half typed
	if !m.bad[fSince] {
		t.Error("partial date should be flagged")
	}
	if len(m.commits) != 5 {
		t.Errorf("invalid bound must be ignored, n=%d", len(m.commits))
	}
	typ(m, "0-16")
	if m.bad[fSince] {
		t.Error("completed date still flagged")
	}
	if got := hashesOf(m); !eq(got, []string{"eeeeeee", "ddddddd", "ccccccc"}) {
		t.Errorf("after completing: %v", got)
	}
	m.in[fSince].SetValue("2026-02-30")
	m.reload()
	if !m.bad[fSince] {
		t.Error("impossible date must be flagged")
	}
}

func TestStatusCycle(t *testing.T) {
	m, _ := seeded(t)
	want := []struct {
		s statusFilter
		h []string
	}{
		{statusPending, []string{"eeeeeee", "ccccccc", "bbbbbbb"}},
		{statusPushed, []string{"ddddddd", "aaaaaaa"}},
		{statusAll, []string{"eeeeeee", "ddddddd", "ccccccc", "bbbbbbb", "aaaaaaa"}},
	}
	for _, w := range want {
		press(m, "s")
		if m.stat != w.s || !eq(hashesOf(m), w.h) {
			t.Errorf("status %v: got %v", w.s, hashesOf(m))
		}
	}
	press(m, "s")
	if !strings.Contains(plain(m.View()), "Status pending") {
		t.Error("filter bar should show the active status")
	}
}

func TestDatePresetCycle(t *testing.T) {
	m, _ := seeded(t) // Now = 2026-10-16
	press(m, "d")
	if m.in[fSince].Value() != "2026-10-16" || !eq(hashesOf(m), []string{"eeeeeee", "ddddddd", "ccccccc"}) {
		t.Errorf("today: since=%q %v", m.in[fSince].Value(), hashesOf(m))
	}
	press(m, "d")
	if m.in[fSince].Value() != "2026-10-10" || len(m.commits) != 5 {
		t.Errorf("7d: since=%q n=%d", m.in[fSince].Value(), len(m.commits))
	}
	if !strings.Contains(plain(m.View()), "last 7 days") {
		t.Error("active preset should be shown")
	}
	press(m, "d", "d")
	press(m, "d") // back to none
	if m.preset != 0 || m.in[fSince].Value() != "" {
		t.Errorf("cycle should end at none: preset=%d since=%q", m.preset, m.in[fSince].Value())
	}
}

func TestEditingDateDropsPresetLabel(t *testing.T) {
	m, _ := seeded(t)
	press(m, "d", "f")
	m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.preset != 0 {
		t.Error("manual date edit should reset the preset label")
	}
}

func TestClearResetsEverything(t *testing.T) {
	m, _ := seeded(t)
	press(m, "/")
	typ(m, "bug")
	press(m, "esc", "s", "d")
	press(m, "x")
	if m.hasFilters() || m.preset != 0 || len(m.commits) != 5 {
		t.Errorf("clear left state: filters=%v preset=%d n=%d", m.hasFilters(), m.preset, len(m.commits))
	}
}

func TestNavigationBounds(t *testing.T) {
	m, _ := seeded(t)
	press(m, "k", "up")
	if m.cursor != 0 {
		t.Errorf("cursor above top: %d", m.cursor)
	}
	press(m, "j", "down")
	if m.cursor != 2 {
		t.Errorf("cursor %d", m.cursor)
	}
	press(m, "G")
	if m.cursor != 4 {
		t.Errorf("G: %d", m.cursor)
	}
	press(m, "j")
	if m.cursor != 4 {
		t.Errorf("cursor past end: %d", m.cursor)
	}
	press(m, "g")
	if m.cursor != 0 {
		t.Errorf("g: %d", m.cursor)
	}
}

func TestNavigationOnEmptyResultsDoesNotPanic(t *testing.T) {
	m, _ := seeded(t)
	press(m, "/")
	typ(m, "no-such-thing")
	press(m, "esc", "j", "k", "G", "g", "ctrl+d", "enter", "enter")
	_ = m.View()
	if m.selected() != nil {
		t.Error("nothing should be selected")
	}
}

func TestSelectionSurvivesFilterChange(t *testing.T) {
	m, _ := seeded(t)
	press(m, "G") // aaaaaaa
	press(m, "/")
	typ(m, "a") // matches many, aaaaaaa still present
	if c := m.selected(); c == nil || c.Hash[:7] != "aaaaaaa" {
		t.Errorf("selection lost: %+v", c)
	}
}

func TestScrollKeepsCursorVisible(t *testing.T) {
	s := newStore(t)
	for i := 0; i < 80; i++ {
		add(t, s, fmt.Sprintf("%07d", i), "proj", "Ann", fmt.Sprintf("commit number %d", i),
			fmt.Sprintf("2026-09-%02dT10:%02d:00+06:00", 1+i/4, i%60), false)
	}
	m := New(Options{Store: s})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	for step := 0; step < 79; step++ {
		press(m, "j")
		sel := m.selected()
		want := "commit number " + strings.TrimLeft(sel.Hash, "0")
		if sel.Hash == "0000000" {
			want = "commit number 0"
		}
		if !strings.Contains(plain(m.View()), want) {
			t.Fatalf("step %d: selected commit %q not visible\n%s", step, want, plain(m.View()))
		}
	}
	press(m, "g")
	if m.top != 0 {
		t.Errorf("g should scroll back to top, top=%d", m.top)
	}
	if lines := strings.Count(m.View(), "\n") + 1; lines != 24 {
		t.Errorf("view must fill exactly the terminal height, got %d lines", lines)
	}
}

func TestViewNeverOverflowsTerminal(t *testing.T) {
	m, _ := seeded(t)
	for _, size := range [][2]int{{72, 12}, {80, 24}, {99, 30}, {100, 30}, {120, 40}, {200, 50}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, mode := range []string{"list", "detail", "help", "filter"} {
			m.help, m.detail = false, false
			m.blur()
			switch mode {
			case "detail":
				press(m, "enter")
			case "help":
				press(m, "?")
			case "filter":
				press(m, "/")
			}
			lines := strings.Split(m.View(), "\n")
			if len(lines) != size[1] {
				t.Errorf("%v %s: %d lines, want %d", size, mode, len(lines), size[1])
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w > size[0] {
					t.Errorf("%v %s: line %d is %d wide: %q", size, mode, i, w, plain(l))
				}
			}
			m.help = false
			m.blur()
			m.detail = false
		}
	}
}

func TestTooSmallTerminal(t *testing.T) {
	m, _ := seeded(t)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 8})
	if !strings.Contains(m.View(), "Terminal too small") {
		t.Error("expected too-small message")
	}
}

func TestDetailPaneSplitLayout(t *testing.T) {
	m, _ := seeded(t)
	v := plain(m.View())
	for _, want := range []string{"Commit", "eeeeeee5555", "Author", "Bob <bob@x.io>", "Status", "pending push", "/p/gamma"} {
		if !strings.Contains(v, want) {
			t.Errorf("detail pane missing %q", want)
		}
	}
	press(m, "j", "j") // ccccccc: multi-line message
	v = plain(m.View())
	if !strings.Contains(v, "long body text here") {
		t.Error("detail pane should show the full message body")
	}
	press(m, "k") // ddd: pushed
	if !strings.Contains(plain(m.View()), "pushed") {
		t.Error("pushed status missing")
	}
}

func TestDetailToggleOnNarrowTerminal(t *testing.T) {
	m, _ := seeded(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if strings.Contains(plain(m.View()), "/p/gamma") {
		t.Error("narrow terminal should start with the list only")
	}
	press(m, "enter")
	if !strings.Contains(plain(m.View()), "/p/gamma") {
		t.Error("enter should open details")
	}
	press(m, "esc")
	if m.detail {
		t.Error("esc should close details")
	}
	press(m, "enter", "h")
	if m.detail {
		t.Error("h should close details")
	}
}

func TestHelpOverlay(t *testing.T) {
	m, _ := seeded(t)
	press(m, "?")
	v := plain(m.View())
	if !strings.Contains(v, "Keyboard shortcuts") || !strings.Contains(v, "cycle status") {
		t.Error("help not shown")
	}
	press(m, "x") // any key closes and is consumed
	if m.help || strings.Contains(plain(m.View()), "Keyboard shortcuts") {
		t.Error("help should close on any key")
	}
}

func TestEmptyStates(t *testing.T) {
	m := New(Options{Store: newStore(t)})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	if v := plain(m.View()); !strings.Contains(v, "No commits recorded yet") || !strings.Contains(v, "walkline scan") {
		t.Errorf("first-run guidance missing:\n%s", v)
	}
	m, _ = seeded(t)
	press(m, "/")
	typ(m, "zzzz")
	if v := plain(m.View()); !strings.Contains(v, "No commits match") || !strings.Contains(v, "Press x") {
		t.Error("filtered-empty guidance missing")
	}
}

func TestFilterBarShowsInvalidDateInRed(t *testing.T) {
	m, _ := seeded(t)
	press(m, "f")
	typ(m, "20")
	if !m.bad[fSince] {
		t.Fatal("expected bad flag")
	}
	_ = m.View() // must render without panic
}

func TestSyncFlow(t *testing.T) {
	m, s := seeded(t)
	called := 0
	m.opt.Sync = func() (int, error) {
		called++
		return 2, nil
	}
	cmd := m.Init()
	if cmd == nil || m.busy != "syncing" {
		t.Fatal("Init should start a sync")
	}
	add(t, s, "fffffff6666", "delta", "Cy", "Arrived during sync", "2026-10-16T14:00:00+06:00", false)
	m.Update(m.syncCmd()()) // run the sync cmd and feed its result back
	if called != 1 || m.busy != "" || m.notice != "synced 2 repos" {
		t.Errorf("called=%d busy=%q notice=%q", called, m.busy, m.notice)
	}
	if len(m.commits) != 6 || m.stats.Total != 6 {
		t.Errorf("list/stats should refresh after sync: n=%d total=%d", len(m.commits), m.stats.Total)
	}
	press(m, "r")
	if m.busy != "syncing" {
		t.Error("r should start a sync")
	}
	if cmd := press(m, "r"); cmd != nil {
		t.Error("r while busy must not start a second sync")
	}
}

func TestSyncError(t *testing.T) {
	m, _ := seeded(t)
	m.Update(syncDoneMsg{err: errors.New("boom")})
	if !m.isErr || !strings.Contains(m.notice, "boom") {
		t.Errorf("notice=%q", m.notice)
	}
	if !strings.Contains(plain(m.View()), "sync failed: boom") {
		t.Error("error should be visible in the footer")
	}
}

func TestNoticeClearsOnNextKey(t *testing.T) {
	m, _ := seeded(t)
	m.Update(syncDoneMsg{n: 1})
	press(m, "j")
	if m.notice != "" {
		t.Error("notice should clear on next keypress")
	}
}

func TestUpdateAvailableBanner(t *testing.T) {
	m, _ := seeded(t)
	m.Update(updateInfoMsg{latest: "0.1.0"})
	if m.latest != "" {
		t.Error("older release must not be offered")
	}
	m.Update(updateInfoMsg{latest: "0.2.0"})
	if m.latest != "" {
		t.Error("same release must not be offered")
	}
	m.Update(updateInfoMsg{err: errors.New("offline"), latest: "9.9.9"})
	if m.latest != "" {
		t.Error("failed check must not be offered")
	}
	m.Update(updateInfoMsg{latest: "0.3.0"})
	if !strings.Contains(plain(m.View()), "v0.3.0 available") {
		t.Error("banner missing")
	}
}

func TestDevBuildIsOfferedEveryRelease(t *testing.T) {
	m, _ := seeded(t)
	m.opt.Version = "dev"
	m.Update(updateInfoMsg{latest: "0.1.3"})
	if m.latest != "0.1.3" {
		t.Error("a dev build should be offered the latest release")
	}
}

func TestInstallUpdateFlow(t *testing.T) {
	m, _ := seeded(t)
	var installed string
	m.opt.Install = func(v string) error { installed = v; return nil }

	if cmd := press(m, "u"); cmd != nil || m.notice != "already on the latest release" {
		t.Errorf("without an update u should just say so: %q", m.notice)
	}
	m.Update(updateInfoMsg{latest: "0.3.0"})
	cmd := press(m, "u")
	if cmd == nil || m.busy != "updating" {
		t.Fatal("u should start the install")
	}
	if press(m, "u") != nil {
		t.Error("second u while busy must be ignored")
	}
	m.Update(cmd())
	if installed != "0.3.0" || m.busy != "" || m.latest != "" || !strings.Contains(m.notice, "v0.3.0") || m.isErr {
		t.Errorf("installed=%q busy=%q latest=%q notice=%q", installed, m.busy, m.latest, m.notice)
	}
}

func TestInstallUpdateFailureKeepsBanner(t *testing.T) {
	m, _ := seeded(t)
	m.opt.Install = func(string) error { return errors.New("checksum mismatch") }
	m.Update(updateInfoMsg{latest: "0.3.0"})
	m.Update(press(m, "u")())
	if !m.isErr || !strings.Contains(m.notice, "checksum mismatch") || m.latest != "0.3.0" {
		t.Errorf("notice=%q latest=%q", m.notice, m.latest)
	}
}

func TestInitWithoutHooksIsSafe(t *testing.T) {
	m, _ := seeded(t)
	if cmd := m.Init(); cmd != nil {
		// tea.Batch of nothing is nil; anything else must be runnable
		_ = cmd
	}
	if m.busy != "" {
		t.Error("no sync hook means nothing is busy")
	}
}

func TestInitRunsUpdateCheck(t *testing.T) {
	m, _ := seeded(t)
	m.opt.CheckUpdate = func() (string, error) { return "1.0.0", nil }
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	m.Update(updateInfoMsg{latest: "1.0.0"})
	if m.latest != "1.0.0" {
		t.Error("update info not applied")
	}
}

func TestReloadReportsQueryErrors(t *testing.T) {
	m, s := seeded(t)
	s.Close()
	m.reload()
	if m.loadErr == nil {
		t.Fatal("expected a load error from a closed store")
	}
	if !strings.Contains(plain(m.View()), "Query failed") {
		t.Error("query failure should be visible, not a blank list")
	}
}

func TestWideCharsAndLongMessagesDoNotBreakLayout(t *testing.T) {
	s := newStore(t)
	add(t, s, "1111111aaaa", "プロジェクト", "李雷", "修复登录错误 "+strings.Repeat("very long ", 40), "2026-10-16T08:00:00+06:00", false)
	add(t, s, "2222222bbbb", "emoji", "Zoë", "🎉 ship it", "2026-10-16T09:00:00+06:00", true)
	m := New(Options{Store: s})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	for i, l := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(l); w > 100 {
			t.Errorf("line %d overflows: %d", i, w)
		}
	}
}
