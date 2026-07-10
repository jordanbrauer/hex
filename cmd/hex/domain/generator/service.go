package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	hexerrors "github.com/jordanbrauer/hex/errors"
)

// funcMap is available inside every template a Blueprint renders.
var funcMap = template.FuncMap{
	"lower":      strings.ToLower,
	"upper":      strings.ToUpper,
	"title":      TitleCase,
	"pascal":     PascalCase,
	"camel":      CamelCase,
	"snake":      SnakeCase,
	"pluralise":  Pluralise,
	"pluralize":  Pluralise, // alias for US spelling
	"go_package": GoPackageName,
}

// Options configures how Service applies a Blueprint.
type Options struct {
	// DryRun records what would be written but does not touch disk.
	DryRun bool
	// Force overwrites existing files. Without it, an existing target
	// is skipped (DryRun) or refused (real run).
	Force bool
}

// Service applies Blueprints — rendering their Files and wiring their
// Wires — against a target project root. It holds no per-invocation
// state; every call takes the Options it needs and returns the actions
// it took (or, under DryRun, would take).
type Service struct {
	repo Repository
}

// NewService wires the Repository into a Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Run looks up the named blueprint and applies it. See Apply.
func (s *Service) Run(ctx context.Context, blueprintID, root string, data any, opts Options) ([]Action, error) {
	bp, err := s.repo.Get(ctx, blueprintID)
	if err != nil {
		return nil, err
	}

	return s.Apply(ctx, bp, root, data, opts)
}

// Apply renders bp's Files and applies its Wires against root, evaluating
// every template string (Target, File, Marker, Insertion) against data.
// Commands that need conditional files/wires beyond a static Blueprint
// (make:controller's route wiring, make:command's group logic) call
// RenderFile/WireMarker/WireImport/PromoteImport directly instead.
func (s *Service) Apply(ctx context.Context, bp *Blueprint, root string, data any, opts Options) ([]Action, error) {
	var actions []Action

	for _, f := range bp.Files {
		target, err := renderString(f.Target, data)
		if err != nil {
			return actions, fmt.Errorf("render target for %s: %w", f.Template, err)
		}

		act, err := s.RenderFile(ctx, f.Template, filepath.Join(root, target), data, opts)
		if err != nil {
			return actions, err
		}

		actions = append(actions, act)
	}

	for _, w := range bp.Wires {
		file, err := renderString(w.File, data)
		if err != nil {
			return actions, err
		}

		insertion, err := renderString(w.Insertion, data)
		if err != nil {
			return actions, err
		}

		detail, err := renderString(w.Detail, data)
		if err != nil {
			return actions, err
		}

		act, err := s.WireMarker(filepath.Join(root, file), w.Marker, insertion, detail, opts)
		if err != nil {
			return actions, err
		}

		if act != nil {
			actions = append(actions, *act)
		}
	}

	return actions, nil
}

// renderString evaluates a Go template string against data, used for
// Blueprint Target/File/Marker/Insertion fields.
func renderString(text string, data any) (string, error) {
	tpl, err := template.New("").Funcs(funcMap).Parse(text)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// RenderFile loads templatePath from the Repository, executes it with
// data, and writes the result to target.
func (s *Service) RenderFile(ctx context.Context, templatePath, target string, data any, opts Options) (Action, error) {
	source, err := s.repo.Read(ctx, templatePath)
	if err != nil {
		return Action{}, fmt.Errorf("read template %s: %w", templatePath, err)
	}

	tpl, err := template.New(filepath.Base(templatePath)).Funcs(funcMap).Parse(string(source))
	if err != nil {
		return Action{}, fmt.Errorf("parse template %s: %w", templatePath, err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return Action{}, fmt.Errorf("execute template %s: %w", templatePath, err)
	}

	return writeFile(target, buf.Bytes(), opts)
}

// WriteRaw persists content to target verbatim (no template rendering).
// Used to publish framework-shipped config files unchanged.
func (s *Service) WriteRaw(target string, content []byte, opts Options) (Action, error) {
	return writeFile(target, content, opts)
}

// Publish copies srcPath from srcFS to target verbatim.
func (s *Service) Publish(srcFS fs.FS, srcPath, target string, opts Options) (Action, error) {
	data, err := fs.ReadFile(srcFS, srcPath)
	if err != nil {
		return Action{}, fmt.Errorf("publish read %s: %w", srcPath, err)
	}

	return writeFile(target, data, opts)
}

// PublishAll copies every file whose name matches suffix from srcFS's
// root into targetDir.
func (s *Service) PublishAll(srcFS fs.FS, suffix, targetDir string, opts Options) ([]Action, error) {
	entries, err := fs.ReadDir(srcFS, ".")
	if err != nil {
		return nil, fmt.Errorf("publishAll read dir: %w", err)
	}

	var actions []Action

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if suffix != "" && !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}

		target := filepath.Join(targetDir, entry.Name())

		act, err := s.Publish(srcFS, entry.Name(), target, opts)
		if err != nil {
			return actions, err
		}

		actions = append(actions, act)
	}

	return actions, nil
}

