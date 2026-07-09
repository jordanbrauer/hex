// Package repl is a Bubble Tea REPL component: a prompt line with
// arrow-key editing, up/down command history, styled output, and a
// caller-supplied evaluator callback.
//
// The component renders ONLY the input line via View(). Everything
// else — banner, echoed input, evaluator output, errors — is pushed
// into the terminal's scrollback via tea.Println. This matches how
// native REPLs (psql, iex, python) behave: the prompt stays at the
// bottom, output flows above it, and the terminal handles scroll,
// selection, and history natively.
//
//	m := repl.New(repl.Options{
//	    Prompt:    "myapp(teal)> ",
//	    Banner:    "hex repl — teal mode. Ctrl+D or \"exit\" to quit.",
//	    Evaluator: func(line string) repl.Result {
//	        return repl.Result{Output: "hi"}
//	    },
//	})
//
//	if _, err := tea.NewProgram(m).Run(); err != nil { ... }
//
// The evaluator runs synchronously inside Update, so slow operations
// block the render loop — acceptable for the REPL case where users
// wait on each line anyway.
package repl

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Result is what the evaluator returns for one submitted line.
type Result struct {
	// Output is normal (stdout-like) content, rendered in the
	// default style.
	Output string

	// Err is error-like content, rendered in the error style
	// (a subdued red by default).
	Err string

	// Exit requests the REPL loop to terminate cleanly.
	Exit bool

	// Incomplete signals that the submitted input is syntactically
	// incomplete — e.g. an unclosed function block or table literal.
	// The REPL switches to a continuation prompt and buffers the
	// user's next line onto the current one instead of evaluating.
	// Output and Err are ignored when Incomplete is true.
	Incomplete bool
}

// Evaluator is the callback invoked for every submitted line. Mode
// carries the name of the active Mode at submission time, letting
// callers dispatch to different runtimes (e.g. Teal vs Lua) without
// juggling closures per mode.
type Evaluator func(mode, line string) Result

// Mode describes a REPL mode — a language or command surface the
// user can switch to. Julia's REPL popularised this pattern with
// `?` (help), `;` (shell), `]` (pkg); we generalise it: any rune
// pressed at an empty prompt swaps modes, and backspace at an
// empty prompt in a non-default mode reverts to the default.
type Mode struct {
	// Name is what gets passed to the Evaluator. Required and
	// unique within Options.Modes.
	Name string

	// Activator is the rune that switches TO this mode when typed
	// on an empty prompt. Zero means the mode is not
	// user-selectable (e.g. a default that only reachable via
	// backspace).
	Activator rune

	// Prompt is the string rendered before the input line while
	// this mode is active, e.g. "myapp(teal)> ".
	Prompt string

	// ContinuationPrompt is rendered when the previous submission
	// returned Result.Incomplete, so the user sees that their input
	// is being buffered. When empty, defaults to Prompt — typically
	// callers pass a same-width variant that swaps "> " for ". "
	// or "... " so column alignment is preserved.
	ContinuationPrompt string

	// PromptColor, when non-nil, overrides the base Styles.Prompt's
	// foreground for this mode. Any nil defers to the base style.
	PromptColor lipgloss.TerminalColor
}

// Options configures a new Model.
type Options struct {
	// Prompt is the string shown before the input line when Modes
	// is empty (single-mode REPL). Defaults to "> ". Ignored when
	// Modes is set — each mode carries its own prompt.
	Prompt string

	// ContinuationPrompt is used in single-mode configurations when
	// the previous submission returned Result.Incomplete. Defaults
	// to Prompt (so continuation is visually indistinguishable
	// unless the caller sets this to something like "... ").
	ContinuationPrompt string

	// Banner is optional multi-line text printed above the first
	// prompt. Newlines are respected.
	Banner string

	// Evaluator is called for each submitted line. Required.
	Evaluator Evaluator

	// Modes are the switchable modes. The first mode is the
	// default; users switch to another mode by typing its
	// Activator rune on an empty prompt, and return to the default
	// by pressing Backspace on an empty prompt.
	//
	// Empty means single-mode: the evaluator sees mode="" and
	// Prompt/Banner behave as before.
	Modes []Mode

	// HistoryLimit caps the in-memory command history. 0 or
	// negative means unlimited.
	HistoryLimit int
}

