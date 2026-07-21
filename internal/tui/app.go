package tui

import (
	"fmt"
	"strings"

	"codebuddy-router/internal/config"
	"codebuddy-router/internal/router"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Theme ─────────────────────────────────────────────────────

var (
	P = lipgloss.Color("#A78BFA")
	G = lipgloss.Color("#69DB7C")
	R = lipgloss.Color("#FF6B6B")
	Y = lipgloss.Color("#FBBF24")
	B = lipgloss.Color("#60A5FA")
	D = lipgloss.Color("#6B7280")

	titleS  = lipgloss.NewStyle().Bold(true).Foreground(P)
	dimS    = lipgloss.NewStyle().Foreground(D)
	greenS  = lipgloss.NewStyle().Foreground(G)
	redS    = lipgloss.NewStyle().Foreground(R)
	yellowS = lipgloss.NewStyle().Foreground(Y)
	blueS   = lipgloss.NewStyle().Foreground(B)
	accentS = lipgloss.NewStyle().Bold(true).Foreground(P)
)

// ─── Model ─────────────────────────────────────────────────────

type Model struct {
	pool     *router.KeyPool
	events   chan router.LogEntry
	cfg      *config.Config
	logs     []router.LogEntry
	width    int
	height   int
	ready    bool
	minimized bool
	total    int
	success  int
	failed   int
}

// tickMsg triggers periodic re-render and channel drain.
type tickMsg struct{}

func New(pool *router.KeyPool, events chan router.LogEntry, cfg *config.Config) Model {
	return Model{
		pool:   pool,
		events: events,
		cfg:    cfg,
		logs:   make([]router.LogEntry, 0, 100),
	}
}

func (m Model) Init() tea.Cmd {
	return drain(m.events)
}

// drain is the single persistent cmd that reads ALL available log entries
// from the channel and sends them as a batch message.
func drain(ch <-chan router.LogEntry) tea.Cmd {
	return func() tea.Msg {
		entries := make([]router.LogEntry, 0, 16)
		// Non-blocking: drain all currently available entries
		for {
			select {
			case e, ok := <-ch:
				if !ok {
					return nil
				}
				entries = append(entries, e)
			default:
				// No more entries right now
				if len(entries) > 0 {
					return logsMsg(entries)
				}
				// Nothing at all — wait for next
				e, ok := <-ch
				if !ok {
					return nil
				}
				return logsMsg([]router.LogEntry{e})
			}
		}
	}
}

type logsMsg []router.LogEntry

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		// ClearScreen + keep draining
		return m, tea.Batch(tea.ClearScreen, drain(m.events))

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "m":
			m.minimized = !m.minimized
		case "l":
			m.logs = m.logs[:0]
			m.total = 0
			m.success = 0
			m.failed = 0
		}
		return m, drain(m.events)

	case logsMsg:
		for _, entry := range msg {
			const maxLogs = 200
			if len(m.logs) >= maxLogs {
				m.logs = m.logs[1:]
			}
			m.logs = append(m.logs, entry)
			m.total++
			if entry.StatusCode >= 200 && entry.StatusCode < 300 {
				m.success++
			} else {
				m.failed++
			}
		}
		// Keep draining
		return m, drain(m.events)
	}
	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}
	if m.minimized {
		return m.renderMinimized()
	}
	return m.renderFull()
}

// ─── Minimized View ────────────────────────────────────────────

func (m Model) renderMinimized() string {
	return fmt.Sprintf(" %s  %s  %s  %s  %s  %s",
		titleS.Render("CB Router"),
		dimS.Render(fmt.Sprintf(":%s", m.cfg.Port)),
		proxyStatus(m.cfg.Proxy),
		dimS.Render(fmt.Sprintf("%d models", len(router.Models))),
		dimS.Render(fmt.Sprintf("T:%d ✅:%d ❌:%d", m.total, m.success, m.failed)),
		dimS.Render("[m] expand  [q] quit"),
	)
}

// ─── Full View ─────────────────────────────────────────────────

func (m Model) renderFull() string {
	w := maxI(m.width, 30)
	b := &strings.Builder{}

	// Header
	b.WriteString(titleS.Render("CodeBuddy CLI Gateway") + "\n")
	b.WriteString(dimS.Render(strings.Repeat("─", w-2)) + "\n\n")

	// Info
	info := []struct{ label, value string }{
		{"Base URL", yellowS.Render(fmt.Sprintf("http://0.0.0.0:%s/v1", m.cfg.Port))},
		{"Models", accentS.Render(fmt.Sprintf("%d", len(router.Models)))},
		{"Keys", fmt.Sprintf("%s (%s)", accentS.Render(fmt.Sprintf("%d", len(m.cfg.APIKeys))), m.keyStatusSummary())},
		{"API Key", yellowS.Render(m.firstRouterKey())},
	}
	for _, i := range info {
		b.WriteString(fmt.Sprintf("  %-10s: %s\n", dimS.Render(i.label), i.value))
	}
	b.WriteString("\n")

	// Stats
	b.WriteString(fmt.Sprintf("  Total request: %s  ✅: %s  ❌: %s  ⚡: %s\n\n",
		accentS.Render(fmt.Sprintf("%d", m.total)),
		greenS.Render(fmt.Sprintf("%d", m.success)),
		redS.Render(fmt.Sprintf("%d", m.failed)),
		blueS.Render(fmt.Sprintf("%d/min", m.throughput())),
	))

	// Log entries (max 8)
	logH := 8
	logs := m.logs
	if len(logs) > logH {
		logs = logs[len(logs)-logH:]
	}

	for _, e := range logs {
		ts := dimS.Render(e.Time.Format("15:04:05"))
		lat := dimS.Render(fmt.Sprintf("%.1fs", e.Latency.Seconds()))
		model := blueS.Render(truncRunes(e.Model, 20))
		ki := dimS.Render(fmt.Sprintf("k%d", e.KeyIndex+1))

		var sc lipgloss.Style
		switch {
		case e.StatusCode >= 200 && e.StatusCode < 300:
			sc = greenS
		case e.StatusCode == 429:
			sc = yellowS
		default:
			sc = redS
		}
		status := sc.Render(fmt.Sprintf("%d", e.StatusCode))

		line := fmt.Sprintf("  %s   %s   %s   %-20s   %s", ts, status, lat, model, ki)
		if e.Error != "" {
			line += "   " + redS.Render(truncRunes(e.Error, maxI(w-58, 4)))
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")

	// Footer
	b.WriteString(dimS.Render("  [m] minimize  [l] clear  [q] quit"))

	return b.String()
}

// ─── Helpers ───────────────────────────────────────────────────

func (m Model) firstRouterKey() string {
	return m.cfg.RouterKey
}

func (m Model) keyStatusSummary() string {
	active := 0
	if m.pool != nil {
		for _, ks := range m.pool.All() {
			if !ks.Failed {
				active++
			}
		}
	} else {
		active = len(m.cfg.APIKeys)
	}
	return fmt.Sprintf("%d active", active)
}

func (m Model) throughput() int {
	if m.total == 0 {
		return 0
	}
	if len(m.logs) < 2 {
		return 0
	}
	first := m.logs[0].Time
	last := m.logs[len(m.logs)-1].Time
	duration := last.Sub(first).Minutes()
	if duration < 0.01 {
		return 0
	}
	return int(float64(m.total) / duration)
}

func proxyStatus(proxy string) string {
	if proxy == "" {
		return greenS.Render("● direct")
	}
	return yellowS.Render("● proxy")
}

func truncRunes(s string, max int) string {
	if max < 1 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}
