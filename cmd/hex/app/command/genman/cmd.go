// Package genman implements the hidden `hex gen-man` command.
package genman

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jordanbrauer/hex/cmd/hex/infrastructure/embedfs"
)

// New builds the hidden `hex gen-man` command. It regenerates the
// *generated* manpage markdown sources under docs/man/ (currently hex.1
// from the cobra tree and hex.3 from the Lua Teal stubs). The
// hand-authored pages (hex.5, hex.7) are not touched. `just man` runs
// this and then renders every docs/man/*.md to roff with pandoc.
//
// root is the already-assembled top-level command tree — passed in
// rather than rebuilt here so this package doesn't need to import back
// into app/command (which imports genman to wire it in).
func New(root *cobra.Command) *cobra.Command {
	var outDir string

	cmd := &cobra.Command{
		Use:    "gen-man",
		Short:  "Regenerate manpage markdown sources (internal)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if outDir == "" {
				outDir = filepath.Join("docs", "man")
			}

			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", outDir, err)
			}

			pages := []struct {
				name   string
				render func() (string, error)
			}{
				{"hex.1.md", func() (string, error) { return RenderHex1(root) }},
				{"hex.3.md", renderHex3},
			}

			for _, p := range pages {
				content, err := p.render()
				if err != nil {
					return err
				}

				target := filepath.Join(outDir, p.name)
				if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
					return fmt.Errorf("write %s: %w", target, err)
				}

				fmt.Fprintln(cmd.OutOrStdout(), "→", target)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&outDir, "out", "", "output directory for generated markdown (default docs/man)")

	return cmd
}

// RenderHex1 walks the live command tree and produces a single markdown
// source for the hex(1) manpage: a hand-authored preamble (NAME, SYNOPSIS,
// DESCRIPTION, CONVENTIONS) followed by an auto-generated COMMANDS section.
// Because it reads cmd.Long/cmd.Example off the tree, the page stays in
// sync with the CLI and its embedded help files.
//
// root is captured by the caller after every subcommand (including this
// hidden one) has been added, so by the time this runs the tree is
// complete.
func RenderHex1(root *cobra.Command) (string, error) {
	intro, err := embedfs.ManTemplate("hex.1.intro.md")
	if err != nil {
		return "", fmt.Errorf("read hex.1 intro: %w", err)
	}

	var b strings.Builder

	b.Write(intro)

	if !strings.HasSuffix(string(intro), "\n") {
		b.WriteByte('\n')
	}

	b.WriteString("\n# COMMANDS\n\n")

	for _, c := range root.Commands() {
		if skipInManpage(c) {
			continue
		}

		writeCommandSection(&b, c)

		for _, sub := range c.Commands() {
			if skipInManpage(sub) {
				continue
			}

			writeCommandSection(&b, sub)
		}
	}

	b.WriteString("# SEE ALSO\n\n")
	b.WriteString("**hex**(3) for the embedded Lua API, **hex**(5) for the config\n")
	b.WriteString("file formats, and **hex**(7) for the framework conventions guide.\n")

	return b.String(), nil
}

// skipInManpage reports whether a command should be omitted from the
// generated reference (hidden internals and Cobra's built-in helpers).
func skipInManpage(c *cobra.Command) bool {
	switch c.Name() {
	case "help", "completion":
		return true
	}

	return c.Hidden
}

// writeCommandSection appends one command's reference section: its long
// description, usage line, options (as a pandoc definition list), and
// examples.
func writeCommandSection(b *strings.Builder, c *cobra.Command) {
	fmt.Fprintf(b, "## %s\n\n", c.CommandPath())

	if long := strings.TrimSpace(c.Long); long != "" {
		b.WriteString(long)
		b.WriteString("\n\n")
	} else if c.Short != "" {
		b.WriteString(c.Short)
		b.WriteString("\n\n")
	}

	fmt.Fprintf(b, "Usage:\n\n```\n%s\n```\n\n", c.UseLine())

	var flags strings.Builder

	c.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}

		writeFlagDef(&flags, f)
	})

	if flags.Len() > 0 {
		b.WriteString("Options:\n\n")
		b.WriteString(flags.String())
	}

	if ex := strings.TrimSpace(c.Example); ex != "" {
		fmt.Fprintf(b, "Examples:\n\n```sh\n%s\n```\n\n", ex)
	}
}

// writeFlagDef renders one flag as a pandoc definition-list entry, which
// the man writer turns into a tagged paragraph (.TP).
func writeFlagDef(b *strings.Builder, f *pflag.Flag) {
	term := "`--" + f.Name + "`"
	if f.Shorthand != "" {
		term += ", `-" + f.Shorthand + "`"
	}

	if arg := f.Value.Type(); arg != "bool" {
		term += " *" + arg + "*"
	}

	usage := f.Usage
	if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
		usage += fmt.Sprintf(` (default "%s")`, f.DefValue)
	}

	fmt.Fprintf(b, "%s\n:   %s\n\n", term, usage)
}