// Model is the bubble tea model for the REPL.
type Model struct {
	input       textinput.Model
	history     []string
	historyIdx  int // -1 = editing a fresh line; else index into history
	liveInput   string
	banner      string
	bannerShown bool
	evaluate    Evaluator
	limit       int
	quit        bool

	// Mode state
	modes       []Mode
	modeIdx     int          // index into modes; 0 is default
	activators  map[rune]int // rune → mode index for quick lookup
	singleMode  bool         // true when Options.Modes was empty (legacy single-mode REPL)
	fixedPrompt string       // used when singleMode

	// Continuation state — buffer of previously-submitted lines
	// whose join was not yet syntactically complete. Populated when
	// the evaluator returns Incomplete; drained (with the newest
	// line appended) on the next Enter and passed to the evaluator
	// as a single joined string.
	inContinuation     bool
	continuationBuffer []string
	fixedContinuation  string // used when singleMode

	// submissions records every line submitted to the evaluator
	// (empty lines and exit directives excluded). Used by tests to
	// assert what the user typed without inspecting tea.Cmds.
	submissions []string
	// echoes records every line rendered to the terminal via
	// tea.Println, including the echo of the submitted input,
	// evaluator Output, and evaluator Err. Used by tests.
	echoes []string
	styles Styles
}

// Styles groups the lipgloss styles the component uses.
type Styles struct {
	Prompt lipgloss.Style
	Input  lipgloss.Style
	Output lipgloss.Style
	Error  lipgloss.Style
	Banner lipgloss.Style
}

// DefaultStyles returns hex's default palette, tuned for readability
// against both light and dark terminals via AdaptiveColor.
//
// Prompt: a calm cyan-blue (bold), matching the charm accent line.
// Error:  a muted red (not the alarming bright red).
// Banner: dim gray.
func DefaultStyles() Styles {
	return Styles{
		Prompt: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3e8b9b")).
			Bold(true),
		Input: lipgloss.NewStyle(),
		Output: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#DDDDDD"}),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#B00020", Dark: "#FF6B6B"}),
		Banner: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}),
	}
}

// New builds a Model.
func New(opts Options) Model {
	ti := textinput.New()
	ti.Prompt = "" // we render the prompt in View for consistent styling
	ti.Focus()
	ti.CharLimit = 0

	m := Model{
		input:      ti,
		history:    nil,
		historyIdx: -1,
		banner:     opts.Banner,
		evaluate:   opts.Evaluator,
		limit:      opts.HistoryLimit,
		styles:     DefaultStyles(),
		modes:      opts.Modes,
		modeIdx:    0,
		activators: map[rune]int{},
		singleMode: len(opts.Modes) == 0,
	}

	if m.singleMode {
		m.fixedPrompt = opts.Prompt
		if m.fixedPrompt == "" {
			m.fixedPrompt = "> "
		}

		m.fixedContinuation = opts.ContinuationPrompt
		if m.fixedContinuation == "" {
			m.fixedContinuation = m.fixedPrompt
		}
	} else {
		for i, mode := range opts.Modes {
			if mode.Activator != 0 {
				m.activators[mode.Activator] = i
			}
		}
	}

	return m
}

