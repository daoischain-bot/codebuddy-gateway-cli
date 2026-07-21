package tui

import (
	"testing"
	"time"

	"codebuddy-router/internal/config"
	"codebuddy-router/internal/router"
	tea "github.com/charmbracelet/bubbletea"
)

// testCfg returns a minimal config for rendering.
func testCfg() *config.Config {
	return &config.Config{
		Port:       "6900",
		RouterKey: "rtr_testkey123",
		APIKeys:    []string{"key1", "key2", "key3"},
		Proxy:      "",
	}
}

// testLogs returns a handful of log entries for exercising the log display.
func testLogs() []router.LogEntry {
	now := time.Now()
	return []router.LogEntry{
		{Time: now.Add(-10 * time.Second), Model: "claude-opus-4.7-1m", StatusCode: 200, Latency: 1200 * time.Millisecond, KeyIndex: 0},
		{Time: now.Add(-8 * time.Second), Model: "gpt-5.5", StatusCode: 200, Latency: 400 * time.Millisecond, KeyIndex: 1},
		{Time: now.Add(-5 * time.Second), Model: "gemini-3.5-flash", StatusCode: 429, Latency: 100 * time.Millisecond, Error: "rate limited", KeyIndex: 2},
		{Time: now.Add(-3 * time.Second), Model: "deepseek-v4-pro", StatusCode: 500, Latency: 2000 * time.Millisecond, Error: "internal server error", KeyIndex: 0},
		{Time: now.Add(-1 * time.Second), Model: "glm-5.2", StatusCode: 200, Latency: 800 * time.Millisecond, KeyIndex: 1},
	}
}

// updateModel sends a msg through Update and returns the concrete Model.
func updateModel(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	result, _ := m.Update(msg)
	return result.(Model)
}

// renderAt sets the model to a given width×height and calls View().
func renderAt(t *testing.T, m Model, w, h int) string {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic at %dx%d: %v", w, h, r)
		}
	}()
	m = updateModel(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	return m.View()
}

// ─── Tests ────────────────────────────────────────────────────

func TestRenderAllSizes(t *testing.T) {
	sizes := []struct {
		w, h int
		name string
	}{
		{30, 10, "narrow"},
		{40, 12, "small"},
		{80, 24, "standard"},
		{120, 40, "wide"},
		{200, 50, "ultra-wide"},
	}

	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			m := New(nil, nil, testCfg())
			for _, entry := range testLogs() {
				m = updateModel(t, m, logsMsg{entry})
			}
			view := renderAt(t, m, sz.w, sz.h)
			if view == "" {
				t.Errorf("View() returned empty string at %dx%d", sz.w, sz.h)
			}
			if len(view) > 0 && sz.w >= 30 {
				if !contains(view, "CodeBuddy CLI Gateway") {
					t.Errorf("expected 'CodeBuddy CLI Gateway' in output at %dx%d", sz.w, sz.h)
				}
			}
		})
	}
}

func TestMinimizeToggle(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic during minimize toggle: %v", r)
		}
	}()

	m := New(nil, nil, testCfg())
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	for _, entry := range testLogs() {
		m = updateModel(t, m, logsMsg{entry})
	}

	fullView := m.View()
	if !contains(fullView, "CodeBuddy CLI Gateway") {
		t.Fatal("full view missing header")
	}

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	minView := m.View()
	if !contains(minView, "CB Router") {
		t.Fatal("minimized view missing 'CB Router'")
	}

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	expandedView := m.View()
	if !contains(expandedView, "CodeBuddy CLI Gateway") {
		t.Fatal("expanded view missing header after toggle back")
	}
}

func TestTruncRunes(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"empty string", "", 10, ""},
		{"zero max", "hello", 0, ""},
		{"negative max", "hello", -1, ""},
		{"exact fit", "abc", 3, "abc"},
		{"no truncation needed", "hi", 10, "hi"},
		{"truncation with ellipsis", "abcdefgh", 5, "abcd…"},
		{"max 1", "abcdef", 1, "a"},
		{"max 2", "abcdef", 2, "ab"},
		{"max 3", "abcdef", 3, "abc"},
		{"unicode", "你好世界地球", 3, "你好世"},
		{"unicode truncation", "你好世界地球", 5, "你好世界…"},
		{"single rune", "x", 1, "x"},
		{"single rune max 3", "x", 3, "x"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncRunes(tc.s, tc.max)
			if got != tc.want {
				t.Errorf("truncRunes(%q, %d) = %q, want %q", tc.s, tc.max, got, tc.want)
			}
		})
	}
}

func TestNotReady(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic in not-ready view: %v", r)
		}
	}()

	m := New(nil, nil, testCfg())
	view := m.View()
	if view != "Loading..." {
		t.Errorf("expected 'Loading...' before ready, got %q", view)
	}
}

func TestClearLogs(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic in clear-logs test: %v", r)
		}
	}()

	m := New(nil, nil, testCfg())
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	for _, entry := range testLogs() {
		m = updateModel(t, m, logsMsg{entry})
	}

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})

	if m.total != 0 || m.success != 0 || m.failed != 0 {
		t.Errorf("after clear: total=%d success=%d failed=%d", m.total, m.success, m.failed)
	}
	if len(m.logs) != 0 {
		t.Errorf("after clear: %d logs remain", len(m.logs))
	}
}

// ─── Helpers ──────────────────────────────────────────────────

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOfSubstring(s, sub) >= 0
}

func indexOfSubstring(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
