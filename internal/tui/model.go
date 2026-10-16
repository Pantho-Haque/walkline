// Package tui is walkline's interactive commit browser.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"walkline/internal/store"
	"walkline/internal/update"
)

// Options wires the TUI to the outside world; every hook is optional so tests can stub them.
type Options struct {
	Store   *store.Store
	Version string
	Now     func() time.Time
	// Sync reconciles pending commits with their upstreams; returns repos reconciled.
	Sync func() (int, error)
	// CheckUpdate returns the latest release version ("" when unknown).
	CheckUpdate func() (string, error)
	// Install replaces the binary with the given release version.
	Install func(version string) error
}

const (
	fSearch = iota
	fProject
	fAuthor
	fSince
	fUntil
	nFields
)

var fieldLabels = [nFields]string{"Search", "Project", "Author", "Since", "Until"}

type statusFilter int

const (
	statusAll statusFilter = iota
	statusPending
	statusPushed
)

func (s statusFilter) String() string { return [...]string{"all", "pending", "pushed"}[s] }

// date presets cycled with `d`: label and how many days back (0 = today).
var datePresets = []struct {
	label string
	days  int
}{{"today", 0}, {"last 7 days", 6}, {"last 30 days", 29}, {"last 365 days", 364}}

type Model struct {
	opt   Options
	w, h  int
	in    [nFields]textinput.Model
	focus int // -1 = list has focus
	bad   [nFields]bool
	stat  statusFilter

	preset  int // index into datePresets+1; 0 = none
	commits []store.Commit
	cursor  int
	top     int // first visible display row
	stats   store.Stats

	detail  bool // narrow terminals: show detail instead of list
	help    bool
	notice  string
	isErr   bool
	latest  string
	busy    string // "syncing", "updating"
	loadErr error
}

type (
	syncDoneMsg struct {
		n   int
		err error
	}
	updateInfoMsg struct {
		latest string
		err    error
	}
	updateDoneMsg struct {
		version string
		err     error
	}
)

func New(o Options) *Model {
	if o.Now == nil {
		o.Now = time.Now
	}
	m := &Model{opt: o, focus: -1, w: 120, h: 32}
	ph := [nFields]string{"message, hash, project or author", "name", "name or email", "YYYY-MM-DD", "YYYY-MM-DD"}
	for i := range m.in {
		t := textinput.New()
		t.Prompt = ""
		t.Placeholder = ph[i]
		t.PlaceholderStyle = sDim
		t.CharLimit = 80
		if i >= fSince {
			t.CharLimit = 10
		}
		m.in[i] = t
	}
	m.reload()
	return m
}

func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.opt.Sync != nil {
		m.busy = "syncing"
		cmds = append(cmds, m.syncCmd())
	}
	if m.opt.CheckUpdate != nil {
		cmds = append(cmds, func() tea.Msg {
			v, err := m.opt.CheckUpdate()
			return updateInfoMsg{v, err}
		})
	}
	return tea.Batch(cmds...)
}

func (m *Model) syncCmd() tea.Cmd {
	return func() tea.Msg {
		n, err := m.opt.Sync()
		return syncDoneMsg{n, err}
	}
}

// filter turns the UI state into a store query. Half-typed dates are ignored and flagged.
func (m *Model) filter() store.CommitFilter {
	f := store.CommitFilter{
		Text:            strings.TrimSpace(m.in[fSearch].Value()),
		ProjectContains: strings.TrimSpace(m.in[fProject].Value()),
		Author:          strings.TrimSpace(m.in[fAuthor].Value()),
	}
	switch m.stat {
	case statusPending:
		v := false
		f.Pushed = &v
	case statusPushed:
		v := true
		f.Pushed = &v
	}
	m.bad[fSince], m.bad[fUntil] = false, false
	if v := strings.TrimSpace(m.in[fSince].Value()); v != "" {
		if d, err := store.BuildDateFilter(v, "", ""); err != nil {
			m.bad[fSince] = true
		} else {
			f.Since = d.Since
		}
	}
	if v := strings.TrimSpace(m.in[fUntil].Value()); v != "" {
		if d, err := store.BuildDateFilter("", v, ""); err != nil {
			m.bad[fUntil] = true
		} else {
			f.Until = d.Until
		}
	}
	return f
}

func (m *Model) hasFilters() bool {
	for i := range m.in {
		if strings.TrimSpace(m.in[i].Value()) != "" {
			return true
		}
	}
	return m.stat != statusAll
}

// reload re-runs the query, keeping the selection on the same commit when it survives.
func (m *Model) reload() {
	var keep string
	if c := m.selected(); c != nil {
		keep = c.Hash
	}
	commits, err := m.opt.Store.QueryCommits(m.filter())
	m.loadErr = err
	m.commits = commits
	m.cursor = 0
	for i, c := range commits {
		if c.Hash == keep {
			m.cursor = i
			break
		}
	}
	if st, err := m.opt.Store.Stats(); err == nil {
		m.stats = st
	}
	m.ensureVisible()
}

