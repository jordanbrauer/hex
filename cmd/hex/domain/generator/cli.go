package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/container"
)

// Flags carries the flags shared by every generating command (`hex
// init` and every `hex make` subcommand).
type Flags struct {
	Force  bool
	DryRun bool
	Format string
}

// AddFlags registers the standard generator flags on cmd. Every
// generating command uses this so the surface stays uniform and
// agent-legible.
func AddFlags(cmd *cobra.Command, f *Flags) {
	cmd.Flags().BoolVar(&f.Force, "force", false, "overwrite existing files")
	cmd.Flags().BoolVar(&f.DryRun, "dry-run", false, "print the actions without writing any files")
	cmd.Flags().StringVar(&f.Format, "format", "text", "output format: text or json")
}

// Options validates Format and converts f into Options.
func (f Flags) Options() (Options, error) {
	switch f.Format {
	case "", "text", "json":
	default:
		return Options{}, fmt.Errorf("unknown --format %q (want text or json)", f.Format)
	}

	return Options{DryRun: f.DryRun, Force: f.Force}, nil
}

// Resolve fetches the shared Service that app/provider/generator.go bound
// into app's container as "generator".
func Resolve(app *hex.App) (*Service, error) {
	return container.Make[*Service](app.Container(), "generator")
}

// ProjectRoot walks up from cwd until it finds a go.mod file. Returns the
// absolute directory containing go.mod plus the module path.
func ProjectRoot() (dir, modulePath string, err error) {
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
