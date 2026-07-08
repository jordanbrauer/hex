package progress

import (
	"fmt"
	"github.com/jordanbrauer/hex/tui/components/spinner"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const defaultBarWidth = 20

// Option configures a progress bar run.
type Option func(*Model)

// WithOutput enables a scrolling output window below the progress bar with the
// given number of visible lines. A height of 0 disables the window (default).
func WithOutput(height int) Option {
	return func(m *Model) {
		m.outputHeight = height
	}
}

// Failure records a single item that failed during a progress run.
type Failure struct {
	Label string
	Err   string
}

// Sender provides methods to report progress from within an Action.
type Sender struct {
	p            *tea.Program
	outputHeight int
	failures     []Failure
}

// Step updates the progress bar position and current item label.
func (s *Sender) Step(index int, label string) {
	s.p.Send(stepMsg{index: index, label: label})
}

// ItemDone prints a success line for a completed item above the progress bar.
func (s *Sender) ItemDone(label string) {
	s.p.Send(itemDoneMsg{label: label})
}

// ItemErr prints an error line for a failed item above the progress bar and
// records the failure. The error string may contain multiple lines; additional
// lines are printed indented below the item.
func (s *Sender) ItemErr(label string, err string) {
	s.failures = append(s.failures, Failure{Label: label, Err: err})
	s.p.Send(itemErrMsg{label: label, err: err})
}

// Failures returns all recorded item failures.
func (s *Sender) Failures() []Failure {
	return s.failures
}

// Err returns a summary error if any items failed, or nil if all succeeded.
func (s *Sender) Err() error {
	if len(s.failures) == 0 {
		return nil
	}

	labels := make([]string, len(s.failures))

	for i, f := range s.failures {
		labels[i] = f.Label
	}

	return fmt.Errorf("%d item(s) failed: %s", len(s.failures), strings.Join(labels, ", "))
}

// Output streams a line of command output into the scrolling window below the
// progress bar. This is a no-op if the output window was not enabled via
// WithOutput.
func (s *Sender) Output(line string) {
	if s.outputHeight > 0 {
		s.p.Send(outputMsg{line: line})
	}
}

// Action is a function that receives a Sender to report progress.
type Action func(s *Sender) error

// Bar is a configured progress bar ready to run.
type Bar struct {
	title string
	total int
	opts  []Option
}

// New creates a progress bar builder. Call Run to execute it.
func New(title string, total int) *Bar {
	return &Bar{title: title, total: total}
}

// WithOutput enables a scrolling output window with the given height.
func (b *Bar) WithOutput(height int) *Bar {
	b.opts = append(b.opts, WithOutput(height))

	return b
}

// Run executes the progress bar with the given action.
func (b *Bar) Run(action Action) error {
	return Run(b.title, b.total, action, b.opts...)
}

// Run creates and runs a progress bar program. The action function runs in a
// goroutine and reports progress via the Sender. The spinner animates
// independently. The component erases itself on completion.
func Run(title string, total int, action Action, opts ...Option) error {
	m := Model{
		title:    title,
		total:    total,
		barWidth: defaultBarWidth,
		spinner:  spinner.New(),
	}

	for _, opt := range opts {
		opt(&m)
	}

	p := tea.NewProgram(m)

	go func() {
		err := action(&Sender{p: p, outputHeight: m.outputHeight})

		if err != nil {
			p.Send(errMsg{err: err})

			return
		}

		p.Send(doneMsg{})
	}()

	result, err := p.Run()

	if err != nil {
		return err
	}

	if model, ok := result.(Model); ok && model.err != nil {
		return model.err
	}

	return nil
}