func (m *Model) selected() *store.Commit {
	if m.cursor < 0 || m.cursor >= len(m.commits) {
		return nil
	}
	return &m.commits[m.cursor]
}

func (m *Model) say(msg string, isErr bool) { m.notice, m.isErr = msg, isErr }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.ensureVisible()
	case syncDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.say("sync failed: "+msg.err.Error(), true)
		} else {
			m.say(fmt.Sprintf("synced %d repos", msg.n), false)
		}
		m.reload()
	case updateInfoMsg:
		if msg.err == nil && m.opt.Version != "" && msg.latest != "" && update.Newer(msg.latest, m.opt.Version) {
			m.latest = msg.latest
		}
	case updateDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.say("update failed: "+msg.err.Error(), true)
		} else {
			m.say("updated to v"+msg.version+" — restart walkline to use it", false)
			m.latest = ""
		}
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *Model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	if s == "ctrl+c" {
		return m, tea.Quit
	}
	m.notice = ""
	if m.help {
		m.help = false
		return m, nil
	}
	if m.focus >= 0 {
		return m, m.filterKey(k)
	}
	switch s {
	case "q":
		return m, tea.Quit
	case "esc":
		if m.detail {
			m.detail = false
		} else if m.hasFilters() {
			m.clear()
		}
	case "?":
		m.help = true
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "pgdown", "ctrl+d":
		m.move(m.listHeight() / 2)
	case "pgup", "ctrl+u":
		m.move(-m.listHeight() / 2)
	case "g", "home":
		m.move(-len(m.commits))
	case "G", "end":
		m.move(len(m.commits))
	case "enter", "l", "right":
		m.detail = !m.detail
	case "h", "left":
		m.detail = false
	case "/":
		return m, m.focusField(fSearch)
	case "p":
		return m, m.focusField(fProject)
	case "a":
		return m, m.focusField(fAuthor)
	case "f":
		return m, m.focusField(fSince)
	case "t":
		return m, m.focusField(fUntil)
	case "tab":
		return m, m.focusField(fSearch)
	case "s":
		m.stat = (m.stat + 1) % 3
		m.reload()
	case "d":
		m.cyclePreset()
	case "x":
		m.clear()
	case "r":
		if m.opt.Sync != nil && m.busy == "" {
			m.busy = "syncing"
			return m, m.syncCmd()
		}
	case "u":
		return m, m.startUpdate()
	}
	return m, nil
}

func (m *Model) startUpdate() tea.Cmd {
	if m.latest == "" {
		m.say("already on the latest release", false)
		return nil
	}
	if m.opt.Install == nil || m.busy != "" {
		return nil
	}
	m.busy = "updating"
	v := m.latest
	return func() tea.Msg { return updateDoneMsg{v, m.opt.Install(v)} }
}

func (m *Model) filterKey(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "esc", "enter":
		m.blur()
		return nil
	case "tab", "down":
		return m.focusField((m.focus + 1) % nFields)
	case "shift+tab", "up":
		return m.focusField((m.focus + nFields - 1) % nFields)
	}
	before := m.in[m.focus].Value()
	var cmd tea.Cmd
	m.in[m.focus], cmd = m.in[m.focus].Update(k)
	if m.in[m.focus].Value() != before {
		if m.focus >= fSince {
			m.preset = 0
		}
		m.reload()
	}
	return cmd
}

func (m *Model) focusField(i int) tea.Cmd {
	m.blur()
	m.focus = i
	m.detail = false
	return m.in[i].Focus()
}

func (m *Model) blur() {
	if m.focus >= 0 {
		m.in[m.focus].Blur()
	}
	m.focus = -1
}

func (m *Model) clear() {
	for i := range m.in {
		m.in[i].SetValue("")
	}
	m.stat, m.preset = statusAll, 0
	m.reload()
}

// cyclePreset steps none → today → 7d → 30d → 365d → none by filling the date fields.
func (m *Model) cyclePreset() {
	m.preset = (m.preset + 1) % (len(datePresets) + 1)
	m.in[fSince].SetValue("")
	m.in[fUntil].SetValue("")
	if m.preset > 0 {
		p := datePresets[m.preset-1]
		m.in[fSince].SetValue(m.opt.Now().AddDate(0, 0, -p.days).Format("2006-01-02"))
	}
	m.reload()
}

func (m *Model) move(d int) {
	m.cursor += d
	if m.cursor >= len(m.commits) {
		m.cursor = len(m.commits) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureVisible()
}
