package main

import (
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

func newPublishCommand() *cobra.Command {
	var (
		flags genFlags
		all   bool
	)

	cmd := &cobra.Command{
		Use:   "publish [component]",
		Short: "Copy a framework provider's config files into config/",
		Long:  helpLong("publish") + "\n\nComponents: " + strings.Join(publishableNames(), ", ") + ".",
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
			root, _, err := projectRoot()
			if err != nil {
				return err
			}

			confDir := filepath.Join(root, "config")

			g, err := newGeneratorFromFlags(flags)
			if err != nil {
				return err
			}

			names := args
			if all {
				names = publishableNames()
			}

			for _, name := range names {
				src, ok := publishables[name]
				if !ok {
					return fmt.Errorf("unknown component %q (known: %s)", name, strings.Join(publishableNames(), ", "))
				}

				n, err := g.publishAll(src, ".toml", confDir)
				if err != nil {
					return fmt.Errorf("publish %s: %w", name, err)
				}

				if n == 0 {
					g.record("skip", name, "no files to publish")
				}
			}

			return g.report()
		},
	}

	setExample(cmd, "publish")
	addGeneratorFlags(cmd, &flags)
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
