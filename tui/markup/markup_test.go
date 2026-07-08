package markup_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/jordanbrauer/hex/tui/markup"

	"github.com/muesli/termenv"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	// Force ANSI output so lipgloss emits escape codes in tests.
	markup.SetColorProfile(termenv.ANSI256)
	os.Exit(m.Run())
}

// hasANSI checks if a string contains ANSI escape sequences.
func hasANSI(s string) bool {
	return bytes.Contains([]byte(s), []byte("\x1b["))
}

func TestParseNoTags(t *testing.T) {
	I := NewWithT(t)

	I.Expect(markup.Parse("hello world")).To(Equal("hello world"))
}

func TestParseNoAngleBracketsFastPath(t *testing.T) {
	I := NewWithT(t)

	input := "no tags here, just text with symbols: 1 + 2 = 3"
	I.Expect(markup.Parse(input)).To(Equal(input))
}

func TestParseBold(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse("hello <bold>world</bold>!")

	I.Expect(result).To(ContainSubstring("world"))
	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(HavePrefix("hello "))
	I.Expect(result).To(HaveSuffix("!"))
}

func TestParseDecorations(t *testing.T) {
	I := NewWithT(t)

	decorations := []string{"bold", "dim", "faint", "italic", "underline", "strikethrough"}

	for _, dec := range decorations {
		result := markup.Parse("<" + dec + ">text</" + dec + ">")
		I.Expect(hasANSI(result)).To(BeTrue(), "expected ANSI for <%s>", dec)
		// Note: lipgloss renders some decorations (e.g. underline)
		// character-by-character, so we check individual chars.
		I.Expect(result).To(ContainSubstring("t"), "expected text content for <%s>", dec)
	}
}

func TestParseFgNamedColor(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse(`<fg color="red">error</fg>`)

	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(ContainSubstring("error"))
}

func TestParseFgHexColor(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse(`<fg color="#ff5733">hex</fg>`)

	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(ContainSubstring("hex"))
}

func TestParseFgAnsi256(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse(`<fg color="245">gray</fg>`)

	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(ContainSubstring("gray"))
}

func TestParseBgColor(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse(`<bg color="blue">highlight</bg>`)

	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(ContainSubstring("highlight"))
}

func TestParseNested(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse(`<bold>hello <fg color="red">world</fg> bye</bold>`)

	I.Expect(result).To(ContainSubstring("hello"))
	I.Expect(result).To(ContainSubstring("world"))
	I.Expect(result).To(ContainSubstring("bye"))
	I.Expect(hasANSI(result)).To(BeTrue())
}

func TestParseStyleCompound(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse(`<ansi bold="true" fg="red" bg="white">compound</ansi>`)

	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(ContainSubstring("compound"))
}

func TestParseStylePartialAttrs(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse(`<ansi fg="cyan">partial</ansi>`)

	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(ContainSubstring("partial"))
}

func TestParseStyleDecorationFalse(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse(`<ansi bold="false">no</ansi>`)

	I.Expect(result).To(Equal("no"))
}

func TestParseStyleDecorationTrue(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse(`<ansi bold="true">yes</ansi>`)

	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(ContainSubstring("yes"))
}

func TestParseUnknownTagPassesThrough(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse("<unknown>text</unknown>")

	I.Expect(result).To(Equal("text"))
}

func TestParseMixedTextAndTags(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse("one <bold>two</bold> three <italic>four</italic> five")

	I.Expect(result).To(ContainSubstring("one "))
	I.Expect(result).To(ContainSubstring("two"))
	I.Expect(result).To(ContainSubstring(" three "))
	I.Expect(result).To(ContainSubstring("four"))
	I.Expect(result).To(HaveSuffix(" five"))
	I.Expect(hasANSI(result)).To(BeTrue())
}

func TestParseMultiline(t *testing.T) {
	I := NewWithT(t)

	input := "<bold>Header</bold>\n  line1\n  <fg color=\"red\">line2</fg>"
	result := markup.Parse(input)

	I.Expect(result).To(ContainSubstring("Header"))
	I.Expect(result).To(ContainSubstring("\n  line1\n  "))
	I.Expect(result).To(ContainSubstring("line2"))
}

func TestParseMalformedGraceful(t *testing.T) {
	I := NewWithT(t)

	// Unclosed tag — should render what it can without panicking.
	result := markup.Parse("hello <bold>world")

	I.Expect(result).To(ContainSubstring("hello"))
}

func TestParseEmptyInput(t *testing.T) {
	I := NewWithT(t)

	I.Expect(markup.Parse("")).To(Equal(""))
}

func TestParseNamedColorVariants(t *testing.T) {
	I := NewWithT(t)

	colors := []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white", "gray", "grey"}

	for _, color := range colors {
		result := markup.Parse(`<fg color="` + color + `">text</fg>`)
		I.Expect(hasANSI(result)).To(BeTrue(), "expected ANSI for color %s", color)
	}
}

func TestParseBrightColors(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse(`<fg color="bright-red">bright</fg>`)

	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(ContainSubstring("bright"))
}

func TestParseDeeplyNested(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse(`<bold><italic><fg color="red">deep</fg></italic></bold>`)

	I.Expect(result).To(ContainSubstring("deep"))
	I.Expect(hasANSI(result)).To(BeTrue())
}

// --- Table support ---