// Init satisfies tea.Model. Emits the banner (if any) into scrollback
// and starts the cursor blink.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages: key presses (submit, history navigation,
// exit) and forwards the rest to the textinput. Never mutates fields
// via pointer-only builders — everything is copy-safe.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Emit the banner on the first Update, once we know the program
	// is running. Doing this here (rather than Init) means the tests
	// see the banner in Echoes() without needing to Init the model.
	//
	// Each entry in initCmds must be sequenced with the response
	// cmds so scrollback order is deterministic — tea.Batch runs
	// its Cmds concurrently, which reorders the tea.Println
	// messages arbitrarily.
	var initCmds []tea.Cmd

	if !m.bannerShown && m.banner != "" {
		rendered := m.styles.Banner.Render(m.banner)
		m.echoes = append(m.echoes, rendered)
		initCmds = append(initCmds, tea.Println(rendered))
		m.bannerShown = true
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			// Ctrl+C in continuation mode aborts the pending buffer
			// (Python REPL convention). Outside continuation it
			// quits, matching the previous behaviour.
			if m.inContinuation {
				m.inContinuation = false
				m.continuationBuffer = nil
				m.input.SetValue("")

				return m, tea.Sequence(initCmds...)
			}

			m.quit = true

			return m, tea.Quit

		case tea.KeyCtrlD:
			m.quit = true

			return m, tea.Quit

		case tea.KeyEnter:
			line := m.input.Value()
			m.input.SetValue("")
			m.historyIdx = -1
			m.liveInput = ""

			echo := m.renderPrompt() + m.styles.Input.Render(line)
			m.echoes = append(m.echoes, echo)

			cmds := append(initCmds, tea.Println(echo))

			// Empty submits: in continuation mode, treat as "finish
			// input as-is" (evaluate whatever's buffered). Outside
			// continuation, an empty line is just a reprompt.
			if strings.TrimSpace(line) == "" && !m.inContinuation {
				return m, tea.Sequence(cmds...)
			}

			m.continuationBuffer = append(m.continuationBuffer, line)
			full := strings.Join(m.continuationBuffer, "\n")

			result := m.evaluate(m.currentModeName(), full)

			if result.Incomplete {
				m.inContinuation = true

				return m, tea.Sequence(cmds...)
			}

			// Complete: commit the input to history as a single
			// (possibly multi-line) entry and clear the buffer.
			m.pushHistory(full)
			m.submissions = append(m.submissions, full)
			m.continuationBuffer = nil
			m.inContinuation = false

			if result.Output != "" {
				out := m.styles.Output.Render(result.Output)
				m.echoes = append(m.echoes, out)
				cmds = append(cmds, tea.Println(out))
			}

			if result.Err != "" {
				errText := m.styles.Error.Render(result.Err)
				m.echoes = append(m.echoes, errText)
				cmds = append(cmds, tea.Println(errText))
			}

			if result.Exit {
				m.quit = true
				cmds = append(cmds, tea.Quit)
			}

			return m, tea.Sequence(cmds...)

		case tea.KeyUp:
			m.navigateHistory(-1)

			return m, tea.Sequence(initCmds...)

		case tea.KeyDown:
			m.navigateHistory(1)

			return m, tea.Sequence(initCmds...)

		case tea.KeyBackspace:
			// Backspace on an empty prompt in a non-default mode
			// returns to the default mode (Julia REPL convention).
			// Any other backspace falls through to textinput.
			if !m.singleMode && m.modeIdx != 0 && m.input.Value() == "" {
				m.modeIdx = 0

				return m, tea.Sequence(initCmds...)
			}

		case tea.KeyRunes:
			// Julia-style mode activation: a single activator rune
			// typed on an empty prompt switches modes instead of
			// inserting the rune. Multi-rune inputs (paste, IME)
			// fall through to textinput normally.
			if !m.singleMode && m.input.Value() == "" && len(msg.Runes) == 1 {
				if idx, ok := m.activators[msg.Runes[0]]; ok && idx != m.modeIdx {
					m.modeIdx = idx

					return m, tea.Sequence(initCmds...)
				}
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if len(initCmds) > 0 {
		return m, tea.Sequence(append(initCmds, cmd)...)
	}

	return m, cmd
}

// View renders ONLY the prompt + input line. All output — banner,
// echoed submissions, evaluator results — goes into the terminal's
// scrollback via tea.Println, so scroll wheel, selection, and shell
// history all continue to work as the user expects.
func (m Model) View() string {
	return m.renderPrompt() + m.input.View()
}

// renderPrompt returns the styled prompt for the current mode /
// continuation state. Handles both single-mode (Options.Prompt) and
// multi-mode (Options.Modes[modeIdx]) configurations.
func (m Model) renderPrompt() string {
	if m.singleMode {
		text := m.fixedPrompt
		if m.inContinuation {
			text = m.fixedContinuation
		}

		return m.styles.Prompt.Render(text)
	}

	mode := m.modes[m.modeIdx]

	style := m.styles.Prompt
	if mode.PromptColor != nil {
		style = style.Foreground(mode.PromptColor)
	}

	text := mode.Prompt
	if m.inContinuation && mode.ContinuationPrompt != "" {
		text = mode.ContinuationPrompt
	}

	return style.Render(text)
}

// currentModeName returns the active mode's Name, or "" for a
// single-mode REPL.
func (m Model) currentModeName() string {
	if m.singleMode {
		return ""
	}

	return m.modes[m.modeIdx].Name
}

// CurrentMode returns the active mode's Name. Useful for tests and
// callers that want to observe or persist the mode across sessions.
func (m Model) CurrentMode() string { return m.currentModeName() }

// InContinuation reports whether the REPL is currently waiting for
// the user to finish a multi-line input.
func (m Model) InContinuation() bool { return m.inContinuation }

// pushHistory records line in the ring buffer, trimming the oldest
// entry when HistoryLimit is exceeded.
func (m *Model) pushHistory(line string) {
	m.history = append(m.history, line)

	if m.limit > 0 && len(m.history) > m.limit {
		m.history = m.history[len(m.history)-m.limit:]
	}
}

// navigateHistory moves through the history buffer. delta is -1 for
// Up (older) or +1 for Down (newer).
func (m *Model) navigateHistory(delta int) {
	if len(m.history) == 0 {
		return
	}

	if m.historyIdx == -1 && delta < 0 {
		m.liveInput = m.input.Value()
		m.historyIdx = len(m.history) - 1
		m.input.SetValue(m.history[m.historyIdx])
		m.input.CursorEnd()

		return
	}

	next := m.historyIdx + delta

	if next < 0 {
		return
	}

	if next >= len(m.history) {
		m.historyIdx = -1
		m.input.SetValue(m.liveInput)
		m.input.CursorEnd()

		return
	}

	m.historyIdx = next
	m.input.SetValue(m.history[m.historyIdx])
	m.input.CursorEnd()
}

// Styles returns the current style set for read-only inspection.
func (m Model) Styles() Styles { return m.styles }

// SetStyles replaces the style set. Return the modified model —
// Bubble Tea models are value types.
func (m Model) SetStyles(s Styles) Model {
	m.styles = s

	return m
}

// History returns a copy of the current in-memory command history.
// Useful for persisting between sessions.
func (m Model) History() []string {
	out := make([]string, len(m.history))
	copy(out, m.history)

	return out
}

// SetHistory replaces the in-memory history. Useful for restoring a
// previous session's log on startup.
func (m Model) SetHistory(h []string) Model {
	m.history = append([]string(nil), h...)

	return m
}

// Submissions returns lines that were submitted to the evaluator
// (empty lines excluded). Used by tests to assert behaviour without
// intercepting tea.Cmd streams.
func (m Model) Submissions() []string {
	out := make([]string, len(m.submissions))
	copy(out, m.submissions)

	return out
}

// Echoes returns every line the model would push into the terminal
// via tea.Println: banner, echoed prompts+input, evaluator output,
// evaluator errors — in order. Used by tests.
func (m Model) Echoes() []string {
	out := make([]string, len(m.echoes))
	copy(out, m.echoes)

	return out
}
