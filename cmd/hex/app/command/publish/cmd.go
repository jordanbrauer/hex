// Package publish implements `hex publish`.
package publish

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	cacheprovider "github.com/jordanbrauer/hex/cache/provider"
	dbprovider "github.com/jordanbrauer/hex/db/provider"
	logprovider "github.com/jordanbrauer/hex/log/provider"
	telemetryprovider "github.com/jordanbrauer/hex/telemetry/provider"
	webprovider "github.com/jordanbrauer/hex/web/provider"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
)

//go:embed long.md
var longFile string

//go:embed example.sh
var exampleFile string

var (
	long    = strings.TrimRight(longFile, "\n")
	example = strings.TrimRight(exampleFile, "\n")
)

// publishables enumerates every framework provider that ships
// publishable configuration. Add new entries here as more framework
// packages start embedding their own config directories.
var publishables = map[string]fs.FS{
	"log":       logprovider.Configs(),
	"db":        dbprovider.Configs(),
	"cache":     cacheprovider.Configs(),
	"web":       webprovider.Configs(),
	"telemetry": telemetryprovider.Configs(),
}

// New builds the `hex publish [component]` command.
func New(app *hex.App) *cobra.Command {
	var (
		flags generator.Flags
		all   bool
	)

	cmd := &cobra.Command{
		Use:     "publish [component]",
		Short:   "Copy a framework provider's config files into config/",
		Long:    long + "\n\nComponents: " + strings.Join(publishableNames(), ", ") + ".",
		Example: example,
		Args: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return errors.New("--all cannot be combined with a component name")
			}

			if !all && len(args) != 1 {
				return errors.New("supply a component name or pass --all")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root, _, err := generator.ProjectRoot()
			if err != nil {
				return err
			}

			confDir := filepath.Join(root, "config")

			opts, err := flags.Options()
			if err != nil {
				return err
			}

			svc, err := generator.Resolve(app)
			if err != nil {
				return err
			}

			var actions []generator.Action

			names := args
			if all {
				names = publishableNames()
			}

			for _, name := range names {
				src, ok := publishables[name]
				if !ok {
					return fmt.Errorf("unknown component %q (known: %s)", name, strings.Join(publishableNames(), ", "))
				}

				acts, err := svc.PublishAll(src, ".toml", confDir, opts)
				if err != nil {
					return fmt.Errorf("publish %s: %w", name, err)
				}

				if len(acts) == 0 {
					acts = []generator.Action{{Kind: "skip", Path: name, Detail: "no files to publish"}}
				}

				actions = append(actions, acts...)
			}

			return generator.Report(cmd.OutOrStdout(), actions, opts.DryRun, flags.Format)
		},
	}

	generator.AddFlags(cmd, &flags)
	cmd.Flags().BoolVar(&all, "all", false, "publish every framework component")

	return cmd
}

// publishableNames returns the sorted list of publishable component
// identifiers for help text and validation.
func publishableNames() []string {
	names := make([]string, 0, len(publishables))
	for name := range publishables {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
