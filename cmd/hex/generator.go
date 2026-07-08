package main

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
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

// generator renders embedded templates into a target directory.
type generator struct {
	// dryRun logs what would be written but does not touch disk.
	dryRun bool
	// force overwrites existing files. Without it, existing files return
	// an error so consumers do not lose work.
	force bool
	// out is where informational messages go. Defaults to stdout.
	out *os.File
}

func newGenerator() *generator {
	return &generator{out: os.Stdout}
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
// set.
func (g *generator) write(target string, content []byte) error {
	if g.dryRun {
		fmt.Fprintf(g.out, "would write %s (%d bytes)\n", target, len(content))

		return nil
	}

	if _, err := os.Stat(target); err == nil && !g.force {
		return fmt.Errorf("refusing to overwrite %s (use --force)", target)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
	}

	if err := os.WriteFile(target, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}

	fmt.Fprintln(g.out, "→", target)

	return nil
}

// writeRaw drops raw bytes at target (no template rendering). Used for
// binary/asset files that would confuse text/template.
func (g *generator) writeRaw(target string, content []byte) error {
	return g.write(target, content)
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

	markerLine := -1

	indent := ""

	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trimmed, marker) {
			continue
		}

		markerLine = i
		indent = line[:len(line)-len(trimmed)]

		break
	}

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
