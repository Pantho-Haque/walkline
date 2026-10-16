package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const (
	headerH = 1
	filterH = 4 // two inner lines + border
	footerH = 1
	minW    = 72
	minH    = 12
)

// row is one display line of the list: a day header or a commit.
type row struct {
	header string // non-empty for day headers
	idx    int    // commit index otherwise
}

func (m *Model) rows() []row {
	var out []row
	last, count := "", map[string]int{}
	for _, c := range m.commits {
		count[day(c.CommittedAt)]++
	}
	for i, c := range m.commits {
		d := day(c.CommittedAt)
		if d != last {
			out = append(out, row{header: dayLabel(d, count[d])})
			last = d
		}
		out = append(out, row{idx: i})
	}
	return out
}

func day(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}

func dayLabel(d string, n int) string {
	label := d
	if t, err := time.Parse("2006-01-02", d); err == nil {
		label = t.Format("Mon 02 Jan 2006")
	}
	noun := "commits"
	if n == 1 {
		noun = "commit"
	}
	return fmt.Sprintf("%s · %d %s", label, n, noun)
}

func hhmm(ts string) string {
	if len(ts) >= 16 {
		return ts[11:16]
	}
	return "--:--"
}

func (m *Model) bodyH() int { return m.h - headerH - filterH - footerH }

// listHeight is how many display rows fit inside the list pane (minus border and column header).
func (m *Model) listHeight() int {
	if h := m.bodyH() - 3; h > 1 {
		return h
	}
	return 1
}

func (m *Model) split() bool { return m.w >= 100 }

func (m *Model) ensureVisible() {
	rows, h := m.rows(), m.listHeight()
	sel := -1
	for i, r := range rows {
		if r.header == "" && r.idx == m.cursor {
			sel = i
			break
		}
	}
	if sel < 0 {
		m.top = 0
		return
	}
	if sel > 0 && rows[sel-1].header != "" {
		sel-- // keep the day header with its first commit
		if sel < m.top {
			m.top = sel
		}
		sel++
	}
	if sel < m.top {
		m.top = sel
	}
	if sel >= m.top+h {
		m.top = sel - h + 1
	}
	if m.top < 0 {
		m.top = 0
	}
}

