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

	if got := m.Submissions(); len(got) != 1 || got[0] != "hello" {
		t.Errorf("submissions = %v, want [hello]", got)
	}

	joined := strings.Join(m.Echoes(), "\n")
	if !strings.Contains(joined, "ok") {
		t.Errorf("echoes missing evaluator output:\n%s", joined)
	}
}

func TestEmptyLineNotSubmitted(t *testing.T) {
	m := New(Options{
		Evaluator: func(line string) Result {
			t.Errorf("evaluator called with empty line: %q", line)

			return Result{}
		},
	})

	m = press(t, m, tea.KeyEnter)

	if len(m.Submissions()) != 0 {
		t.Errorf("empty line should not be submitted")
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
		t.Error("expected tea.Cmd (batch with tea.Quit)")
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

	for _, line := range []string{"one", "two", "three"} {
		m = typeString(t, m, line)
		m = press(t, m, tea.KeyEnter)
	}

	if got := len(m.history); got != 3 {
		t.Fatalf("history len=%d, want 3", got)
	}

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

func TestBannerEmittedOnFirstUpdate(t *testing.T) {
	m := New(Options{
		Banner:    "welcome",
		Evaluator: func(line string) Result { return Result{} },
	})

	// Trigger any Update — even a no-op key.
	m = press(t, m, tea.KeyRunes, 'x')

	echoes := strings.Join(m.Echoes(), "\n")
	if !strings.Contains(echoes, "welcome") {
		t.Errorf("banner not emitted:\n%s", echoes)
	}
}

func TestBannerEmittedOnce(t *testing.T) {
	m := New(Options{
		Banner:    "welcome",
		Evaluator: func(line string) Result { return Result{} },
	})

	m = press(t, m, tea.KeyRunes, 'x')
	firstEchoes := len(m.Echoes())

	m = press(t, m, tea.KeyRunes, 'y')
	if len(m.Echoes()) != firstEchoes {
		t.Errorf("banner should not repeat: echoes grew %d -> %d", firstEchoes, len(m.Echoes()))
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

	echoes := strings.Join(m.Echoes(), "\n")
	if !strings.Contains(echoes, "boom") {
		t.Errorf("error not echoed:\n%s", echoes)
	}
}

func TestViewIsOnlyPromptLine(t *testing.T) {
	// The whole point of the tea.Println-driven design: View() only
	// contains the current prompt line, so terminal scrollback stays
	// untouched.
	m := New(Options{
		Prompt:    "> ",
		Banner:    "welcome",
		Evaluator: func(line string) Result { return Result{Output: "ok"} },
	})

	m = typeString(t, m, "hi")
	m = press(t, m, tea.KeyEnter)

	view := m.View()
	if strings.Contains(view, "welcome") {
		t.Errorf("View should NOT contain banner; that goes to scrollback via tea.Println. View:\n%s", view)
	}

	if strings.Contains(view, "ok") {
		t.Errorf("View should NOT contain evaluator output. View:\n%s", view)
	}

	if !strings.Contains(view, "> ") {
		t.Errorf("View missing prompt:\n%s", view)
	}
}
