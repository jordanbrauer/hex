package renderer_test

import (
	"bytes"
	"fmt"
	"html/template"
	"testing"

	"github.com/jordanbrauer/hex/tui/renderer"

	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

type testData struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (d testData) String() string {
	return d.Name
}

func (d testData) Table() string {
	return fmt.Sprintf("| name  | count |\n| %s | %d     |\n", d.Name, d.Count)
}

func newTestCmd(format string) (*cobra.Command, *bytes.Buffer) {
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringP("format", "f", format, "")
	cmd.SetOut(buf)

	return cmd, buf
}

func TestRenderJSON(t *testing.T) {
	I := NewWithT(t)

	cmd, buf := newTestCmd("json")
	r := renderer.New(cmd)

	err := r.Render(testData{Name: "foo", Count: 3})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(ContainSubstring(`"name": "foo"`))
	I.Expect(buf.String()).To(ContainSubstring(`"count": 3`))
}

func TestRenderPlain(t *testing.T) {
	I := NewWithT(t)

	cmd, buf := newTestCmd("plain")
	r := renderer.New(cmd)

	// plain uses Tabular when available (same as table)
	err := r.Render(testData{Name: "foo", Count: 3})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(ContainSubstring("| name  | count |"))
}

func TestRenderPlainFallsBackToStringer(t *testing.T) {
	I := NewWithT(t)

	// stringOnly implements Stringer but not Tabular
	type stringOnly struct{ Name string }

	cmd, buf := newTestCmd("plain")
	r := renderer.New(cmd)

	err := r.Render(stringOnly{Name: "hello"})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(Equal("{hello}\n"))
}

func TestRenderTable(t *testing.T) {
	I := NewWithT(t)

	cmd, buf := newTestCmd("table")
	r := renderer.New(cmd)

	err := r.Render(testData{Name: "foo", Count: 3})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(ContainSubstring("| name  | count |"))
	I.Expect(buf.String()).To(ContainSubstring("| foo |"))
}

func TestRenderDefaultIsTable(t *testing.T) {
	I := NewWithT(t)

	cmd, buf := newTestCmd("")
	r := renderer.New(cmd)

	I.Expect(r.Format()).To(Equal(renderer.FormatTable))

	err := r.Render(testData{Name: "bar", Count: 1})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(ContainSubstring("| name  | count |"))
}

func TestRenderTableFallsBackToPlain(t *testing.T) {
	I := NewWithT(t)

	cmd, buf := newTestCmd("table")
	r := renderer.New(cmd)

	// A type without Tabular interface falls back to plain
	err := r.Render("just a string")

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(Equal("just a string\n"))
}

func TestNewWith(t *testing.T) {
	I := NewWithT(t)

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatJSON, buf)

	err := r.Render(map[string]string{"key": "value"})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(ContainSubstring(`"key": "value"`))
}

// --- Templated interface ---

type templatedData struct {
	Greeting string `json:"greeting"`
	Name     string `json:"template_name"`
	tmpl     *template.Template
}

func (d templatedData) Template() *template.Template {
	return d.tmpl
}

func TestRenderTemplated(t *testing.T) {
	I := NewWithT(t)

	tmpl := renderer.NewTemplate("test", "{{ .Greeting }}, {{ .Name }}!")

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatTable, buf)

	err := r.Render(templatedData{
		Greeting: "Hello",
		Name:     "World",
		tmpl:     tmpl,
	})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(Equal("Hello, World!"))
}

func TestRenderTemplatedJSON(t *testing.T) {
	I := NewWithT(t)

	tmpl := renderer.NewTemplate("test", "{{ .Greeting }}, {{ .Name }}!")

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatJSON, buf)

	err := r.Render(templatedData{
		Greeting: "Hello",
		Name:     "World",
		tmpl:     tmpl,
	})

	I.Expect(err).To(BeNil())
	// JSON renderer ignores Template(), uses struct tags
	I.Expect(buf.String()).To(ContainSubstring(`"greeting": "Hello"`))
	I.Expect(buf.String()).To(ContainSubstring(`"template_name": "World"`))
}

func TestRenderTemplatedTakesPrecedenceOverTabular(t *testing.T) {
	I := NewWithT(t)

	// A type that implements both Templated and Tabular
	type both struct {
		testData
		templatedData
	}

	tmpl := renderer.NewTemplate("test", "from template")

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatTable, buf)

	err := r.Render(both{
		testData:      testData{Name: "foo", Count: 1},
		templatedData: templatedData{tmpl: tmpl},
	})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(Equal("from template"))
}

func TestNewTemplateWithSharedFuncMap(t *testing.T) {
	I := NewWithT(t)

	// Register a shared func.
	original := renderer.FuncMap
	renderer.FuncMap = template.FuncMap{
		"shout": func(s string) string { return s + "!!!" },
	}

	defer func() { renderer.FuncMap = original }()

	tmpl := renderer.NewTemplate("test", "{{ shout .Name }}")

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatTable, buf)

	err := r.Render(templatedData{Name: "hello", tmpl: tmpl})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(Equal("hello!!!"))
}

func TestNewTemplateLocalOverridesShared(t *testing.T) {
	I := NewWithT(t)

	original := renderer.FuncMap
	renderer.FuncMap = template.FuncMap{
		"greet": func(s string) string { return "hi " + s },
	}

	defer func() { renderer.FuncMap = original }()

	// Local func overrides shared "greet".
	localFuncs := template.FuncMap{
		"greet": func(s string) string { return "HELLO " + s },
	}

	tmpl := renderer.NewTemplate("test", "{{ greet .Name }}", localFuncs)

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatTable, buf)

	err := r.Render(templatedData{Name: "world", tmpl: tmpl})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(Equal("HELLO world"))
}