// clip keeps at most n lines so a pane can never grow past its slot.
func clip(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

func fit(s string, w int) string {
	s = strings.Join(strings.Fields(s), " ")
	return runewidth.FillRight(runewidth.Truncate(s, w, "…"), w)
}

func (m *Model) View() string {
	if m.w < minW || m.h < minH {
		return fmt.Sprintf("Terminal too small (%dx%d). Need at least %dx%d.", m.w, m.h, minW, minH)
	}
	body := m.body()
	return lipgloss.JoinVertical(lipgloss.Left, m.header(), m.filterBar(), body, m.footer())
}

func (m *Model) header() string {
	left := sTitle.Render("walkline")
	if m.opt.Version != "" {
		left += sDim.Render(" " + m.opt.Version)
	}
	right := fmt.Sprintf("%d commits  ", m.stats.Total) +
		lipgloss.NewStyle().Foreground(cPending).Render(fmt.Sprintf("● %d pending", m.stats.Pending)) +
		sDim.Render(fmt.Sprintf("  ▣ %d projects", m.stats.Projects))
	switch {
	case m.busy != "":
		right = sAccent.Render(m.busy+"…") + "  " + right
	case m.latest != "":
		right = sAccent.Render("⬆ v"+m.latest+" available (u)") + "  " + right
	}
	gap := m.w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

var shortLabels = [nFields]string{"Search", "Proj", "Auth", "From", "To"}

func (m *Model) label(i int) string {
	if m.w < 100 {
		return shortLabels[i]
	}
	return fieldLabels[i]
}

func (m *Model) field(i, width int) string {
	m.in[i].Width = width - 1 // leave a cell for the cursor
	name := m.label(i)
	label := sDim.Render(name + " ")
	if m.focus == i {
		label = sAccent.Render(name + " ")
	}
	v := m.in[i].View()
	if m.bad[i] {
		v = sError.Render(m.in[i].Value())
		if pad := width - lipgloss.Width(v); pad > 0 {
			v += strings.Repeat(" ", pad)
		}
	}
	return label + lipgloss.NewStyle().Width(width).MaxWidth(width).Render(v)
}

func (m *Model) filterBar() string {
	inner := m.w - 4
	var statusColor lipgloss.TerminalColor = cDim
	if m.stat == statusPending {
		statusColor = cPending
	} else if m.stat == statusPushed {
		statusColor = cPushed
	}
	status := sDim.Render("Status ") + lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(m.stat.String()+" ⇄")
	if m.preset > 0 {
		status += sDim.Render("   Range ") + sAccent.Render(datePresets[m.preset-1].label)
	}
	line1 := m.field(fSearch, max(10, inner-lipgloss.Width(status)-len(m.label(fSearch))-1-3)) + "   " + status

	dates := 10
	rest := inner - 2 - (len("Project ") + len("Author ") + len("Since ") + len("Until ")) - 3*3 - 2*dates
	pw := max(6, rest/2)
	line2 := strings.Join([]string{m.field(fProject, pw), m.field(fAuthor, pw), m.field(fSince, dates+1), m.field(fUntil, dates+1)}, "   ")

	st := sPane
	if m.focus >= 0 {
		st = sPaneFocus
	}
	return st.Width(m.w-2).Padding(0, 1).Render(line1 + "\n" + line2)
}

func (m *Model) body() string {
	h := m.bodyH()
	if m.help {
		return sPane.Width(m.w - 2).Height(h - 2).Render(clip(m.helpText(), h-2))
	}
	if !m.split() {
		if m.detail {
			return sPane.Width(m.w - 2).Height(h - 2).Render(clip(m.detailText(m.w-6), h-2))
		}
		return m.listPane(m.w, h)
	}
	lw := m.w * 58 / 100
	dw := m.w - lw
	detail := sPane.Width(dw - 2).Height(h - 2).Render(clip(m.detailText(dw-6), h-2))
	return lipgloss.JoinHorizontal(lipgloss.Top, m.listPane(lw, h), detail)
}

func (m *Model) listPane(w, h int) string {
	st := sPane
	if m.focus < 0 {
		st = sPaneFocus
	}
	iw := w - 4 // border + one cell of padding each side
	lines := []string{sDim.Render(fitCol("  TIME   "+fitCol("PROJECT", projW(iw))+"  HASH       MESSAGE", iw))}
	lh := m.listHeight()

	switch {
	case m.loadErr != nil:
		lines = append(lines, sError.Render(fit("Query failed: "+m.loadErr.Error(), iw)))
	case len(m.commits) == 0 && m.hasFilters():
		lines = append(lines, "", sDim.Render("  No commits match these filters."), sDim.Render("  Press x to clear them."))
	case len(m.commits) == 0:
		lines = append(lines, "", sDim.Render("  No commits recorded yet."),
			sDim.Render("  Run `walkline install`, then `walkline scan <dir>`, and commit something."))
	default:
		rows := m.rows()
		end := min(len(rows), m.top+lh)
		for _, r := range rows[m.top:end] {
			if r.header != "" {
				lines = append(lines, sAccent.Render(fit("▾ "+r.header, iw)))
				continue
			}
			lines = append(lines, m.commitLine(r.idx, iw))
		}
	}
	return st.Width(w - 2).Height(h - 2).Render(strings.Join(lines, "\n"))
}

func projW(iw int) int { return min(22, max(8, iw/4)) }

func fitCol(s string, w int) string { return runewidth.FillRight(runewidth.Truncate(s, w, "…"), w) }

func (m *Model) commitLine(i, iw int) string {
	c := m.commits[i]
	sel := i == m.cursor && m.focus < 0
	pw := projW(iw)
	msgW := max(0, iw-2-5-2-pw-2-7-2-1-2)

	glyph, gc := "●", cPending
	if c.Pushed {
		glyph, gc = "✓", cPushed
	}
	marker := "  "
	if sel {
		marker = "▌ "
	}
	sp := fg(cText, sel).Render("  ")
	return fg(cAccent, sel).Render(marker) +
		fg(cDim, sel).Render(hhmm(c.CommittedAt)) + sp +
		fg(cText, sel).Render(fitCol(c.ProjectName, pw)) + sp +
		fg(cDim, sel).Render(c.Hash[:min(7, len(c.Hash))]) + sp +
		fg(gc, sel).Render(glyph) + sp +
		fg(cText, sel).Render(fit(subject(c.Message), msgW))
}

func subject(msg string) string {
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		return msg[:i]
	}
	return msg
}

func (m *Model) detailText(w int) string {
	c := m.selected()
	if c == nil {
		return sDim.Render("Select a commit to see its details.")
	}
	kv := func(k, v string) string {
		lines := strings.Split(lipgloss.NewStyle().Width(max(10, w-10)).Render(v), "\n")
		return sDim.Render(fmt.Sprintf("%-10s", k)) + strings.Join(lines, "\n"+strings.Repeat(" ", 10))
	}
	status := lipgloss.NewStyle().Foreground(cPending).Render("● pending push")
	if c.Pushed {
		status = lipgloss.NewStyle().Foreground(cPushed).Render("✓ pushed")
		if c.PushedAt != "" {
			status += sDim.Render("  " + c.PushedAt)
		}
	}
	wrap := lipgloss.NewStyle().Width(max(10, w))
	out := []string{
		sAccent.Render(c.ProjectName),
		"",
		kv("Commit", c.Hash[:min(12, len(c.Hash))]),
		kv("Author", c.AuthorName+" <"+c.AuthorEmail+">"),
		kv("Date", c.CommittedAt),
		kv("Status", status),
		kv("Path", c.ProjectPath),
	}
	if c.RemoteURL != "" {
		out = append(out, kv("Remote", c.RemoteURL))
	}
	out = append(out, "", wrap.Render(strings.TrimSpace(c.Message)))
	return wrap.Render(strings.Join(out, "\n"))
}

func (m *Model) helpText() string {
	keys := [][2]string{
		{"j / k  ↑ / ↓", "move selection"},
		{"g / G", "jump to newest / oldest"},
		{"ctrl+d / ctrl+u", "half page down / up"},
		{"enter", "toggle commit details (narrow terminals)"},
		{"/", "search message, hash, project, author"},
		{"p  a", "filter by project / author"},
		{"f  t", "filter from / until date (YYYY-MM-DD)"},
		{"tab / shift+tab", "next / previous filter field"},
		{"enter / esc", "leave filter field"},
		{"s", "cycle status: all → pending → pushed"},
		{"d", "cycle range: today → 7d → 30d → 365d → none"},
		{"x  or  esc", "clear all filters"},
		{"r", "sync push status with git remotes"},
		{"u", "install the latest release (when shown)"},
		{"?", "toggle this help"},
		{"q / ctrl+c", "quit"},
	}
	var b strings.Builder
	b.WriteString(sAccent.Render("Keyboard shortcuts") + "\n\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s  %s\n", sAccent.Render(fmt.Sprintf("%-16s", k[0])), k[1])
	}
	b.WriteString("\n" + sDim.Render("Press any key to close."))
	return b.String()
}

