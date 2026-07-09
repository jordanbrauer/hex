package repl

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// press sends a KeyMsg through Update and returns the new model.
func press(t *testing.T, m Model, key tea.KeyType, runes ...rune) Model {
	t.Helper()

	msg := tea.KeyMsg{Type: key}
	if len(runes) > 0 {
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: runes}
	}

	newModel, _ := m.Update(msg)

	return newModel.(Model)
}

func typeString(t *testing.T, m Model, s string) Model {
	t.Helper()

	for _, r := range s {
		m = press(t, m, tea.KeyRunes, r)
	}

	return m
}

func TestEvaluatorCalledOnEnter(t *testing.T) {
	var received string

	m := New(Options{
		Prompt: "> ",
		Evaluator: func(line string) Result {
			received = line

			return Result{Output: "ok"}
		},
	})

	m = typeString(t, m, "hello")
	m = press(t, m, tea.KeyEnter)

	if received != "hello" {
		t.Errorf("evaluator got %q, want %q", received, "hello")
	}

	if !strings.Contains(m.output.String(), "ok") {
		t.Errorf("output missing evaluator result: %s", m.output.String())
	}
}

func TestExitResultQuits(t *testing.T) {
	m := New(Options{
		Evaluator: func(line string) Result {
			return Result{Exit: true}
		},
	})

	m = typeString(t, m, "exit")

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	if !m.quit {
		t.Error("model.quit should be true after exit result")
	}

	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestCtrlDQuits(t *testing.T) {
	m := New(Options{
		Evaluator: func(line string) Result { return Result{} },
	})

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = newModel.(Model)

	if !m.quit {
		t.Error("Ctrl+D should set quit")
	}

	if cmd == nil {
		t.Error("Ctrl+D should return tea.Quit")
	}
}

func TestHistoryUpDown(t *testing.T) {
	m := New(Options{
		Evaluator: func(line string) Result { return Result{} },
	})

	// Submit three lines
	for _, line := range []string{"one", "two", "three"} {
		m = typeString(t, m, line)
		m = press(t, m, tea.KeyEnter)
	}

	if got := len(m.history); got != 3 {
		t.Fatalf("history len=%d, want 3", got)
	}

	// Type something live, then navigate up
	m = typeString(t, m, "live")
	m = press(t, m, tea.KeyUp)

	if got := m.input.Value(); got != "three" {
		t.Errorf("Up: got %q, want three", got)
	}

	m = press(t, m, tea.KeyUp)
	if got := m.input.Value(); got != "two" {
		t.Errorf("Up*2: got %q, want two", got)
	}

	m = press(t, m, tea.KeyDown)
	if got := m.input.Value(); got != "three" {
		t.Errorf("Down: got %q, want three", got)
	}

	m = press(t, m, tea.KeyDown)
	if got := m.input.Value(); got != "live" {
		t.Errorf("Down back to live: got %q", got)
	}
}

func TestBannerRenders(t *testing.T) {
	m := New(Options{
		Banner:    "welcome",
		Evaluator: func(line string) Result { return Result{} },
	})

	if !strings.Contains(m.View(), "welcome") {
		t.Errorf("banner not in initial view:\n%s", m.View())
	}
}

func TestHistoryLimit(t *testing.T) {
	m := New(Options{
		HistoryLimit: 2,
		Evaluator:    func(line string) Result { return Result{} },
	})

	for _, line := range []string{"a", "b", "c"} {
		m = typeString(t, m, line)
		m = press(t, m, tea.KeyEnter)
	}

	if got := len(m.history); got != 2 {
		t.Errorf("expected trimmed history len=2, got %d", got)
	}

	if m.history[0] != "b" || m.history[1] != "c" {
		t.Errorf("oldest entry not evicted: %v", m.history)
	}
}

func TestErrorResultRenders(t *testing.T) {
	m := New(Options{
		Evaluator: func(line string) Result {
			return Result{Err: "boom"}
		},
	})

	m = typeString(t, m, "bad")
	m = press(t, m, tea.KeyEnter)

	if !strings.Contains(m.output.String(), "boom") {
		t.Errorf("error not rendered: %s", m.output.String())
	}
}
