// Package spinner is a thin wrapper around bubbles/spinner that ships hex's
// default spinner style. It is intentionally minimal — consumers who want a
// custom spinner can construct one directly with bubbles/spinner.
package spinner

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// Model re-exports bubbles/spinner.Model so consumers of hex/tui do not
// need to import the underlying package for common uses.
type Model = spinner.Model

// TickMsg re-exports bubbles/spinner.TickMsg.
type TickMsg = spinner.TickMsg

// DefaultColor is the ANSI color code applied to the spinner's foreground.
// Consumers may override before calling New to change the palette without
// constructing their own spinner from scratch.
var DefaultColor = lipgloss.Color("205")

// New returns a spinner styled with hex's defaults (dot animation, DefaultColor
// foreground).
func New() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(DefaultColor)

	return s
}
