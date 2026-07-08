// Package markup provides inline XML-style tags for styled terminal output.
//
// Supported tags:
//
//	<bold>text</bold>
//	<dim>text</dim>
//	<italic>text</italic>
//	<underline>text</underline>
//	<strikethrough>text</strikethrough>
//	<fg color="red">text</fg>
//	<fg color="#ff5733">text</fg>
//	<fg color="245">text</fg>
//	<bg color="blue">text</bg>
//	<ansi bold="true" fg="red" bg="#fff">text</ansi>
//
// Tags nest naturally — inner tags compose with the outer style:
//
//	<bold>hello <fg color="red">world</fg></bold>
//
// Self-closing elements:
//
//	<hr/>                         ── horizontal rule (═══...)
//	<check/>                      ── green ✓
//	<x/>                          ── red ✗
//	<warn/>                       ── yellow !
//
// Badges:
//
//	<tag>Custom</tag>             ── default badge
//	<tag color="success">Live</tag>
//	<tag color="info">Sandbox</tag>
//	<tag color="warning">Staging</tag>
//	<tag color="error">Failed</tag>
//
// Table support:
//
//	<table>
//	<tr><th>name</th><th>status</th></tr>
//	<tr><td>Acme</td><td><fg color="green">active</fg></td></tr>
//	</table>
package markup

import (
	"encoding/xml"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// namedColors maps color names to ANSI color numbers.
var namedColors = map[string]string{
	"black":          "0",
	"red":            "1",
	"green":          "2",
	"yellow":         "3",
	"blue":           "4",
	"magenta":        "5",
	"cyan":           "6",
	"white":          "7",
	"bright-black":   "8",
	"gray":           "8",
	"grey":           "8",
	"bright-red":     "9",
	"bright-green":   "10",
	"bright-yellow":  "11",
	"bright-blue":    "12",
	"bright-magenta": "13",
	"bright-cyan":    "14",
	"bright-white":   "15",
}

// resolveColor converts a color string (name, hex, or ANSI number) to a
// lipgloss.Color value.
func resolveColor(s string) lipgloss.Color {
	s = strings.TrimSpace(strings.ToLower(s))

	if ansi, ok := namedColors[s]; ok {
		return lipgloss.Color(ansi)
	}

	// Hex (#rrggbb) and ANSI 256 numbers pass through directly.
	return lipgloss.Color(s)
}

// selfClosingTags is the set of tags that emit content immediately on
// the start element (they may appear as <tag/> in XML).
var selfClosingTags = map[string]bool{
	"hr":    true,
	"check": true,
	"x":     true,
	"warn":  true,
}

// ruleString is the horizontal rule used by <hr/>.
const ruleString = "══════════════════════════════════════════════════════════"

// renderSelfClosing returns the output for a self-closing element.
// The base style is composed onto the icon's own styling so outer
// decorations (e.g. <bold><check/></bold>) are preserved.
func renderSelfClosing(tag string, base lipgloss.Style) string {
	switch tag {
	case "hr":
		return base.Render(ruleString)
	case "check":
		return base.Foreground(lipgloss.Color("42")).Render("✓")
	case "x":
		return base.Foreground(lipgloss.Color("1")).Render("✗")
	case "warn":
		return base.Foreground(lipgloss.Color("3")).Render("!")
	default:
		return ""
	}
}

// selfClosingIcon returns the plain-text glyph for a self-closing tag,
// suitable for table column width calculations.
func selfClosingIcon(tag string) string {
	switch tag {
	case "hr":
		return ruleString
	case "check":
		return "✓"
	case "x":
		return "✗"
	case "warn":
		return "!"
	default:
		return ""
	}
}

// tagColors maps tag color attribute values to background/foreground pairs.
var tagColors = map[string][2]string{
	"success": {"#d1fadf", "#05603a"},
	"info":    {"#eaebff", "#030d8c"},
	"warning": {"#fef0c7", "#93370d"},
	"error":   {"#fef3f2", "#b42318"},
	"":        {"#d1fadf", "#05603a"}, // default = success
}

// renderTag produces a styled badge from a <tag> element's collected text.
func renderTag(text, color string) string {
	colors, ok := tagColors[color]
	if !ok {
		colors = tagColors[""]
	}

	return lipgloss.NewStyle().
		Bold(true).
		Background(lipgloss.Color(colors[0])).
		Foreground(lipgloss.Color(colors[1])).
		Padding(0, 1).
		Render(text)
}

// decorationTags is the set of tag names treated as decoration toggles.
var decorationTags = map[string]bool{
	"bold":          true,
	"dim":           true,
	"faint":         true,
	"italic":        true,
	"underline":     true,
	"strikethrough": true,
}

// applyDecoration returns a new style with the named decoration enabled.
func applyDecoration(s lipgloss.Style, name string) lipgloss.Style {
	switch name {
	case "bold":
		return s.Bold(true)
	case "dim", "faint":
		return s.Faint(true)
	case "italic":
		return s.Italic(true)
	case "underline":
		return s.Underline(true)
	case "strikethrough":
		return s.Strikethrough(true)
	default:
		return s
	}
}

// styleFromElement builds a lipgloss.Style delta from an XML start element,
// composing onto the provided base style.
func styleFromElement(base lipgloss.Style, el xml.StartElement) lipgloss.Style {
	tag := el.Name.Local
	s := base

	// Decoration tags: <bold>, <italic>, etc.
	if decorationTags[tag] {
		return applyDecoration(s, tag)
	}

	// <fg color="...">
	if tag == "fg" {
		for _, attr := range el.Attr {
			if attr.Name.Local == "color" {
				s = s.Foreground(resolveColor(attr.Value))
			}
		}

		return s
	}

	// <bg color="...">
	if tag == "bg" {
		for _, attr := range el.Attr {
			if attr.Name.Local == "color" {
				s = s.Background(resolveColor(attr.Value))
			}
		}

		return s
	}

	// <ansi bold="true" fg="red" bg="#fff" ...>
	if tag == "ansi" {
		for _, attr := range el.Attr {
			switch attr.Name.Local {
			case "fg":
				s = s.Foreground(resolveColor(attr.Value))
			case "bg":
				s = s.Background(resolveColor(attr.Value))
			default:
				// Treat as decoration if value is "true" or attr has no meaningful value.
				if decorationTags[attr.Name.Local] && isTruthy(attr.Value) {
					s = applyDecoration(s, attr.Name.Local)
				}
			}
		}

		return s
	}

	// Unknown tag — return base unchanged (text passes through).
	return s
}

// isTruthy returns true for "true", "1", "yes", or empty string (bare attr).
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "":
		return true
	default:
		return false
	}
}

