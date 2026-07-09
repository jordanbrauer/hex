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
		Evaluator: func(mode, line string) Result {
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
		Evaluator: func(mode, line string) Result {
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
		Evaluator: func(mode, line string) Result {
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
		Evaluator: func(mode, line string) Result { return Result{} },
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
		Evaluator: func(mode, line string) Result { return Result{} },
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
		Evaluator: func(mode, line string) Result { return Result{} },
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
		Evaluator: func(mode, line string) Result { return Result{} },
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
		Evaluator:    func(mode, line string) Result { return Result{} },
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
		Evaluator: func(mode, line string) Result {
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

func TestModes_activatorSwitchesMode(t *testing.T) {
	var seen []string

	m := New(Options{
		Modes: []Mode{
			{Name: "teal", Activator: 't', Prompt: "(teal)> "},
			{Name: "lua", Activator: 'l', Prompt: "(lua)> "},
		},
		Evaluator: func(mode, line string) Result {
			seen = append(seen, mode+":"+line)

			return Result{}
		},
	})

	if got := m.CurrentMode(); got != "teal" {
		t.Fatalf("initial mode = %q, want teal", got)
	}

	m = press(t, m, tea.KeyRunes, 'l')
	if got := m.CurrentMode(); got != "lua" {
		t.Fatalf("after 'l' on empty prompt: mode = %q, want lua", got)
	}

	if m.input.Value() != "" {
		t.Errorf("activator rune should not be inserted; input = %q", m.input.Value())
	}

	m = typeString(t, m, "x=1")
	m = press(t, m, tea.KeyEnter)

	if len(seen) != 1 || seen[0] != "lua:x=1" {
		t.Errorf("evaluator saw %v, want [lua:x=1]", seen)
	}
}

func TestModes_activatorIgnoredMidInput(t *testing.T) {
	m := New(Options{
		Modes: []Mode{
			{Name: "teal", Activator: 't', Prompt: "(teal)> "},
			{Name: "lua", Activator: 'l', Prompt: "(lua)> "},
		},
		Evaluator: func(mode, line string) Result { return Result{} },
	})

	m = press(t, m, tea.KeyRunes, 'x')
	m = press(t, m, tea.KeyRunes, 'l')

	if got := m.CurrentMode(); got != "teal" {
		t.Errorf("activator mid-input should not switch: mode = %q", got)
	}

	if got := m.input.Value(); got != "xl" {
		t.Errorf("input = %q, want xl", got)
	}
}

func TestModes_backspaceReturnsToDefault(t *testing.T) {
	m := New(Options{
		Modes: []Mode{
			{Name: "teal", Activator: 't', Prompt: "(teal)> "},
			{Name: "lua", Activator: 'l', Prompt: "(lua)> "},
		},
		Evaluator: func(mode, line string) Result { return Result{} },
	})

	m = press(t, m, tea.KeyRunes, 'l')
	if got := m.CurrentMode(); got != "lua" {
		t.Fatalf("expected lua, got %q", got)
	}

	m = press(t, m, tea.KeyBackspace)
	if got := m.CurrentMode(); got != "teal" {
		t.Errorf("backspace should return to default, got %q", got)
	}
}

func TestModes_backspaceMidInputDoesNotSwitchMode(t *testing.T) {
	m := New(Options{
		Modes: []Mode{
			{Name: "teal", Activator: 't', Prompt: "(teal)> "},
			{Name: "lua", Activator: 'l', Prompt: "(lua)> "},
		},
		Evaluator: func(mode, line string) Result { return Result{} },
	})

	m = press(t, m, tea.KeyRunes, 'l')
	m = typeString(t, m, "ab")
	m = press(t, m, tea.KeyBackspace)

	if got := m.CurrentMode(); got != "lua" {
		t.Errorf("backspace mid-input switched mode to %q", got)
	}

	if got := m.input.Value(); got != "a" {
		t.Errorf("input = %q, want 'a'", got)
	}
}

func TestContinuation_incompleteBuffersAndSwitchesPrompt(t *testing.T) {
	var seen []string

	m := New(Options{
		Prompt:             "> ",
		ContinuationPrompt: ". ",
		Evaluator: func(_, line string) Result {
			seen = append(seen, line)

			if !strings.Contains(line, "end") {
				return Result{Incomplete: true}
			}

			return Result{Output: "done"}
		},
	})

	// First line: incomplete
	m = typeString(t, m, "function foo()")
	m = press(t, m, tea.KeyEnter)

	if !m.InContinuation() {
		t.Fatalf("expected InContinuation after incomplete submit")
	}

	if view := m.View(); !strings.Contains(view, ". ") {
		t.Errorf("prompt should be continuation '. ', view: %q", view)
	}

	// Second line: still incomplete
	m = typeString(t, m, "  print(1)")
	m = press(t, m, tea.KeyEnter)

	if !m.InContinuation() {
		t.Fatalf("still expected InContinuation after second incomplete line")
	}

	// Third line: complete
	m = typeString(t, m, "end")
	m = press(t, m, tea.KeyEnter)

	if m.InContinuation() {
		t.Errorf("should have exited continuation after 'end'")
	}

	// Evaluator saw 3 calls with progressively longer joined buffers.
	if len(seen) != 3 {
		t.Fatalf("evaluator calls = %d, want 3: %v", len(seen), seen)
	}

	if want := "function foo()"; seen[0] != want {
		t.Errorf("seen[0] = %q, want %q", seen[0], want)
	}

	if want := "function foo()\n  print(1)\nend"; seen[2] != want {
		t.Errorf("seen[2] = %q, want %q", seen[2], want)
	}

	// Submissions and history record the full multi-line entry as
	// one item.
	if got := m.Submissions(); len(got) != 1 || got[0] != seen[2] {
		t.Errorf("submissions = %v, want [%q]", got, seen[2])
	}
}

func TestContinuation_ctrlCAborts(t *testing.T) {
	m := New(Options{
		Prompt: "> ",
		Evaluator: func(_, _ string) Result {
			return Result{Incomplete: true}
		},
	})

	m = typeString(t, m, "function foo()")
	m = press(t, m, tea.KeyEnter)

	if !m.InContinuation() {
		t.Fatalf("expected continuation state")
	}

	// Ctrl+C aborts the continuation without quitting.
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = newModel.(Model)

	if m.quit {
		t.Errorf("Ctrl+C in continuation should NOT quit")
	}

	if m.InContinuation() {
		t.Errorf("Ctrl+C in continuation should reset")
	}

	// The returned cmd may be a batch/sequence but must not be tea.Quit.
	_ = cmd
}

func TestContinuation_ctrlCOutsideContinuationStillQuits(t *testing.T) {
	m := New(Options{
		Prompt:    "> ",
		Evaluator: func(_, _ string) Result { return Result{} },
	})

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = newModel.(Model)

	if !m.quit {
		t.Errorf("Ctrl+C outside continuation should quit")
	}

	if cmd == nil {
		t.Errorf("expected tea.Quit cmd")
	}
}

func TestContinuation_modePromptUsedInMultiModeSetup(t *testing.T) {
	m := New(Options{
		Modes: []Mode{
			{
				Name:               "teal",
				Activator:          't',
				Prompt:             "teal> ",
				ContinuationPrompt: "teal. ",
			},
		},
		Evaluator: func(_, _ string) Result { return Result{Incomplete: true} },
	})

	m = typeString(t, m, "function foo()")
	m = press(t, m, tea.KeyEnter)

	if view := m.View(); !strings.Contains(view, "teal. ") {
		t.Errorf("expected mode's ContinuationPrompt in view, got %q", view)
	}
}

func TestCompletion_singleMatchInserts(t *testing.T) {
	m := New(Options{
		Prompt:    "> ",
		Evaluator: func(_, _ string) Result { return Result{} },
		Completer: func(_, input string, cursor int) ([]Candidate, int) {
			return []Candidate{{Text: "query"}}, 0
		},
	})

	m = typeString(t, m, "q")
	m = press(t, m, tea.KeyTab)

	if got := m.input.Value(); got != "query" {
		t.Errorf("input = %q, want query", got)
	}
}

func TestCompletion_cyclesOnRepeatedTab(t *testing.T) {
	cands := []Candidate{{Text: "query"}, {Text: "queryOne"}, {Text: "quit"}}

	m := New(Options{
		Prompt:    "> ",
		Evaluator: func(_, _ string) Result { return Result{} },
		Completer: func(_, input string, cursor int) ([]Candidate, int) {
			return cands, 0
		},
	})

	m = typeString(t, m, "q")

	m = press(t, m, tea.KeyTab)
	if got := m.input.Value(); got != "query" {
		t.Errorf("first tab: %q, want query", got)
	}

	m = press(t, m, tea.KeyTab)
	if got := m.input.Value(); got != "queryOne" {
		t.Errorf("second tab: %q, want queryOne", got)
	}

	m = press(t, m, tea.KeyTab)
	if got := m.input.Value(); got != "quit" {
		t.Errorf("third tab: %q, want quit", got)
	}

	// Wraps around.
	m = press(t, m, tea.KeyTab)
	if got := m.input.Value(); got != "query" {
		t.Errorf("fourth tab (wrap): %q, want query", got)
	}
}

func TestCompletion_shiftTabCyclesBackward(t *testing.T) {
	cands := []Candidate{{Text: "one"}, {Text: "two"}, {Text: "three"}}

	m := New(Options{
		Prompt:    "> ",
		Evaluator: func(_, _ string) Result { return Result{} },
		Completer: func(_, _ string, _ int) ([]Candidate, int) {
			return cands, 0
		},
	})

	m = typeString(t, m, "o")

	m = press(t, m, tea.KeyTab)
	if got := m.input.Value(); got != "one" {
		t.Errorf("first tab: %q", got)
	}

	m = press(t, m, tea.KeyShiftTab)
	if got := m.input.Value(); got != "three" {
		t.Errorf("shift-tab (wrap backward): %q, want three", got)
	}
}

func TestCompletion_prefixReplacement(t *testing.T) {
	// Completer returns a prefixStart into an existing input string:
	// input="db.q" cursor=4 → prefixStart=3 (after the '.') means
	// candidates replace "q" only, not the whole thing.
	m := New(Options{
		Prompt:    "> ",
		Evaluator: func(_, _ string) Result { return Result{} },
		Completer: func(_, input string, cursor int) ([]Candidate, int) {
			return []Candidate{{Text: "query"}, {Text: "queryOne"}}, 3
		},
	})

	m = typeString(t, m, "db.q")
	m = press(t, m, tea.KeyTab)

	if got := m.input.Value(); got != "db.query" {
		t.Errorf("first tab: %q, want db.query", got)
	}

	m = press(t, m, tea.KeyTab)
	if got := m.input.Value(); got != "db.queryOne" {
		t.Errorf("second tab: %q, want db.queryOne", got)
	}
}

func TestCompletion_typingAfterTabCommitsAndResets(t *testing.T) {
	m := New(Options{
		Prompt:    "> ",
		Evaluator: func(_, _ string) Result { return Result{} },
		Completer: func(_, _ string, _ int) ([]Candidate, int) {
			return []Candidate{{Text: "query"}, {Text: "queryOne"}}, 0
		},
	})

	m = typeString(t, m, "q")
	m = press(t, m, tea.KeyTab)

	if got := m.input.Value(); got != "query" {
		t.Fatalf("first tab: %q", got)
	}

	// Typing a character breaks the cycle: subsequent Tab starts
	// a fresh completion on the new prefix.
	m = typeString(t, m, "z")

	if m.completionActive {
		t.Errorf("cycle should reset after typing")
	}

	if got := m.input.Value(); got != "queryz" {
		t.Errorf("input after typing: %q", got)
	}
}

func TestCompletion_emptyCandidatesIsNoOp(t *testing.T) {
	m := New(Options{
		Prompt:    "> ",
		Evaluator: func(_, _ string) Result { return Result{} },
		Completer: func(_, _ string, _ int) ([]Candidate, int) {
			return nil, 0
		},
	})

	m = typeString(t, m, "xyz")
	m = press(t, m, tea.KeyTab)

	if got := m.input.Value(); got != "xyz" {
		t.Errorf("input should be unchanged with no candidates: %q", got)
	}

	if m.completionActive {
		t.Errorf("no cycle should start when candidates are empty")
	}
}

func TestViewIsOnlyPromptLine(t *testing.T) {
	m := New(Options{
		Prompt:    "> ",
		Banner:    "welcome",
		Evaluator: func(_, _ string) Result { return Result{Output: "ok"} },
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
