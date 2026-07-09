package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

//go:embed all:templates
var templatesFS embed.FS

// funcMap is available inside every template rendered by generator.
var funcMap = template.FuncMap{
	"lower":      strings.ToLower,
	"upper":      strings.ToUpper,
	"title":      titleCase,
	"pascal":     pascalCase,
	"camel":      camelCase,
	"snake":      snakeCase,
	"pluralise":  pluralise,
	"pluralize":  pluralise, // alias for US spelling
	"go_package": goPackageName,
}

// action records one filesystem effect a generator produced (or, under
// dryRun, would produce). Actions accumulate in memory for the lifetime of
// a single command and are flushed by report — nothing is persisted to disk.
type action struct {
	// Kind is one of: create, overwrite, wire, skip, mkdir.
	Kind string `json:"kind"`
	// Path is the file or directory the action targets.
	Path string `json:"path"`
	// Detail is optional context (e.g. the symbol wired, or why a write
	// was skipped).
	Detail string `json:"detail,omitempty"`
}

// generator renders embedded templates into a target directory and records
// every effect it has as an action, so the same run can be previewed
// (dryRun) or reported as structured output (format == "json").
type generator struct {
	// dryRun records what would be written but does not touch disk.
	dryRun bool
	// force overwrites existing files. Without it, existing files return
	// an error so consumers do not lose work.
	force bool
	// format selects how report renders the action log: "text" or "json".
	format string
	// out is where report and progress messages go. Defaults to stdout.
	out *os.File
	// actions is the in-memory log of everything this generator did.
	actions []action
}

func newGenerator() *generator {
	return &generator{out: os.Stdout, format: "text"}
}

// record appends an action to the in-memory log.
func (g *generator) record(kind, path, detail string) {
	g.actions = append(g.actions, action{Kind: kind, Path: path, Detail: detail})
}

// render loads a template from the embedded FS, executes it with data,
// and writes the result to path.
func (g *generator) render(templatePath, targetPath string, data any) error {
	source, err := fs.ReadFile(templatesFS, templatePath)
	if err != nil {
		return fmt.Errorf("read template %s: %w", templatePath, err)
	}

	tpl, err := template.New(filepath.Base(templatePath)).Funcs(funcMap).Parse(string(source))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", templatePath, err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template %s: %w", templatePath, err)
	}

	return g.write(targetPath, buf.Bytes())
}

// write persists content to path unless dryRun is set. Missing parent
// directories are created. Existing files are refused unless force is
// set. Every outcome is recorded as an action.
func (g *generator) write(target string, content []byte) error {
	_, statErr := os.Stat(target)
	exists := statErr == nil

	if g.dryRun {
		switch {
		case exists && !g.force:
			g.record("skip", target, "exists; use --force")
		case exists:
			g.record("overwrite", target, "")
		default:
			g.record("create", target, "")
		}

		return nil
	}

	if exists && !g.force {
		return fmt.Errorf("refusing to overwrite %s (use --force)", target)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
	}

	if err := os.WriteFile(target, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}

	if exists {
		g.record("overwrite", target, "")
	} else {
		g.record("create", target, "")
	}

	return nil
}

// writeRaw drops raw bytes at target (no template rendering). Used for
// binary/asset files that would confuse text/template.
func (g *generator) writeRaw(target string, content []byte) error {
	return g.write(target, content)
}

// mkdirp creates dir (and parents) unless it already exists, recording a
// mkdir action. A no-op — recording nothing — when dir already exists.
func (g *generator) mkdirp(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return nil
	}

	if g.dryRun {
		g.record("mkdir", dir, "")

		return nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	g.record("mkdir", dir, "")

	return nil
}

// wireMarker inserts insertion above marker in path (see insertBeforeMarker),
// recording a wire action. Under dryRun it validates read-only that the
// marker exists and does not touch disk. detail labels the wired symbol for
// reporting.
func (g *generator) wireMarker(path, marker, insertion, detail string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	already := bytes.Contains(source, bytes.TrimSpace([]byte(insertion)))

	if g.dryRun {
		if already {
			g.record("skip", path, "already wired")

			return nil
		}

		if idx, _ := markerLineIndex(strings.Split(string(source), "\n"), marker); idx < 0 {
			return fmt.Errorf("marker %q not found as a bare line in %s", marker, path)
		}

		g.record("wire", path, detail)

		return nil
	}

	if err := insertBeforeMarker(path, marker, insertion); err != nil {
		return err
	}

	if already {
		g.record("skip", path, "already wired")
	} else {
		g.record("wire", path, detail)
	}

	return nil
}

// wireImport adds importPath to path's import block (see addImport),
// recording a wire action. Idempotent and dryRun-aware.
func (g *generator) wireImport(path, importPath string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	if bytes.Contains(source, []byte(`"`+importPath+`"`)) {
		return nil // already imported
	}

	if g.dryRun {
		g.record("wire", path, "import "+importPath)

		return nil
	}

	if err := addImport(path, importPath); err != nil {
		return err
	}

	g.record("wire", path, "import "+importPath)

	return nil
}

// promoteImport rewrites a blank import (`_ "x"`) to a real one in file
// (see promoteBlankImport), recording a wire action. Idempotent and
// dryRun-aware.
func (g *generator) promoteImport(file, importPath string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read %s: %w", file, err)
	}

	if !bytes.Contains(data, []byte(fmt.Sprintf(`_ %q`, importPath))) {
		return nil // already promoted or never blank
	}

	if g.dryRun {
		g.record("wire", file, "promote import "+importPath)

		return nil
	}

	if err := promoteBlankImport(file, importPath); err != nil {
		return err
	}

	g.record("wire", file, "promote import "+importPath)

	return nil
}