func TestNewTemplateMultipleLocalFuncMaps(t *testing.T) {
	I := NewWithT(t)

	fm1 := template.FuncMap{
		"wrap": func(s string) string { return "[" + s + "]" },
	}
	fm2 := template.FuncMap{
		"upper": func(s string) string { return s + " (upper)" },
	}

	tmpl := renderer.NewTemplate("test", "{{ wrap .Name }} {{ upper .Name }}", fm1, fm2)

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatTable, buf)

	err := r.Render(templatedData{Name: "x", tmpl: tmpl})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(Equal("[x] x (upper)"))
}

func TestParseFormat(t *testing.T) {
	I := NewWithT(t)

	I.Expect(renderer.ParseFormat("json")).To(Equal(renderer.FormatJSON))
	I.Expect(renderer.ParseFormat("JSON")).To(Equal(renderer.FormatJSON))
	I.Expect(renderer.ParseFormat("plain")).To(Equal(renderer.FormatPlain))
	I.Expect(renderer.ParseFormat("table")).To(Equal(renderer.FormatTable))
	I.Expect(renderer.ParseFormat("")).To(Equal(renderer.FormatTable))
	I.Expect(renderer.ParseFormat("unknown")).To(Equal(renderer.FormatTable))
}

// --- YAML / TOML / CSV renderers ---

func TestRenderYAML(t *testing.T) {
	I := NewWithT(t)

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatYAML, buf)

	err := r.Render(testData{Name: "Acme", Count: 42})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(ContainSubstring("name: Acme"))
	I.Expect(buf.String()).To(ContainSubstring("count: 42"))
}

func TestRenderTOML(t *testing.T) {
	I := NewWithT(t)

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatTOML, buf)

	err := r.Render(testData{Name: "Acme", Count: 42})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(ContainSubstring("Name = 'Acme'"))
	I.Expect(buf.String()).To(ContainSubstring("Count = 42"))
}

func TestRenderCSV(t *testing.T) {
	I := NewWithT(t)

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatCSV, buf)

	err := r.Render(testData{Name: "Acme", Count: 42})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(ContainSubstring("count,name"))
	I.Expect(buf.String()).To(ContainSubstring("42,Acme"))
}

func TestRenderCSVSlice(t *testing.T) {
	I := NewWithT(t)

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatCSV, buf)

	err := r.Render([]testData{
		{Name: "Acme", Count: 42},
		{Name: "Beta", Count: 7},
	})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(ContainSubstring("count,name"))
	I.Expect(buf.String()).To(ContainSubstring("42,Acme"))
	I.Expect(buf.String()).To(ContainSubstring("7,Beta"))
}

func TestRenderCSVNestedFlattens(t *testing.T) {
	I := NewWithT(t)

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatCSV, buf)

	data := map[string]any{
		"name": "Acme",
		"address": map[string]any{
			"city":  "SF",
			"state": "CA",
		},
	}

	err := r.Render(data)

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(ContainSubstring("address.city"))
	I.Expect(buf.String()).To(ContainSubstring("SF"))
}

func TestParseFormatNewFormats(t *testing.T) {
	I := NewWithT(t)

	I.Expect(renderer.ParseFormat("yaml")).To(Equal(renderer.FormatYAML))
	I.Expect(renderer.ParseFormat("YAML")).To(Equal(renderer.FormatYAML))
	I.Expect(renderer.ParseFormat("toml")).To(Equal(renderer.FormatTOML))
	I.Expect(renderer.ParseFormat("csv")).To(Equal(renderer.FormatCSV))
}

func TestRenderNilTemplate(t *testing.T) {
	I := NewWithT(t)

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatTable, buf)

	data := templatedData{Name: "x", tmpl: nil}
	err := r.Render(data)

	I.Expect(err).To(HaveOccurred())
	I.Expect(err.Error()).To(ContainSubstring("nil template"))
}

// --- SharedFuncMap helpers ---

func TestFuncMapDash(t *testing.T) {
	I := NewWithT(t)

	tmpl := renderer.NewTemplate("test", `{{ dash .A }}|{{ dash .B }}|{{ dash .C }}`)

	buf := new(bytes.Buffer)
	r := renderer.NewWith(renderer.FormatTable, buf)

	s := "hello"
	err := r.Render(templatedData{
		tmpl: tmpl,
		Name: "test",
	})
	_ = s
	_ = err

	// Test dash directly via template execution
	tmpl2 := renderer.NewTemplate("dash", `{{ dash .Empty }}|{{ dash .Full }}|{{ dash .Nil }}`)
	buf2 := new(bytes.Buffer)

	err2 := tmpl2.Execute(buf2, map[string]any{
		"Empty": "",
		"Full":  "ok",
		"Nil":   nil,
	})

	I.Expect(err2).To(BeNil())
	I.Expect(buf2.String()).To(Equal("—|ok|—"))
}

func TestFuncMapTitle(t *testing.T) {
	I := NewWithT(t)

	tmpl := renderer.NewTemplate("test", `{{ title .S }}`)
	buf := new(bytes.Buffer)

	err := tmpl.Execute(buf, map[string]any{"S": "hello world"})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(Equal("Hello World"))
}

func TestFuncMapPadAndPlural(t *testing.T) {
	I := NewWithT(t)

	tmpl := renderer.NewTemplate("test", `{{ pad 10 .Name }}|{{ plural .Count "item" "items" }}`)
	buf := new(bytes.Buffer)

	err := tmpl.Execute(buf, map[string]any{"Name": "hi", "Count": 1})

	I.Expect(err).To(BeNil())
	I.Expect(buf.String()).To(Equal("hi        |item"))
}