// writeFile persists content to target unless opts.DryRun is set.
// Missing parent directories are created. Existing files are refused
// unless opts.Force is set.
func writeFile(target string, content []byte, opts Options) (Action, error) {
	_, statErr := os.Stat(target)
	exists := statErr == nil

	if opts.DryRun {
		switch {
		case exists && !opts.Force:
			return Action{Kind: "skip", Path: target, Detail: "exists; use --force"}, nil
		case exists:
			return Action{Kind: "overwrite", Path: target}, nil
		default:
			return Action{Kind: "create", Path: target}, nil
		}
	}

	if exists && !opts.Force {
		return Action{}, hexerrors.Wrap(nil, ErrTargetExists, "refusing to overwrite %s (use --force)", target)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Action{}, fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
	}

	if err := os.WriteFile(target, content, 0o644); err != nil {
		return Action{}, fmt.Errorf("write %s: %w", target, err)
	}

	if exists {
		return Action{Kind: "overwrite", Path: target}, nil
	}

	return Action{Kind: "create", Path: target}, nil
}

// Mkdirp creates dir (and parents) unless it already exists. Returns nil
// when dir already exists — a silent no-op, nothing to record.
func (s *Service) Mkdirp(dir string, opts Options) (*Action, error) {
	if _, err := os.Stat(dir); err == nil {
		return nil, nil
	}

	if opts.DryRun {
		return &Action{Kind: "mkdir", Path: dir}, nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	return &Action{Kind: "mkdir", Path: dir}, nil
}

// WireMarker inserts insertion above marker in path, recording detail
// (e.g. "added Payments") against the resulting action. Idempotent: if
// insertion (trimmed) already appears in path, it records a "skip"
// action instead of inserting again. Under opts.DryRun it validates
// read-only that the marker exists and does not touch disk.
func (s *Service) WireMarker(path, marker, insertion, detail string, opts Options) (*Action, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	already := bytes.Contains(source, bytes.TrimSpace([]byte(insertion)))

	if opts.DryRun {
		if already {
			return &Action{Kind: "skip", Path: path, Detail: "already wired"}, nil
		}

		if idx, _ := markerLineIndex(strings.Split(string(source), "\n"), marker); idx < 0 {
			return nil, fmt.Errorf("marker %q not found as a bare line in %s", marker, path)
		}

		return &Action{Kind: "wire", Path: path, Detail: detail}, nil
	}

	if err := insertBeforeMarker(path, marker, insertion); err != nil {
		return nil, err
	}

	if already {
		return &Action{Kind: "skip", Path: path, Detail: "already wired"}, nil
	}

	return &Action{Kind: "wire", Path: path, Detail: detail}, nil
}

// WireImport adds importPath to path's import block if not already
// present. Returns a nil Action when the import already exists — a
// silent, idempotent no-op.
func (s *Service) WireImport(path, importPath string, opts Options) (*Action, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if bytes.Contains(source, []byte(`"`+importPath+`"`)) {
		return nil, nil // already imported
	}

	if opts.DryRun {
		return &Action{Kind: "wire", Path: path, Detail: "import " + importPath}, nil
	}

	if err := addImport(path, importPath); err != nil {
		return nil, err
	}

	return &Action{Kind: "wire", Path: path, Detail: "import " + importPath}, nil
}

// PromoteImport rewrites a blank import (`_ "x"`) in file to a real one.
// Returns a nil Action when there was nothing to promote.
func (s *Service) PromoteImport(file, importPath string, opts Options) (*Action, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}

	if !bytes.Contains(data, []byte(fmt.Sprintf(`_ %q`, importPath))) {
		return nil, nil // already promoted or never blank
	}

	if opts.DryRun {
		return &Action{Kind: "wire", Path: file, Detail: "promote import " + importPath}, nil
	}

	if err := promoteBlankImport(file, importPath); err != nil {
		return nil, err
	}

	return &Action{Kind: "wire", Path: file, Detail: "promote import " + importPath}, nil
}

