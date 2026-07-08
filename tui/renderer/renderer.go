package renderer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"maps"
	"sort"
	"strings"

	"github.com/jordanbrauer/hex/tui/markup"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Format represents an output format for command results.
type Format string

const (
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatTOML  Format = "toml"
	FormatCSV   Format = "csv"
	FormatTable Format = "table"
	FormatPlain Format = "plain"
)

// ParseFormat parses a format string, defaulting to FormatTable.
func ParseFormat(s string) Format {
	switch Format(strings.TrimSpace(strings.ToLower(s))) {
	case FormatJSON:
		return FormatJSON
	case FormatYAML:
		return FormatYAML
	case FormatTOML:
		return FormatTOML
	case FormatCSV:
		return FormatCSV
	case FormatPlain:
		return FormatPlain
	case FormatTable:
		return FormatTable
	default:
		return FormatTable
	}
}

// Renderer writes command output in a specific format.
type Renderer interface {
	Render(data any) error
	Format() Format
}

// New creates a Renderer from a cobra command, reading the --format flag and
// using the command's output writer for testability.
func New(cmd *cobra.Command) Renderer {
	f, _ := cmd.Flags().GetString("format")

	return NewWith(ParseFormat(f), cmd.OutOrStdout())
}

// NewWith creates a Renderer with explicit format and writer.
func NewWith(format Format, writer io.Writer) Renderer {
	switch format {
	case FormatJSON:
		return &JSONRenderer{writer: writer}
	case FormatYAML:
		return &YAMLRenderer{writer: writer}
	case FormatTOML:
		return &TOMLRenderer{writer: writer}
	case FormatCSV:
		return &CSVRenderer{writer: writer}
	default:
		return &TextRenderer{writer: writer, format: format}
	}
}

// Templated is an interface that types can implement to provide
// template-backed text output. The renderer executes the returned template
// against the data value itself. Templates should be pre-parsed with any
// required FuncMaps (see NewTemplate).
type Templated interface {
	Template() *template.Template
}

// Tabular is an interface that types can implement to provide styled text
// output. Used by both table and plain formats.
type Tabular interface {
	Table() string
}

// JSONRenderer writes data as pretty-printed JSON.
type JSONRenderer struct {
	writer io.Writer
}

func (r *JSONRenderer) Render(data any) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}

	_, err = fmt.Fprintln(r.writer, string(b))

	return err
}

func (r *JSONRenderer) Format() Format {
	return FormatJSON
}

// YAMLRenderer writes data as YAML.
type YAMLRenderer struct {
	writer io.Writer
}

func (r *YAMLRenderer) Render(data any) error {
	b, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal yaml: %w", err)
	}

	_, err = fmt.Fprint(r.writer, string(b))

	return err
}

func (r *YAMLRenderer) Format() Format {
	return FormatYAML
}

// TOMLRenderer writes data as TOML.
type TOMLRenderer struct {
	writer io.Writer
}

func (r *TOMLRenderer) Render(data any) error {
	b, err := toml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal toml: %w", err)
	}

	_, err = fmt.Fprint(r.writer, string(b))

	return err
}

func (r *TOMLRenderer) Format() Format {
	return FormatTOML
}

// CSVRenderer writes data as CSV. Nested values are flattened using dot-path
// keys (e.g. request.headers.Content-Type). Handles single objects, slices of
// objects, and arbitrary nesting.
type CSVRenderer struct {
	writer io.Writer
}

func (r *CSVRenderer) Render(data any) error {
	// Normalize to JSON-friendly types via round-trip. UseNumber
	// preserves numeric precision (avoids float64 conversion).
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal for csv: %w", err)
	}

	var raw any

	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()

	if err := dec.Decode(&raw); err != nil {
		return fmt.Errorf("failed to decode for csv: %w", err)
	}

	// Normalize to a slice of rows.
	var rows []map[string]string

	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			rows = append(rows, flatten("", item))
		}
	default:
		rows = append(rows, flatten("", v))
	}

	if len(rows) == 0 {
		return nil
	}

	// Collect and sort all keys across all rows.
	keySet := map[string]struct{}{}
	for _, row := range rows {
		for k := range row {
			keySet[k] = struct{}{}
		}
	}

	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	// Write CSV.
	w := csv.NewWriter(r.writer)

	if err := w.Write(keys); err != nil {
		return err
	}

	for _, row := range rows {
		record := make([]string, len(keys))
		for i, k := range keys {
			record[i] = row[k]
		}

		if err := w.Write(record); err != nil {
			return err
		}
	}

	w.Flush()

	return w.Error()
}

func (r *CSVRenderer) Format() Format {
	return FormatCSV
}

// flatten recursively converts a nested structure into a flat map with
// dot-separated keys.
func flatten(prefix string, v any) map[string]string {
	out := map[string]string{}

	key := func(k string) string {
		if prefix == "" {
			return k
		}

		return prefix + "." + k
	}

	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			for fk, fv := range flatten(key(k), child) {
				out[fk] = fv
			}
		}
	case []any:
		for i, child := range val {
			for fk, fv := range flatten(fmt.Sprintf("%s.%d", prefix, i), child) {
				out[fk] = fv
			}
		}
	case nil:
		out[prefix] = ""
	default:
		out[prefix] = fmt.Sprintf("%v", val)
	}

	return out
}

// TextRenderer writes data as text. If the data implements Tabular, it uses
// the Table() output. Otherwise falls back to fmt.Stringer or Fprintln.
// Used for both "table" and "plain" formats.
type TextRenderer struct {
	writer io.Writer
	format Format
}

func (r *TextRenderer) Render(data any) error {
	if t, ok := data.(Templated); ok {
		tmpl := t.Template()
		if tmpl == nil {
			return fmt.Errorf("render: Templated data returned nil template")
		}

		var buf bytes.Buffer

		if err := tmpl.Execute(&buf, data); err != nil {
			return err
		}

		_, err := fmt.Fprint(r.writer, markup.Parse(buf.String()))

		return err
	}

	if t, ok := data.(Tabular); ok {
		_, err := fmt.Fprint(r.writer, t.Table())

		return err
	}

	if s, ok := data.(fmt.Stringer); ok {
		_, err := fmt.Fprintln(r.writer, s.String())

		return err
	}

	_, err := fmt.Fprintln(r.writer, data)

	return err
}

func (r *TextRenderer) Format() Format {
	return r.format
}

// FuncMap contains shared template functions available to all templates
// created via NewTemplate. Commands can extend this with local functions.
var FuncMap = template.FuncMap{}

// NewTemplate parses a named template with the shared FuncMap merged with
// any local function maps. Local functions take precedence over shared ones.
//
//	var tmpl = renderer.NewTemplate("whoami", tmplSource, template.FuncMap{
//	    "hpad": func(rendered string, visible int) string { ... },
//	})
func NewTemplate(name, source string, localFuncs ...template.FuncMap) *template.Template {
	merged := make(template.FuncMap, len(FuncMap))
	maps.Copy(merged, FuncMap)

	for _, fm := range localFuncs {
		maps.Copy(merged, fm)
	}

	return template.Must(template.New(name).Funcs(merged).Parse(source))
}