func TestParseTableBasic(t *testing.T) {
	I := NewWithT(t)

	input := `<table>
<tr><th>name</th><th>status</th></tr>
<tr><td>Acme</td><td>active</td></tr>
<tr><td>Globex</td><td>inactive</td></tr>
</table>`

	result := markup.Parse(input)

	// Headers should be bold.
	I.Expect(hasANSI(result)).To(BeTrue())
	// All text should be present.
	I.Expect(result).To(ContainSubstring("name"))
	I.Expect(result).To(ContainSubstring("status"))
	I.Expect(result).To(ContainSubstring("Acme"))
	I.Expect(result).To(ContainSubstring("active"))
	I.Expect(result).To(ContainSubstring("Globex"))
	I.Expect(result).To(ContainSubstring("inactive"))
}

func TestParseTableColumnAlignment(t *testing.T) {
	I := NewWithT(t)

	// "Acme" (4) vs "Globex Corp" (11) — first column should pad Acme.
	input := `<table>
<tr><th>name</th><th>id</th></tr>
<tr><td>Acme</td><td>1</td></tr>
<tr><td>Globex Corp</td><td>2</td></tr>
</table>`

	result := markup.Parse(input)
	lines := strings.Split(strings.TrimSpace(result), "\n")

	// Should have 3 lines (header + 2 data rows).
	I.Expect(len(lines)).To(BeNumerically(">=", 3))
}

func TestParseTableStyledCells(t *testing.T) {
	I := NewWithT(t)

	input := `<table>
<tr><th>name</th><th>status</th></tr>
<tr><td>Acme</td><td><fg color="green">active</fg></td></tr>
</table>`

	result := markup.Parse(input)

	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(ContainSubstring("Acme"))
	I.Expect(result).To(ContainSubstring("active"))
}

func TestParseTableSurroundedByText(t *testing.T) {
	I := NewWithT(t)

	input := "before\n<table>\n<tr><td>a</td><td>b</td></tr>\n</table>\nafter"
	result := markup.Parse(input)

	I.Expect(result).To(HavePrefix("before\n"))
	I.Expect(result).To(HaveSuffix("after"))
	I.Expect(result).To(ContainSubstring("a"))
	I.Expect(result).To(ContainSubstring("b"))
}

func TestParseTableEmpty(t *testing.T) {
	I := NewWithT(t)

	input := "<table></table>"
	result := markup.Parse(input)

	I.Expect(result).To(Equal(""))
}

func TestParseTableNestedStyles(t *testing.T) {
	I := NewWithT(t)

	// Style tags inside cells should compose correctly.
	input := `<table>
<tr><td><bold><fg color="red">error</fg></bold></td><td>details</td></tr>
</table>`

	result := markup.Parse(input)

	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(ContainSubstring("error"))
	I.Expect(result).To(ContainSubstring("details"))
}

// --- Self-closing elements ---

func TestParseHorizontalRule(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse("above\n<hr/>\nbelow")

	I.Expect(result).To(ContainSubstring("══════"))
	I.Expect(result).To(HavePrefix("above\n"))
	I.Expect(result).To(HaveSuffix("\nbelow"))
}

func TestParseCheckIcon(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse("<check/> done")

	I.Expect(result).To(ContainSubstring("✓"))
	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(ContainSubstring("done"))
}

func TestParseXIcon(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse("<x/> failed")

	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(ContainSubstring("failed"))
}

func TestParseWarnIcon(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse("<warn/> caution")

	I.Expect(result).To(ContainSubstring("!"))
	I.Expect(hasANSI(result)).To(BeTrue())
}

func TestParseIconsInTableCells(t *testing.T) {
	I := NewWithT(t)

	input := `<table>
<tr><td><check/></td><td>passing</td></tr>
<tr><td><x/></td><td>failing</td></tr>
</table>`

	result := markup.Parse(input)

	I.Expect(result).To(ContainSubstring("✓"))
	I.Expect(hasANSI(result)).To(BeTrue())
	I.Expect(result).To(ContainSubstring("passing"))
	I.Expect(result).To(ContainSubstring("failing"))
}

// --- Tag badges ---

func TestParseTagDefault(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse("<tag>Production</tag>")

	I.Expect(result).To(ContainSubstring("Production"))
	I.Expect(hasANSI(result)).To(BeTrue())
}

func TestParseTagColors(t *testing.T) {
	I := NewWithT(t)

	colors := []string{"success", "info", "warning", "error"}

	for _, color := range colors {
		result := markup.Parse(`<tag color="` + color + `">Label</tag>`)
		I.Expect(hasANSI(result)).To(BeTrue(), "expected ANSI for tag color %s", color)
		I.Expect(result).To(ContainSubstring("Label"), "expected text for tag color %s", color)
	}
}

func TestParseTagInline(t *testing.T) {
	I := NewWithT(t)

	result := markup.Parse(`hello <tag color="info">Sandbox</tag> world`)

	I.Expect(result).To(HavePrefix("hello "))
	I.Expect(result).To(HaveSuffix(" world"))
	I.Expect(result).To(ContainSubstring("Sandbox"))
}

func TestParseTagInTableCell(t *testing.T) {
	I := NewWithT(t)

	input := `<table>
<tr><td>App</td><td><tag color="success">Live</tag></td></tr>
</table>`

	result := markup.Parse(input)

	I.Expect(result).To(ContainSubstring("App"))
	I.Expect(result).To(ContainSubstring("Live"))
	I.Expect(hasANSI(result)).To(BeTrue())
}