// Parse processes inline XML-style tags in the input and returns a string
// with ANSI escape sequences applied via lipgloss. It uses the global
// lipgloss color profile, which is typically set at application startup
// via lipgloss.SetColorProfile.
//
// If the input contains no tags, it is returned unchanged. If the XML is
// malformed, the original input is returned unchanged.
func Parse(input string) string {
	return parse(input)
}

// SetColorProfile sets the lipgloss color profile globally. This is
// exposed for tests that need ANSI output in non-TTY environments.
func SetColorProfile(p termenv.Profile) {
	lipgloss.SetColorProfile(p)
}

func parse(input string) string {
	// Fast path: no markup at all (no tags, no entities).
	if !strings.Contains(input, "<") && !strings.Contains(input, "&") {
		return input
	}

	// Wrap in a synthetic root to form valid XML. If parsing fails
	// (malformed input), return the original input unchanged.
	decoder := xml.NewDecoder(strings.NewReader("<_>" + input + "</_>"))

	type tagFrame struct {
		name  string
		style lipgloss.Style
		color string // for <tag color="..."> badges
		text  string // buffered text for <tag>
	}

	// composedStyle returns the current style from the top of the stack,
	// or a blank style if the stack is empty.
	composedStyle := func(s []tagFrame) lipgloss.Style {
		if len(s) == 0 {
			return lipgloss.NewStyle()
		}

		return s[len(s)-1].style
	}

	var (
		buf   strings.Builder
		stack []tagFrame
		table *tableBuilder // non-nil when inside <table>
	)

	// Pre-size the buffer for the common case (output ≈ input length + ANSI codes).
	buf.Grow(len(input) + 64)

	// Start with an unstyled base on the stack.
	stack = append(stack, tagFrame{name: "_", style: lipgloss.NewStyle()})

	for {
		tok, err := decoder.Token()
		if err != nil {
			// EOF is normal completion. Any other error means
			// malformed input — return the original unchanged.
			if err.Error() != "EOF" {
				return input
			}

			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "_" {
				continue
			}

			// Self-closing elements: <hr/>, <check/>, <x/>, <warn/>.
			if selfClosingTags[t.Name.Local] {
				currentStyle := composedStyle(stack)

				if table != nil && table.cell != nil {
					raw := selfClosingIcon(t.Name.Local)
					styled := renderSelfClosing(t.Name.Local, currentStyle)
					table.writeCell(raw, styled)
				} else {
					buf.WriteString(renderSelfClosing(t.Name.Local, currentStyle))
				}

				continue
			}

			// Table structural tags.
			switch t.Name.Local {
			case "table":
				table = newTableBuilder()

				continue
			case "tr":
				if table != nil {
					table.startRow()
				}

				continue
			case "th":
				if table != nil {
					table.startCell(true)
				}

				continue
			case "td":
				if table != nil {
					table.startCell(false)
				}

				continue
			}

			// <tag> badges — buffer text for rendering at close.
			if t.Name.Local == "tag" {
				color := ""
				for _, attr := range t.Attr {
					if attr.Name.Local == "color" {
						color = attr.Value
					}
				}

				stack = append(stack, tagFrame{
					name:  "tag",
					style: stack[len(stack)-1].style,
					color: color,
				})

				continue
			}

			// Style tags — push onto the style stack.
			current := stack[len(stack)-1].style
			next := styleFromElement(current, t)
			stack = append(stack, tagFrame{name: t.Name.Local, style: next})

		case xml.EndElement:
			if t.Name.Local == "_" {
				continue
			}

			// Self-closing end elements (XML parser emits these).
			if selfClosingTags[t.Name.Local] {
				continue
			}

			// Table structural close tags.
			switch t.Name.Local {
			case "table":
				if table != nil {
					buf.WriteString(table.render())
					table = nil
				}

				continue
			case "tr":
				if table != nil {
					table.endRow()
				}

				continue
			case "th", "td":
				if table != nil {
					table.endCell()
				}

				continue
			}

			// </tag> — render the buffered badge.
			if t.Name.Local == "tag" && len(stack) > 1 && stack[len(stack)-1].name == "tag" {
				frame := stack[len(stack)-1]
				stack = stack[:len(stack)-1]

				rendered := renderTag(frame.text, frame.color)

				if table != nil && table.cell != nil {
					table.writeCell(frame.text, rendered)
				} else {
					buf.WriteString(rendered)
				}

				continue
			}

			// Style tag close — pop the style stack.
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}

		case xml.CharData:
			text := string(t)

			// Inside a <tag> — buffer raw text for badge rendering.
			if len(stack) > 1 && stack[len(stack)-1].name == "tag" {
				stack[len(stack)-1].text += text

				continue
			}

			// Inside a table cell — buffer both raw and styled text.
			if table != nil && table.cell != nil {
				if len(stack) > 1 {
					current := stack[len(stack)-1].style
					table.writeCell(text, current.Render(text))
				} else {
					table.writeCell(text, text)
				}

				continue
			}

			// Inside a table but outside a cell — skip structural
			// whitespace (newlines between tags).
			if table != nil {
				if isTableStructuralWhitespace(text) {
					continue
				}
				// Non-whitespace text outside cells is passed through.
				buf.WriteString(text)

				continue
			}

			// Normal (non-table) text rendering.
			if len(stack) <= 1 {
				buf.WriteString(text)
			} else {
				current := stack[len(stack)-1].style
				buf.WriteString(current.Render(text))
			}
		}
	}

	return buf.String()
}
