package command

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/container"

	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
)

// genFlags carries the flags shared by every generating command.
type genFlags struct {
	force  bool
	dryRun bool
	format string
}

// addGeneratorFlags registers the standard generator flags on cmd. Every
// `hex init` / `hex make:*` command uses this so the surface stays
// uniform and agent-legible.
func addGeneratorFlags(cmd *cobra.Command, f *genFlags) {
	cmd.Flags().BoolVar(&f.force, "force", false, "overwrite existing files")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "print the actions without writing any files")
	cmd.Flags().StringVar(&f.format, "format", "text", "output format: text or json")
}

// options validates format and converts f into generator.Options.
func (f genFlags) options() (generator.Options, error) {
	switch f.format {
	case "", "text", "json":
	default:
		return generator.Options{}, fmt.Errorf("unknown --format %q (want text or json)", f.format)
	}

	return generator.Options{DryRun: f.dryRun, Force: f.force}, nil
}

// resolveGenerator fetches the shared domain/generator.Service that
// app/provider/generator.go bound into the container as "generator".
func resolveGenerator(app *hex.App) (*generator.Service, error) {
	return container.Make[*generator.Service](app.Container(), "generator")
}

// report writes actions to w per opts/format — the shared tail every
// generating command's RunE ends with.
func report(w io.Writer, actions []generator.Action, opts generator.Options, format string) error {
	return generator.Report(w, actions, opts.DryRun, format)
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