// report flushes the action log to out. In "json" format it emits a single
// object; otherwise it prints one human-readable line per action.
func (g *generator) report() error {
	if g.format == "json" {
		enc := json.NewEncoder(g.out)
		enc.SetIndent("", "  ")

		return enc.Encode(struct {
			DryRun  bool     `json:"dry_run"`
			Actions []action `json:"actions"`
		}{DryRun: g.dryRun, Actions: g.actions})
	}

	for _, a := range g.actions {
		g.printAction(a)
	}

	return nil
}

// printAction renders one action as a human-readable line.
func (g *generator) printAction(a action) {
	label := a.Path

	switch a.Kind {
	case "wire":
		if a.Detail != "" {
			label = fmt.Sprintf("%s (%s)", a.Path, a.Detail)
		}
	case "skip":
		if a.Detail != "" {
			label = fmt.Sprintf("%s (%s)", a.Path, a.Detail)
		}
	case "mkdir":
		label = a.Path + string(os.PathSeparator)
	}

	if g.dryRun {
		fmt.Fprintf(g.out, "would %s %s\n", a.Kind, label)

		return
	}

	if a.Kind == "skip" {
		fmt.Fprintln(g.out, "•", label)

		return
	}

	fmt.Fprintln(g.out, "→", label)
}

// publish copies srcPath from srcFS to target verbatim. Used to
// materialise framework-shipped config files (from a provider's
// embedded Configs()) into the consumer's config/ dir at scaffold time.
func (g *generator) publish(srcFS fs.FS, srcPath, target string) error {
	data, err := fs.ReadFile(srcFS, srcPath)
	if err != nil {
		return fmt.Errorf("publish read %s: %w", srcPath, err)
	}

	return g.writeRaw(target, data)
}

// publishAll copies every file whose name matches suffix (e.g. ".toml")
// from srcFS's root into targetDir. Returns the number of files copied.
// Used for bulk publishing when a provider ships multiple assets.
func (g *generator) publishAll(srcFS fs.FS, suffix, targetDir string) (int, error) {
	entries, err := fs.ReadDir(srcFS, ".")
	if err != nil {
		return 0, fmt.Errorf("publishAll read dir: %w", err)
	}

	n := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if suffix != "" && !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}

		target := filepath.Join(targetDir, entry.Name())
		if err := g.publish(srcFS, entry.Name(), target); err != nil {
			return n, err
		}

		n++
	}

	return n, nil
}

// genFlags carries the flags shared by every generating command.
type genFlags struct {
	force  bool
	dryRun bool
	format string
}

// addGeneratorFlags registers the standard generator flags on cmd. Every
// `hex init` / `hex make:*` command uses this so the surface stays uniform
// and agent-legible.
func addGeneratorFlags(cmd *cobra.Command, f *genFlags) {
	cmd.Flags().BoolVar(&f.force, "force", false, "overwrite existing files")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "print the actions without writing any files")
	cmd.Flags().StringVar(&f.format, "format", "text", "output format: text or json")
}

// newGeneratorFromFlags builds a generator configured from f, validating
// the requested output format.
func newGeneratorFromFlags(f genFlags) (*generator, error) {
	switch f.format {
	case "", "text", "json":
	default:
		return nil, fmt.Errorf("unknown --format %q (want text or json)", f.format)
	}

	g := newGenerator()
	g.force = f.force
	g.dryRun = f.dryRun

	if f.format != "" {
		g.format = f.format
	}

	return g, nil
}

// markerLineIndex finds the first line whose first non-whitespace token is
// marker, returning its index and leading indentation. Returns (-1, "")
// when absent. The bare-line requirement prevents false matches against
// doc-comment references to the marker.
func markerLineIndex(lines []string, marker string) (int, string) {
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, marker) {
			return i, line[:len(line)-len(trimmed)]
		}
	}

	return -1, ""
}

// insertBeforeMarker finds marker in path's file and inserts insertion
// on a new line before it. Idempotent: skips insertion when the exact
// text is already present.
//
// The marker must appear as the first non-whitespace token on its line —
// this prevents false matches against doc-comment references to the
// marker (e.g. "insert above the `// hex:providers` line").
//
// This is how hex generators auto-wire new providers, commands, etc.
// without touching AST.
func insertBeforeMarker(path, marker, insertion string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	// Idempotency: if `insertion` (trimmed) already exists in the file,
	// don't add it again.
	if bytes.Contains(source, bytes.TrimSpace([]byte(insertion))) {
		return nil
	}

	lines := strings.Split(string(source), "\n")

	markerLine, indent := markerLineIndex(lines, marker)
	if markerLine < 0 {
		return fmt.Errorf("marker %q not found as a bare line in %s", marker, path)
	}

	block := indent + strings.TrimLeft(insertion, "\r\n\t ")

	// Insert block above the marker line, preserving the trailing lines.
	out := append([]string{}, lines[:markerLine]...)
	out = append(out, block)
	out = append(out, lines[markerLine:]...)

	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}

// projectRoot walks up from cwd until it finds a go.mod file. Returns the
// absolute directory containing go.mod plus the module path.
func projectRoot() (dir, modulePath string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}

	current := cwd

	for {
		gomod := filepath.Join(current, "go.mod")
		if data, err := os.ReadFile(gomod); err == nil {
			mod, mErr := parseModulePath(data)
			if mErr != nil {
				return "", "", mErr
			}

			return current, mod, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", "", errors.New("go.mod not found (are you inside a hex project?)")
		}

		current = parent
	}
}

// parseModulePath extracts the module path from go.mod bytes.
func parseModulePath(data []byte) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "module")), nil
		}
	}

	return "", errors.New("module directive not found in go.mod")
}
