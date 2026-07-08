package progress

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Sentinel glyphs and styles used in progress output. Kept local to the
// progress package so hex/tui does not need a shared styles package for
// what amounts to three symbols.
var (
	checkMark     = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	xMark         = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).SetString("𝘅")
	secondaryText = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

type stepMsg struct {
	index int
	label string
}

type itemDoneMsg struct {
	label string
}

type itemErrMsg struct {
	label string
	err   string
}

type outputMsg struct {
	line string
}

type doneMsg struct{}

type errMsg struct{ err error }

// Model is a bubbletea model that renders a spinner, progress bar, percentage,
// and current item label. The spinner animates independently from the progress.
type Model struct {
	title        string
	total        int
	current      int
	label        string
	barWidth     int
	spinner      spinner.Model
	done         bool
	err          error
	outputLines  []string
	outputHeight int
}

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case stepMsg:
		m.current = msg.index
		m.label = msg.label
		m.outputLines = nil

		return m, nil

	case outputMsg:
		if m.outputHeight > 0 {
			m.outputLines = append(m.outputLines, msg.line)

			if len(m.outputLines) > m.outputHeight {
				m.outputLines = m.outputLines[len(m.outputLines)-m.outputHeight:]
			}
		}

		return m, nil

	case itemDoneMsg:
		return m, tea.Printf(" %s %s", checkMark, msg.label)

	case itemErrMsg:
		lines := strings.Split(msg.err, "\n")
		var detail strings.Builder

		fmt.Fprintf(&detail, " %s %s %s", xMark, msg.label, secondaryText.Render(lines[0]))

		for _, dl := range lines[1:] {
			fmt.Fprintf(&detail, "\n   %s %s", secondaryText.Render("│"), secondaryText.Render(dl))
		}

		return m, tea.Printf("%s", detail.String())

	case doneMsg:
		m.done = true

		return m, tea.Quit

	case errMsg:
		m.err = msg.err

		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd

		m.spinner, cmd = m.spinner.Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.done {
		return ""
	}

	pct := 0
	filled := 0

	if m.total > 0 {
		pct = int(float64(m.current) / float64(m.total) * 100)
		filled = int(float64(m.current) / float64(m.total) * float64(m.barWidth))
	}

	bar := renderBar(filled, m.barWidth)
	label := m.label

	if label != "" {
		label = fmt.Sprintf("%s: %s", m.title, label)
	} else {
		label = m.title
	}

	line := fmt.Sprintf(" %s %s %3d%% %s", m.spinner.View(), bar, pct, label)

	if m.outputHeight > 0 && len(m.outputLines) > 0 {
		var out strings.Builder

		out.WriteString(line)

		for _, ol := range m.outputLines {
			out.WriteString("\n")
			out.WriteString("   ")
			out.WriteString(secondaryText.Render(ol))
		}

		// Pad remaining lines so the view height is stable
		for i := len(m.outputLines); i < m.outputHeight; i++ {
			out.WriteString("\n")
		}

		return out.String()
	}

	return line
}

func renderBar(filled, width int) string {
	var b strings.Builder

	b.WriteString("\033[32m")
	b.WriteString(strings.Repeat("━", filled))
	b.WriteString("\033[30m")

	rest := width - filled

	if rest > 0 && filled > 0 {
		b.WriteString("╺")
		b.WriteString(strings.Repeat("━", rest-1))
	} else if rest > 0 {
		b.WriteString(strings.Repeat("━", rest))
	}

	b.WriteString("\033[39m")

	return b.String()
}
