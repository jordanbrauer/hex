// Package console provides a high-level output and interaction API for CLI
// commands. It wraps a cobra.Command and renderer, providing structured
// output (tables, records, lists), interactive prompts (ask, confirm,
// choose, secret), feedback (spinner, progress), and markup-aware printing.
//
// Usage:
//
//	c := console.New(cmd)
//	c.Render(vm)                           // template + markup → output
//	c.Print("<bold>hello</bold> world")    // markup-aware print
//	name, _ := c.Ask("What is your name?")
//	ok, _ := c.Confirm("Continue?")
package console

import (
	"errors"
	"fmt"

	"github.com/jordanbrauer/hex/tui/components/progress"
	"github.com/jordanbrauer/hex/tui/markup"
	"github.com/jordanbrauer/hex/tui/renderer"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/log"
	"github.com/goforj/godump"
	"github.com/spf13/cobra"
)

// ErrNonInteractive is returned when a prompt is called in non-interactive mode.
var ErrNonInteractive = errors.New("prompt requires interactive mode (remove --non-interactive)")

// Console wraps a cobra.Command with structured output and interaction methods.
type Console struct {
	cmd      *cobra.Command
	renderer renderer.Renderer
	logger   *log.Logger
}

// New creates a Console from a cobra command. The renderer format is
// determined by the command's --format flag.
func New(cmd *cobra.Command) *Console {
	logger := log.NewWithOptions(cmd.ErrOrStderr(), log.Options{})
	logger.SetLevel(log.GetLevel())

	return &Console{
		cmd:      cmd,
		renderer: renderer.New(cmd),
		logger:   logger,
	}
}

// --- Renderer ---

// Render delegates to the underlying renderer. For Templated viewmodels,
// this executes the template and applies markup post-processing. For JSON
// format, it marshals the struct.
func (c *Console) Render(data any) error {
	return c.renderer.Render(data)
}

// Format returns the active output format.
func (c *Console) Format() renderer.Format {
	return c.renderer.Format()
}

// isTextFormat returns true when the output format is human-readable
// (text or table), as opposed to a machine-readable format like JSON.
func (c *Console) isTextFormat() bool {
	f := c.renderer.Format()
	return f == renderer.FormatTable || f == renderer.FormatPlain
}

// --- Output ---
//
// Print, Println, Printf, and the status helpers write to stderr when
// the output format is machine-readable (JSON, YAML, TOML, CSV) to
// avoid contaminating structured stdout.

// Print writes markup-processed text to the appropriate output stream.
func (c *Console) Print(text string) {
	if c.isTextFormat() {
		c.cmd.Print(markup.Parse(text))
	} else {
		_, _ = fmt.Fprint(c.cmd.ErrOrStderr(), markup.Parse(text))
	}
}

// Println writes markup-processed text followed by a newline.
func (c *Console) Println(text string) {
	if c.isTextFormat() {
		c.cmd.Println(markup.Parse(text))
	} else {
		_, _ = fmt.Fprintln(c.cmd.ErrOrStderr(), markup.Parse(text))
	}
}

// Printf formats and writes markup-processed text.
func (c *Console) Printf(format string, args ...any) {
	if c.isTextFormat() {
		c.cmd.Print(markup.Parse(fmt.Sprintf(format, args...)))
	} else {
		_, _ = fmt.Fprint(c.cmd.ErrOrStderr(), markup.Parse(fmt.Sprintf(format, args...)))
	}
}

// Success prints a green check mark followed by the message.
func (c *Console) Success(message string) {
	c.Println(fmt.Sprintf("<check/> %s", message))
}

// Fail prints a red x mark followed by the message.
func (c *Console) Fail(message string) {
	c.Println(fmt.Sprintf("<x/> %s", message))
}

// Alert prints a yellow warning mark followed by the message.
func (c *Console) Alert(message string) {
	c.Println(fmt.Sprintf("<warn/> %s", message))
}

// Info prints a dim informational message.
func (c *Console) Info(message string) {
	c.Println(fmt.Sprintf("<dim>%s</dim>", message))
}

// --- Logging ---
//
// Log methods write to the command's stderr via a console-scoped logger,
// so they never contaminate structured stdout output.

// Log writes an info-level log message.
func (c *Console) Log(msg string, keyvals ...any) {
	c.logger.Info(msg, keyvals...)
}

// Debug writes a debug-level log message.
func (c *Console) Debug(msg string, keyvals ...any) {
	c.logger.Debug(msg, keyvals...)
}

// Warn writes a warn-level log message.
func (c *Console) Warn(msg string, keyvals ...any) {
	c.logger.Warn(msg, keyvals...)
}

// Error writes an error-level log message.
func (c *Console) Error(msg string, keyvals ...any) {
	c.logger.Error(msg, keyvals...)
}

// --- Debug ---