func (m *Model) footer() string {
	var hints [][2]string
	switch {
	case m.help:
		hints = [][2]string{{"any key", "close"}}
	case m.focus >= 0:
		hints = [][2]string{{"type", "filter live"}, {"tab", "next field"}, {"enter/esc", "back to list"}}
	default:
		hints = [][2]string{{"/", "search"}, {"p a f t", "filters"}, {"s", "status"}, {"d", "range"}, {"x", "clear"}, {"r", "sync"}, {"?", "help"}, {"q", "quit"}}
	}
	right := sDim.Render(fmt.Sprintf("%d/%d", min(m.cursor+1, len(m.commits)), len(m.commits)))
	if m.notice != "" {
		right = m.notice
		if m.isErr {
			right = sError.Render(right)
		} else {
			right = lipgloss.NewStyle().Foreground(cPushed).Render(right)
		}
	}
	var left string
	for { // drop low-priority hints (just before the last two) until the line fits
		parts := make([]string, len(hints))
		for i, h := range hints {
			parts[i] = sAccent.Render(h[0]) + " " + sDim.Render(h[1])
		}
		left = strings.Join(parts, sDim.Render(" · "))
		if lipgloss.Width(left)+lipgloss.Width(right)+2 <= m.w || len(hints) <= 2 {
			break
		}
		hints = append(hints[:len(hints)-3], hints[len(hints)-2:]...)
	}
	gap := m.w - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}
