package markup

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// tableBuilder collects rows and cells during parsing, then renders an
// aligned table once all data is buffered.
type tableBuilder struct {
	rows    []tableRow
	current *tableRow  // row being built
	cell    *tableCell // cell being built
	padding int        // column gap (default 2)
}

type tableRow struct {
	cells []tableCell
}

// tableCell holds both the raw display text (for width measurement) and
// the styled text (with ANSI codes) for final output.
type tableCell struct {
	raw      string // plain text for measurement
	styled   string // ANSI-styled text for output
	isHeader bool   // true for <th> cells
}

func newTableBuilder() *tableBuilder {
	return &tableBuilder{padding: 2}
}

func (tb *tableBuilder) startRow() {
	tb.current = &tableRow{}
}

func (tb *tableBuilder) endRow() {
	if tb.current != nil {
		tb.rows = append(tb.rows, *tb.current)
		tb.current = nil
	}
}

func (tb *tableBuilder) startCell(header bool) {
	if tb.current == nil {
		tb.startRow()
	}

	tb.cell = &tableCell{isHeader: header}
}

func (tb *tableBuilder) endCell() {
	if tb.cell != nil && tb.current != nil {
		tb.current.cells = append(tb.current.cells, *tb.cell)
		tb.cell = nil
	}
}

// writeCell appends text to the current cell. raw is the unstyled text
// (for width measurement) and styled is the ANSI-rendered version.
func (tb *tableBuilder) writeCell(raw, styled string) {
	if tb.cell != nil {
		tb.cell.raw += raw
		tb.cell.styled += styled
	}
}

// render measures column widths across all rows, then produces the
// aligned table output.
func (tb *tableBuilder) render() string {
	if len(tb.rows) == 0 {
		return ""
	}

	// Determine the maximum number of columns.
	maxCols := 0
	for _, row := range tb.rows {
		if len(row.cells) > maxCols {
			maxCols = len(row.cells)
		}
	}

	if maxCols == 0 {
		return ""
	}

	// Measure column widths using the raw (unstyled) text.
	widths := make([]int, maxCols)

	for _, row := range tb.rows {
		for i, cell := range row.cells {
			w := runewidth.StringWidth(cell.raw)
			if w > widths[i] {
				widths[i] = w
			}
		}
	}

	// Render rows with padding.
	var buf strings.Builder

	for ri, row := range tb.rows {
		if ri > 0 {
			buf.WriteByte('\n')
		}

		for i, cell := range row.cells {
			if i > 0 {
				buf.WriteString(strings.Repeat(" ", tb.padding))
			}

			styled := cell.styled

			// Apply bold to header cells, preserving nested styling.
			if cell.isHeader {
				styled = lipgloss.NewStyle().Bold(true).Render(cell.styled)
			}

			// Pad to column width. We pad with spaces after the styled
			// text, compensating for the difference between the display
			// width (raw) and the actual string length.
			rawWidth := runewidth.StringWidth(cell.raw)
			pad := widths[i] - rawWidth

			buf.WriteString(styled)

			if i < len(row.cells)-1 && pad > 0 {
				buf.WriteString(strings.Repeat(" ", pad))
			}
		}
	}

	return buf.String()
}

// isTableStructuralWhitespace returns true for whitespace-only CharData
// that appears between table structural elements (e.g. newlines between
// <tr> and <td>).
func isTableStructuralWhitespace(text string) bool {
	return strings.TrimSpace(text) == ""
}
