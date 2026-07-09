package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex/build"
)

// VersionOptions configures the version subcommand.
type VersionOptions struct {
	// App is a human-readable app name used in the one-line short output
	// (e.g. "myapp"). Falls back to the binary name if empty.
	App string

	// Long, when true, prints the multi-line build.Info block. When false
	// (default), prints a single line.
	Long bool
}

// Version returns a Cobra `version` subcommand that renders build metadata
// from hex/build. It has no dependencies beyond hex/build and Cobra.
func Version(opts VersionOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print build version and metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			name := opts.App
			if name == "" {
				name = cmd.Root().Use
			}

			if opts.Long {
				cmd.Println(name)
				cmd.Print(build.Info())

				return nil
			}

			cmd.Println(oneLine(name))

			return nil
		},
	}

	cmd.Flags().BoolP("long", "l", false, "print full build metadata")

	// Runtime --long overrides the compile-time default so consumers can
	// bind the flag without owning the whole command.
	prev := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if long, _ := cmd.Flags().GetBool("long"); long {
			opts.Long = true
		}

		return prev(cmd, args)
	}

	return cmd
}

func oneLine(name string) string {
	commit := build.ShortCommit()
	built := ""

	if t := build.Time(); !t.IsZero() {
		built = " " + t.Format(time.DateOnly)
	}

	suffix := ""
	if build.Modified() {
		suffix = " (modified)"
	}

	return fmt.Sprintf("%s %s (%s)%s%s", name, build.Version(), commit, built, suffix)
}
