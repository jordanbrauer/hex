package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// tlField is one field of a Teal record: a name and its (raw) type or
// function signature, verbatim from the .d.tl source.
type tlField struct {
	Name string
	Type string
}

// tlRecord is a `local record <Name> … end` block.
type tlRecord struct {
	Name   string
	Fields []tlField
}

// tlModule is a parsed Lua module type stub (`<pkg>/lua/<key>.d.tl`).
type tlModule struct {
	Key     string     // require key (the file's base name, e.g. "db")
	Desc    string     // leading `--` comment block
	Records []tlRecord // every record declared in the file
	Returns string     // the record name the module returns (its surface)
}

// parseTealStub parses a `.d.tl` type stub. The stubs are deliberately
// regular (leading comment, `local record … end` blocks, a `return`), so a
// line-based parser is sufficient and keeps hex.3 in sync with the real
// module surface.
func parseTealStub(path string) (tlModule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return tlModule{}, fmt.Errorf("read %s: %w", path, err)
	}

	m := tlModule{Key: strings.TrimSuffix(filepath.Base(path), ".d.tl")}

	var (
		desc     []string
		cur      tlRecord
		building bool
		inHeader = true
	)

	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)

		switch {
		case !building && strings.HasPrefix(t, "--"):
			if inHeader {
				desc = append(desc, strings.TrimSpace(strings.TrimPrefix(t, "--")))
			}

		case strings.HasPrefix(t, "local record "):
			inHeader = false
			cur = tlRecord{Name: strings.TrimSpace(strings.TrimPrefix(t, "local record "))}
			building = true

		case building && t == "end":
			m.Records = append(m.Records, cur)
			building = false

		case building && t != "":
			if i := strings.Index(t, ":"); i > 0 {
				cur.Fields = append(cur.Fields, tlField{
					Name: strings.TrimSpace(t[:i]),
					Type: strings.TrimSpace(t[i+1:]),
				})
			}

		case strings.HasPrefix(t, "return "):
			m.Returns = strings.TrimSpace(strings.TrimPrefix(t, "return "))
		}
	}

	m.Desc = strings.TrimSpace(strings.Join(desc, " "))

	return m, nil
}

// renderHex3 builds the markdown source for the hex(3) manpage from a
// hand-authored preamble plus the parsed Lua type stubs. Function
// signatures are emitted verbatim in Teal notation.
func renderHex3() (string, error) {
	intro, err := manTemplatesFS.ReadFile("mantemplates/hex.3.intro.md")
	if err != nil {
		return "", fmt.Errorf("read hex.3 intro: %w", err)
	}

	files, err := filepath.Glob(filepath.Join("*", "lua", "*.d.tl"))
	if err != nil {
		return "", fmt.Errorf("glob teal stubs: %w", err)
	}

	sort.Strings(files)

	var b strings.Builder

	b.Write(intro)

	if !strings.HasSuffix(string(intro), "\n") {
		b.WriteByte('\n')
	}

	b.WriteString("\n# MODULES\n\n")

	// auxiliary records (types referenced by the modules) are collected
	// and emitted together under TYPES.
	type auxType struct {
		module string
		rec    tlRecord
	}

	var aux []auxType

	for _, f := range files {
		m, err := parseTealStub(f)
		if err != nil {
			return "", err
		}

		fmt.Fprintf(&b, "## %s\n\n", m.Key)

		if m.Desc != "" {
			b.WriteString(m.Desc)
			b.WriteString("\n\n")
		}

		for _, r := range m.Records {
			if r.Name != m.Returns {
				aux = append(aux, auxType{module: m.Key, rec: r})

				continue
			}

			for _, fld := range r.Fields {
				fmt.Fprintf(&b, "`%s.%s`\n:   `%s`\n\n", m.Key, fld.Name, fld.Type)
			}
		}
	}

	if len(aux) > 0 {
		b.WriteString("# TYPES\n\n")

		for _, a := range aux {
			fmt.Fprintf(&b, "## %s\n\n", a.rec.Name)
			fmt.Fprintf(&b, "A table used by the `%s` module.\n\n", a.module)

			for _, fld := range a.rec.Fields {
				fmt.Fprintf(&b, "`%s`\n:   `%s`\n\n", fld.Name, fld.Type)
			}
		}
	}

	b.WriteString("# ENVIRONMENT\n\n")
	b.WriteString("*hex reads no environment variables of its own.* Application")
	b.WriteString(" configuration is supplied through config files and, in scaffolded")
	b.WriteString(" apps, a declarative env map — see **hex**(5).\n\n")

	b.WriteString("# SEE ALSO\n\n**hex**(1), **hex**(5), **hex**(7)\n")

	return b.String(), nil
}
