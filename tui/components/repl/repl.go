// Package repl is a Bubble Tea REPL component: a prompt line with
// arrow-key editing, up/down command history, a scrollable output
// viewport, and a caller-supplied evaluator callback.
//
// It is intentionally generic — hex/lua/repl uses it to drive Teal/Lua
// evaluation, but any caller with a `func(string) Result` can plug
// in their own language or shell-like tool.
//
//	m := repl.New(repl.Options{
//	    Prompt:    "myapp(teal)> ",
//	    Banner:    "hex repl — teal mode. Ctrl+D or \"exit\" to quit.",
//	    Evaluator: func(line string) repl.Result {
//	        out, err := evaluateSomehow(line)
//	        return repl.Result{Output: out, Err: err}
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
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Result is what the evaluator returns for one submitted line.
type Result struct {
	// Output is normal (stdout-like) content, rendered in the
	// default style.
	Output string

	// Err is error-like content, rendered in the error style
	// (dim red by default).
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

	// Banner is optional multi-line text shown above the first
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
	input      textinput.Model
	viewport   viewport.Model
	history    []string
	historyIdx int // -1 = editing a fresh line; else index into history
	liveInput  string
	output     strings.Builder
	prompt     string
	banner     string
	evaluate   Evaluator
	limit      int
	width      int
	height     int
	quit       bool

	styles Styles
}

// Styles groups the lipgloss styles the component uses. Overwrite
// fields on the returned Model.Styles to customise.
type Styles struct {
	Prompt lipgloss.Style
	Input  lipgloss.Style
	Output lipgloss.Style
	Error  lipgloss.Style
	Banner lipgloss.Style
}

// DefaultStyles returns the hex-default color palette. Prompt uses
// bright cyan, error uses dim red, banner is faint.
func DefaultStyles() Styles {
	return Styles{
		Prompt: lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true),
		Input:  lipgloss.NewStyle(),
		Output: lipgloss.NewStyle(),
		Error:  lipgloss.NewStyle().Foreground(lipgloss.Color("204")),
		Banner: lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
	}
}

// New builds a Model.
func New(opts Options) Model {
	prompt := opts.Prompt
	if prompt == "" {
		prompt = "> "
	}

	ti := textinput.New()
	ti.Prompt = "" // we render prompt in View for consistent styling
	ti.Focus()
	ti.CharLimit = 0

	vp := viewport.New(80, 20)

	m := Model{
		input:      ti,
		viewport:   vp,
		history:    nil,
		historyIdx: -1,
		prompt:     prompt,
		banner:     opts.Banner,
		evaluate:   opts.Evaluator,
		limit:      opts.HistoryLimit,
		styles:     DefaultStyles(),
	}

	// Seed the viewport with the banner so it's visible even before
	// the first WindowSizeMsg arrives.
	m.viewport.SetContent(m.renderHistory())

	return m
}

// Init satisfies tea.Model. No initial commands needed — the
// textinput is already focused.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages: window resize, key presses (submit,
// history navigation, exit), and forwards the rest to the textinput.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Reserve one line for the input at the bottom.
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 1

		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

		return m, nil

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

			// Echo the submitted line into the output as
			// "<prompt><line>", styled.
			m.appendLine(m.styles.Prompt.Render(m.prompt) + m.styles.Input.Render(line))

			if strings.TrimSpace(line) == "" {
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()

				return m, nil
			}

			m.pushHistory(line)

			result := m.evaluate(line)

			if result.Output != "" {
				m.appendLine(m.styles.Output.Render(result.Output))
			}

			if result.Err != "" {
				m.appendLine(m.styles.Error.Render(result.Err))
			}

			if result.Exit {
				m.quit = true

				return m, tea.Quit
			}

			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()

			return m, nil

		case tea.KeyUp:
			m.navigateHistory(-1)

			return m, nil

		case tea.KeyDown:
			m.navigateHistory(1)

			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	return m, cmd
}

// View renders viewport (banner + history) above the prompt+input.
func (m Model) View() string {
	return m.viewport.View() + "\n" + m.renderInputLine()
}

// renderHistory produces the scrollback content (banner + all
// completed lines). Called on resize and refreshed after each
// evaluation via appendLine + SetContent.
func (m *Model) renderHistory() string {
	var b strings.Builder

	if m.banner != "" {
		b.WriteString(m.styles.Banner.Render(m.banner))
		b.WriteString("\n")
	}

	b.WriteString(m.output.String())

	return b.String()
}

// renderInputLine is the always-visible bottom row: prompt + cursor.
func (m Model) renderInputLine() string {
	return m.styles.Prompt.Render(m.prompt) + m.input.View()
}

// appendLine adds line + "\n" to the output builder. Called after
// each user submission so the viewport reflects the new content.
func (m *Model) appendLine(line string) {
	m.output.WriteString(line)
	m.output.WriteString("\n")
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
// Up (older) or +1 for Down (newer). Wraps between the live line
// and the buffer edges.
func (m *Model) navigateHistory(delta int) {
	if len(m.history) == 0 {
		return
	}

	// Save the live line when starting to navigate from -1.
	if m.historyIdx == -1 && delta < 0 {
		m.liveInput = m.input.Value()
		m.historyIdx = len(m.history) - 1
		m.input.SetValue(m.history[m.historyIdx])
		m.input.CursorEnd()

		return
	}

	next := m.historyIdx + delta

	if next < 0 {
		// Already at the oldest — no-op.
		return
	}

	if next >= len(m.history) {
		// Past the newest — restore live line.
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
// bubble tea models are value types.
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