// Report writes actions to w: one JSON object when format == "json",
// otherwise one human-readable line per action.
func Report(w io.Writer, actions []Action, dryRun bool, format string) error {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")

		return enc.Encode(struct {
			DryRun  bool     `json:"dry_run"`
			Actions []Action `json:"actions"`
		}{DryRun: dryRun, Actions: actions})
	}

	for _, a := range actions {
		printAction(w, a, dryRun)
	}

	return nil
}

// printAction renders one action as a human-readable line.
func printAction(w io.Writer, a Action, dryRun bool) {
	label := a.Path

	switch a.Kind {
	case "wire", "skip":
		if a.Detail != "" {
			label = fmt.Sprintf("%s (%s)", a.Path, a.Detail)
		}
	case "mkdir":
		label = a.Path + string(os.PathSeparator)
	}

	if dryRun {
		fmt.Fprintf(w, "would %s %s\n", a.Kind, label)

		return
	}

	if a.Kind == "skip" {
		fmt.Fprintln(w, "•", label)

		return
	}

	fmt.Fprintln(w, "→", label)
}

// markerLineIndex finds the first line whose first non-whitespace token
// is marker, returning its index. Returns -1 when absent. The bare-line
// requirement prevents false matches against doc-comment references to
// the marker.
func markerLineIndex(lines []string, marker string) (int, string) {
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, marker) {
			return i, line[:len(line)-len(trimmed)]
		}
	}

	return -1, ""
}

// insertBeforeMarker finds marker in path's file and inserts insertion on
// a new line before it. Idempotent: skips insertion when the exact text
// is already present.
func insertBeforeMarker(path, marker, insertion string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	if bytes.Contains(source, bytes.TrimSpace([]byte(insertion))) {
		return nil
	}

	lines := strings.Split(string(source), "\n")

	markerLine, indent := markerLineIndex(lines, marker)
	if markerLine < 0 {
		return fmt.Errorf("marker %q not found as a bare line in %s", marker, path)
	}

	block := indent + strings.TrimLeft(insertion, "\r\n\t ")

	out := append([]string{}, lines[:markerLine]...)
	out = append(out, block)
	out = append(out, lines[markerLine:]...)

	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}

// addImport inserts importPath into path's import block if not already
// present. Simple heuristic: locate the first `import (` line and the
// next `)`; insert a tab-indented line above the closing paren. `go fmt`
// cleans up the shape afterward.
func addImport(path, importPath string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	src := string(source)

	if strings.Contains(src, `"`+importPath+`"`) {
		return nil // already imported
	}

	openIdx := strings.Index(src, "import (")
	if openIdx < 0 {
		return fmt.Errorf("import block not found in %s", path)
	}

	closeIdx := strings.Index(src[openIdx:], ")")
	if closeIdx < 0 {
		return fmt.Errorf("import block not closed in %s", path)
	}

	closeIdx += openIdx

	lineStart := strings.LastIndex(src[:closeIdx], "\n")
	if lineStart < 0 {
		lineStart = openIdx
	} else {
		lineStart++
	}

	line := "\t\"" + importPath + "\"\n"
	out := src[:lineStart] + line + src[lineStart:]

	return os.WriteFile(path, []byte(out), 0o644)
}

// promoteBlankImport rewrites the underscore-blank form of an import into
// a normal import so name references compile. Idempotent.
func promoteBlankImport(file, importPath string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read %s: %w", file, err)
	}

	blank := fmt.Sprintf(`_ %q`, importPath)
	real_ := fmt.Sprintf(`%q`, importPath)

	if !bytes.Contains(data, []byte(blank)) {
		return nil // already promoted or never blank
	}

	out := bytes.ReplaceAll(data, []byte(blank), []byte(real_))

	return os.WriteFile(file, out, 0o644)
}
