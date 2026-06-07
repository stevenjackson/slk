package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	slkconfig "github.com/stevejackson/slk/internal/config"
	slkdb "github.com/stevejackson/slk/internal/db"
)

// --- styles ---

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	normalStyle = lipgloss.NewStyle()

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	channelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	replyCountStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("178"))
)

// --- model ---

type viewMode int

const (
	listView viewMode = iota
	cardView
)

type model struct {
	db        *sql.DB
	threads   []slkdb.Thread
	filtered  []int // indices into threads currently visible (search filter applied)
	searching bool
	query     string
	cursor    int
	mode      viewMode
	viewport  viewport.Model
	width     int
	height    int
	renderer  *glamour.TermRenderer
	cache     map[string]string            // ts → rendered card
	unfurls   map[string]map[string]string // ts → (url → tweet text)
	err       error
}

func initialModel() (model, error) {
	db, err := slkdb.Open()
	if err != nil {
		return model{}, err
	}
	threads, err := slkdb.LoadInbox(db)
	if err != nil {
		db.Close()
		return model{}, err
	}
	filtered := make([]int, len(threads))
	for i := range threads {
		filtered[i] = i
	}
	return model{
		db:       db,
		threads:  threads,
		filtered: filtered,
		cache:    make(map[string]string),
		unfurls:  make(map[string]map[string]string),
	}, nil
}

// applyFilter rebuilds m.filtered from m.threads based on m.query (case-insensitive
// substring match against author, channel, and text).
func (m *model) applyFilter() {
	if m.query == "" {
		m.filtered = make([]int, len(m.threads))
		for i := range m.threads {
			m.filtered[i] = i
		}
		return
	}
	q := strings.ToLower(m.query)
	m.filtered = m.filtered[:0]
	for i, t := range m.threads {
		if strings.Contains(strings.ToLower(t.Text), q) ||
			strings.Contains(strings.ToLower(t.Author), q) ||
			strings.Contains(strings.ToLower(t.ChannelName), q) {
			m.filtered = append(m.filtered, i)
		}
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

// --- update ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport = viewport.New(msg.Width, msg.Height-3)
		// Rebuild renderer and clear cache on resize — word wrap width changed
		m.renderer, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(msg.Width),
		)
		m.cache = make(map[string]string)
		if m.mode == cardView && len(m.filtered) > 0 {
			m.viewport.SetContent(m.renderCard(m.threads[m.filtered[m.cursor]]))
		}

	case unfurledMsg:
		m.unfurls[msg.ts] = msg.unfurls
		delete(m.cache, msg.ts) // invalidate so card re-renders with tweet text
		if m.mode == cardView && len(m.filtered) > 0 && m.threads[m.filtered[m.cursor]].TS == msg.ts {
			m.viewport.SetContent(m.renderCard(m.threads[m.filtered[m.cursor]]))
		}

	case tea.KeyMsg:
		switch m.mode {
		case listView:
			return m.updateList(msg)
		case cardView:
			return m.updateCard(msg)
		}
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.searching {
		return m.updateSearch(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.db.Close()
		return m, tea.Quit

	case "/":
		m.searching = true
		m.query = ""
		m.applyFilter()
		m.cursor = 0

	case "j", "down":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}

	case "ctrl+d":
		step := max(1, (m.height-5)/2)
		m.cursor = min(m.cursor+step, len(m.filtered)-1)

	case "ctrl+u":
		step := max(1, (m.height-5)/2)
		m.cursor = max(m.cursor-step, 0)

	case "enter", " ":
		if len(m.filtered) > 0 {
			t := m.threads[m.filtered[m.cursor]]
			m.mode = cardView
			m.viewport.SetContent(m.renderCard(t))
			m.viewport.GotoTop()
			return m, fetchUnfurls(t)
		}

	case "r":
		if len(m.filtered) > 0 {
			idx := m.filtered[m.cursor]
			t := m.threads[idx]
			slkdb.SetStatus(m.db, t.TS, "read")
			m.threads = append(m.threads[:idx], m.threads[idx+1:]...)
			m.applyFilter()
			if m.cursor > 0 && m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
		}
	}
	return m, nil
}

// updateSearch handles key input while the "/" filter prompt is active.
func (m model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.searching = false
		m.query = ""
		m.applyFilter()
		m.cursor = 0

	case tea.KeyEnter:
		m.searching = false

	case tea.KeyBackspace:
		if len(m.query) > 0 {
			m.query = m.query[:len(m.query)-1]
			m.applyFilter()
			m.cursor = 0
		}

	case tea.KeyRunes, tea.KeySpace:
		m.query += string(msg.Runes)
		m.applyFilter()
		m.cursor = 0
	}
	return m, nil
}

