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
}

// Evaluator is the callback invoked for every submitted line.
type Evaluator func(line string) Result

// Options configures a new Model.
type Options struct {
	// Prompt is the string shown before the input on each line.
	// Defaults to "> ".
	Prompt string

	// Banner is optional multi-line text printed above the first
	// prompt. Newlines are respected.
	Banner string

	// Evaluator is called for each submitted line. Required.
	Evaluator Evaluator

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
	prompt      string
	banner      string
	bannerShown bool
	evaluate    Evaluator
	limit       int
	quit        bool
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
			Foreground(lipgloss.AdaptiveColor{Light: "#0068AA", Dark: "#5AB0FF"}).
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
	prompt := opts.Prompt
	if prompt == "" {
		prompt = "> "
	}

	ti := textinput.New()
	ti.Prompt = "" // we render the prompt in View for consistent styling
	ti.Focus()
	ti.CharLimit = 0

	return Model{
		input:      ti,
		history:    nil,
		historyIdx: -1,
		prompt:     prompt,
		banner:     opts.Banner,
		evaluate:   opts.Evaluator,
		limit:      opts.HistoryLimit,
		styles:     DefaultStyles(),
	}
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
		case tea.KeyCtrlC, tea.KeyCtrlD:
			m.quit = true

			return m, tea.Quit

		case tea.KeyEnter:
			line := m.input.Value()
			m.input.SetValue("")
			m.historyIdx = -1
			m.liveInput = ""

			echo := m.styles.Prompt.Render(m.prompt) + m.styles.Input.Render(line)
			m.echoes = append(m.echoes, echo)

			cmds := append(initCmds, tea.Println(echo))

			if strings.TrimSpace(line) == "" {
				return m, tea.Batch(cmds...)
			}

			m.pushHistory(line)
			m.submissions = append(m.submissions, line)

			result := m.evaluate(line)

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

			return m, tea.Batch(cmds...)

		case tea.KeyUp:
			m.navigateHistory(-1)

			return m, tea.Batch(initCmds...)

		case tea.KeyDown:
			m.navigateHistory(1)

			return m, tea.Batch(initCmds...)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if len(initCmds) > 0 {
		return m, tea.Batch(append(initCmds, cmd)...)
	}

	return m, cmd
}

// View renders ONLY the prompt + input line. All output — banner,
// echoed submissions, evaluator results — goes into the terminal's
// scrollback via tea.Println, so scroll wheel, selection, and shell
// history all continue to work as the user expects.
func (m Model) View() string {
	return m.styles.Prompt.Render(m.prompt) + m.input.View()
}

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