// Dump pretty-prints values to stderr for debugging. Output is always
// sent to stderr to avoid contaminating structured stdout.
func (c *Console) Dump(v ...any) {
	d := godump.NewDumper(godump.WithoutHeader(), godump.WithWriter(c.cmd.ErrOrStderr()))
	d.Dump(v...)
}

// --- Interactive ---
//
// All prompt methods check the --non-interactive flag and return
// ErrNonInteractive if prompts are disabled.

// isNonInteractive returns true when the --non-interactive flag is set.
func (c *Console) isNonInteractive() bool {
	ni, _ := c.cmd.Root().Flags().GetBool("non-interactive")
	return ni
}

// Confirm shows a yes/no confirmation prompt and returns the user's choice.
// Returns ErrNonInteractive if --non-interactive is set.
func (c *Console) Confirm(title string) (bool, error) {
	if c.isNonInteractive() {
		return false, ErrNonInteractive
	}

	var confirmed bool

	err := huh.NewConfirm().
		Title(title).
		Affirmative("Yes").
		Negative("No").
		Value(&confirmed).
		Run()

	return confirmed, err
}

// Ask shows a text input prompt and returns the user's input.
// Returns ErrNonInteractive if --non-interactive is set.
func (c *Console) Ask(title string) (string, error) {
	if c.isNonInteractive() {
		return "", ErrNonInteractive
	}

	var value string

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title(title).Value(&value),
		),
	).Run()

	return value, err
}

// AskWithPlaceholder shows a text input prompt with a placeholder hint.
// Returns ErrNonInteractive if --non-interactive is set.
func (c *Console) AskWithPlaceholder(title, placeholder string) (string, error) {
	if c.isNonInteractive() {
		return "", ErrNonInteractive
	}

	var value string

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title(title).Placeholder(placeholder).Value(&value),
		),
	).Run()

	return value, err
}

// Secret shows a password input prompt (masked characters).
// Returns ErrNonInteractive if --non-interactive is set.
func (c *Console) Secret(title string) (string, error) {
	if c.isNonInteractive() {
		return "", ErrNonInteractive
	}

	var value string

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title(title).EchoMode(huh.EchoModePassword).Value(&value),
		),
	).Run()

	return value, err
}

// Option represents a selectable choice for Choose.
type Option struct {
	Label string
	Value string
}

// huhOptions builds huh.Option values from Option, marking any value present
// in preselected as selected by default. preselected may be nil. Shared by
// Choose (single-select) and Select (multi-select).
func huhOptions(options []Option, preselected map[string]bool) []huh.Option[string] {
	opts := make([]huh.Option[string], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o.Label, o.Value).Selected(preselected[o.Value])
	}

	return opts
}

// Choose shows a selection prompt and returns the chosen value.
// Returns ErrNonInteractive if --non-interactive is set.
func (c *Console) Choose(title string, options []Option) (string, error) {
	if c.isNonInteractive() {
		return "", ErrNonInteractive
	}

	var value string

	opts := huhOptions(options, nil)

	err := huh.NewSelect[string]().
		Title(title).
		Options(opts...).
		Value(&value).
		Run()

	return value, err
}

// ChooseStrings is a convenience wrapper for Choose when labels and values
// are the same.
func (c *Console) ChooseStrings(title string, choices []string) (string, error) {
	opts := make([]Option, len(choices))
	for i, ch := range choices {
		opts[i] = Option{Label: ch, Value: ch}
	}

	return c.Choose(title, opts)
}

// Select shows a multi-select prompt and returns the chosen values.
// preselected values are checked by default. Returns ErrNonInteractive if
// --non-interactive is set.
func (c *Console) Select(title string, options []Option, preselected []string) ([]string, error) {
	if c.isNonInteractive() {
		return nil, ErrNonInteractive
	}

	pre := make(map[string]bool, len(preselected))
	for _, v := range preselected {
		pre[v] = true
	}

	values := []string{}

	opts := huhOptions(options, pre)

	err := huh.NewMultiSelect[string]().
		Title(title).
		Options(opts...).
		Value(&values).
		Run()

	return values, err
}

// SelectStrings is a convenience wrapper for Select when labels and
// values are the same.
func (c *Console) SelectStrings(title string, choices []string, preselected []string) ([]string, error) {
	opts := make([]Option, len(choices))
	for i, ch := range choices {
		opts[i] = Option{Label: ch, Value: ch}
	}

	return c.Select(title, opts, preselected)
}

// --- Feedback ---

// Spinner shows a spinner with a title while the action runs.
func (c *Console) Spinner(title string, action func()) error {
	return spinner.New().Title(title).Action(action).Run()
}

// Progress returns a configured progress bar ready to run.
//
//	c.Progress("deploying", len(services)).Run(func(s *progress.Sender) error {
//	    for i, svc := range services {
//	        s.Step(i, svc.Name)
//	        // ...
//	        s.ItemDone(svc.Name)
//	    }
//	    s.Step(len(services), "")
//	    return nil
//	})
func (c *Console) Progress(title string, total int) *progress.Bar {
	return progress.New(title, total)
}