func (m model) updateCard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.db.Close()
		return m, tea.Quit

	case "esc", "backspace":
		m.mode = listView

	case "r":
		if len(m.filtered) > 0 {
			idx := m.filtered[m.cursor]
			t := m.threads[idx]
			slkdb.SetStatus(m.db, t.TS, "read")
			m.threads = append(m.threads[:idx], m.threads[idx+1:]...)
			m.applyFilter()
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			if len(m.filtered) == 0 {
				m.mode = listView
			} else {
				next := m.threads[m.filtered[m.cursor]]
				m.viewport.SetContent(m.renderCard(next))
				m.viewport.GotoTop()
				return m, fetchUnfurls(next)
			}
		}

	case "ctrl+d":
		m.viewport.HalfPageDown()

	case "ctrl+u":
		m.viewport.HalfPageUp()

	case "n":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			t := m.threads[m.filtered[m.cursor]]
			m.viewport.SetContent(m.renderCard(t))
			m.viewport.GotoTop()
			return m, fetchUnfurls(t)
		}

	case "p":
		if m.cursor > 0 {
			m.cursor--
			t := m.threads[m.filtered[m.cursor]]
			m.viewport.SetContent(m.renderCard(t))
			m.viewport.GotoTop()
			return m, fetchUnfurls(t)
		}

	case "o":
		if len(m.filtered) > 0 {
			openURL(m.threads[m.filtered[m.cursor]].SlackURL)
		}

	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// --- view ---

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("error: %v\n", m.err)
	}
	if len(m.threads) == 0 {
		return "inbox empty\n"
	}
	switch m.mode {
	case listView:
		return m.viewList()
	case cardView:
		return m.viewCard()
	}
	return ""
}

func (m model) viewList() string {
	var b strings.Builder

	header := headerStyle.Render(fmt.Sprintf("inbox  %d unread", len(m.threads)))
	if m.query != "" {
		header += "  " + dimStyle.Render(fmt.Sprintf("(filter %q: %d match)", m.query, len(m.filtered)))
	}
	b.WriteString(header + "\n\n")

	if len(m.filtered) == 0 {
		b.WriteString(dimStyle.Render("no matches") + "\n")
	}

	start := 0
	maxRows := m.height - 5
	if maxRows < 1 {
		maxRows = 10
	}
	if m.cursor >= maxRows {
		start = m.cursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := start; i < end; i++ {
		t := m.threads[m.filtered[i]]
		indicator := noiseLabel(noiseScore(t))
		channel := channelStyle.Render("#" + t.ChannelName)
		author := t.Author
		replies := ""
		if t.ReplyCount > 0 {
			replies = replyCountStyle.Render(fmt.Sprintf(" [%d]", t.ReplyCount))
		}
		first := firstLine(t.Text, m.width-44)
		time := dimStyle.Render(slkdb.FormatTime(t.TS))

		line := fmt.Sprintf("%s %-18s  %-16s  %s%s  %s", indicator, time, channel, author, replies, first)
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(">"+line) + "\n")
		} else {
			b.WriteString(normalStyle.Render(" "+line) + "\n")
		}
	}

	if m.searching {
		b.WriteString("\n" + helpStyle.Render("/"+m.query+"█"))
	} else {
		b.WriteString("\n" + helpStyle.Render("j/k navigate  ctrl+d/ctrl+u page  enter card  / search  r read  q quit"))
	}
	return b.String()
}

func (m model) viewCard() string {
	var b strings.Builder
	t := m.threads[m.filtered[m.cursor]]

	header := fmt.Sprintf("%s  %s  #%s",
		headerStyle.Render(t.Author),
		dimStyle.Render(slkdb.FormatTime(t.TS)),
		channelStyle.Render(t.ChannelName),
	)
	b.WriteString(header + "\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n" + helpStyle.Render("j/k scroll  n/p next/prev  r read  o open  esc back  q quit"))
	return b.String()
}

func (m *model) renderCard(t slkdb.Thread) string {
	if cached, ok := m.cache[t.TS]; ok {
		return cached
	}

	threadUnfurls := m.unfurls[t.TS]

	var md strings.Builder
	md.WriteString(inlineUnfurls(t.Text, threadUnfurls))
	if len(t.Replies) > 0 {
		md.WriteString(fmt.Sprintf("\n\n---\n*%d replies*\n\n", len(t.Replies)))
		for _, r := range t.Replies {
			md.WriteString(fmt.Sprintf("**%s**: %s\n\n", r.Author, inlineUnfurls(r.Text, threadUnfurls)))
		}
	}

	if m.renderer == nil {
		m.cache[t.TS] = md.String()
		return md.String()
	}
	out, err := m.renderer.Render(md.String())
	if err != nil {
		m.cache[t.TS] = md.String()
		return md.String()
	}
	m.cache[t.TS] = out
	return out
}

// --- helpers ---

func firstLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if max > 0 && len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	cmd.Start()
}

// --- main ---

func main() {
	slkconfig.Load()
	m, err := initialModel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
