package config

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
)

// schemaFile is the conventional top-level schema filename. When present,
// its top-level fields describe each namespace's constraints and are
// unified with any per-namespace <namespace>.cue file that also exists.
const schemaFile = "schema.cue"

// schemaSet holds per-namespace validators derived from schema.cue and/or
// <namespace>.cue files.
type schemaSet struct {
	ctx      *cue.Context
	perNS    map[string]cue.Value
	fromFile map[string]string // namespace -> filename that contributed (for error messages)
}

// loadSchemas walks the config FS looking for a top-level schema.cue and
// per-namespace <namespace>.cue files. Returns a schemaSet with one
// entry per namespace that has any schema material.
//
// The set of TOML-declared namespaces is passed in so schemas for
// unknown namespaces (typos, orphaned files) return an error rather
// than being silently ignored.
func loadSchemas(fsys fs.FS, dir string, tomlNamespaces map[string]bool) (*schemaSet, error) {
	if fsys == nil {
		return nil, nil
	}

	ctx := cuecontext.New()
	set := &schemaSet{
		ctx:      ctx,
		perNS:    make(map[string]cue.Value),
		fromFile: make(map[string]string),
	}

	// Load top-level schema.cue and split it into per-namespace values.
	if data, err := readIfExists(fsys, joinPath(dir, schemaFile)); err != nil {
		return nil, fmt.Errorf("config: read %s: %w", schemaFile, err)
	} else if data != nil {
		v := ctx.CompileBytes(data, cue.Filename(schemaFile))
		if err := v.Err(); err != nil {
			return nil, fmt.Errorf("config: compile %s: %w", schemaFile, err)
		}

		iter, err := v.Fields(cue.Optional(true), cue.Definitions(false))
		if err != nil {
			return nil, fmt.Errorf("config: iterate %s: %w", schemaFile, err)
		}

		for iter.Next() {
			ns := iter.Selector().String()
			set.perNS[ns] = iter.Value()
			set.fromFile[ns] = schemaFile
		}
	}

	// Load per-namespace <ns>.cue files, unifying with anything schema.cue
	// contributed.
	scanDir := dir
	if scanDir == "" {
		scanDir = "."
	}

	entries, err := fs.ReadDir(fsys, scanDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return set, nil
		}

		return nil, fmt.Errorf("config: read schema dir: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".cue") || name == schemaFile {
			continue
		}

		ns := strings.TrimSuffix(name, ".cue")

		data, err := fs.ReadFile(fsys, joinPath(dir, name))
		if err != nil {
			return nil, fmt.Errorf("config: read %s: %w", name, err)
		}

		v := ctx.CompileBytes(data, cue.Filename(name))
		if err := v.Err(); err != nil {
			return nil, fmt.Errorf("config: compile %s: %w", name, err)
		}

		if existing, ok := set.perNS[ns]; ok {
			set.perNS[ns] = existing.Unify(v)
			set.fromFile[ns] = fmt.Sprintf("%s+%s", set.fromFile[ns], name)
		} else {
			set.perNS[ns] = v
			set.fromFile[ns] = name
		}
	}

	// Warn (via error return path) on schemas for namespaces we did not
	// find TOML for — likely a typo. Only surface when tomlNamespaces
	// was supplied; when nil, callers do not want this check.
	if tomlNamespaces != nil {
		for ns := range set.perNS {
			if !tomlNamespaces[ns] {
				return set, fmt.Errorf("config: schema %s references namespace %q with no matching TOML file", set.fromFile[ns], ns)
			}
		}
	}

	return set, nil
}

// hasSchema reports whether a schema exists for namespace.
func (s *schemaSet) hasSchema(ns string) bool {
	if s == nil {
		return false
	}

	_, ok := s.perNS[ns]

	return ok
}

// validate runs the namespace's schema against values. Returns nil when
// no schema is registered for the namespace (silent skip is the caller's
// job when StrictValidation is off; strict callers pre-check with
// hasSchema).
func (s *schemaSet) validate(ns string, values map[string]any) error {
	if s == nil {
		return nil
	}

	schema, ok := s.perNS[ns]
	if !ok {
		return nil
	}

	// Encode the merged runtime values into a CUE value and unify with
	// the schema. Concrete=true forces all constraints down to real
	// values (catching type mismatches / missing required fields).
	data := s.ctx.Encode(values)
	if err := data.Err(); err != nil {
		return fmt.Errorf("encode values: %w", err)
	}

	unified := schema.Unify(data)
	if err := unified.Err(); err != nil {
		return formatSchemaError(ns, s.fromFile[ns], err)
	}

	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return formatSchemaError(ns, s.fromFile[ns], err)
	}

	return nil
}

// schemaValue returns the raw cue.Value for a namespace or the zero
// value when none is registered. Exposed via Store.Schema for consumers
// who want to introspect the schema (docs, tooling).
func (s *schemaSet) schemaValue(ns string) cue.Value {
	if s == nil {
		return cue.Value{}
	}

	return s.perNS[ns]
}

// formatSchemaError renders CUE's multi-error into a hex-friendly
// message that names the namespace, source file, and every violation.
func formatSchemaError(ns, filename string, err error) error {
	lines := []string{
		fmt.Sprintf("config: %s: schema validation failed (%s)", ns, filename),
	}

	for _, e := range cueerrors.Errors(err) {
		msg := fmt.Sprintf("  - %s", e)

		if pos := e.Position(); pos.Filename() != "" {
			msg = fmt.Sprintf("  - %s:%d: %s", pos.Filename(), pos.Line(), e)
		}

		lines = append(lines, msg)
	}

	return errors.New(strings.Join(lines, "\n"))
}

// readIfExists reads path from fsys or returns (nil, nil) if the file is
// missing. All other errors are returned verbatim.
func readIfExists(fsys fs.FS, path string) ([]byte, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	return data, nil
}

// joinPath uses forward slashes for fs.FS lookups (Windows callers still
// pass "/" separators for embed).
func joinPath(dir, name string) string {
	if dir == "" || dir == "." {
		return name
	}

	return filepath.ToSlash(filepath.Join(dir, name))
}
