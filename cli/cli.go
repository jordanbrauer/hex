// Package cli provides Cobra scaffolding for hex applications.
//
// The helpers here cover the boilerplate every hex-based CLI wants: a root
// command wired to *hex.App, common persistent flags (--log-level, --env,
// --verbose), a version subcommand backed by hex/build, and a small
// context helper so subcommands can retrieve the *hex.App without a
// package-level global.
//
// Anything app-specific — command groups, subcommand tree, error
// pretty-printing, plugin discovery, session/env semantics — stays in the
// consumer app. hex/cli deliberately keeps its surface tight so it does
// not turn into "one big framework CLI."
//
// Typical main:
//
//	app := hex.New()
//	provider.Boot(app)
//	if err := app.Bootstrap(ctx); err != nil { log.Fatal(err) }
//	defer app.Shutdown(ctx)
//
//	root := hexcli.Root(hexcli.RootOptions{
//	    Name:  "myapp",
//	    Short: "my hex-powered CLI",
//	    App:   app,
//	})
//	root.AddCommand(hexcli.Version(hexcli.VersionOptions{App: "myapp"}))
//	root.AddCommand(cmd.Auth(app), cmd.Token(app))
//	os.Exit(hexcli.Execute(root))
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	hexlog "github.com/jordanbrauer/hex/log"
)

// RootOptions configures Root. Only Name is required.
type RootOptions struct {
	// Name is the command binary name (cobra.Command.Use).
	Name string

	// Short is the one-line description shown in help output.
	Short string

	// Long is the long-form description. If empty, Root leaves it unset so
	// Cobra falls back to Short.
	Long string

	// App is the hex kernel to stash in the command context so subcommands
	// can retrieve it via FromContext. Optional; may be nil for pure-help
	// setups.
	App *hex.App

	// DefaultLogLevel is the value used for the --log-level flag default.
	// Empty means the flag is added with an empty default (equivalent to
	// "leave the current logger level alone until the user opts in").
	DefaultLogLevel string

	// SilenceUsage and SilenceErrors mirror the Cobra fields of the same
	// name. Default: both true — hex apps generally render their own
	// errors and don't want Cobra's usage dump on every failed run.
	// Set the *Enable variants to force-enable one you want back.
	EnableUsageOnError bool
	EnableErrorPrint   bool
}

// Root builds the top-level Cobra command with hex's standard wiring.
//
// The returned command:
//
//   - Silences usage-on-error and error print by default; consumer apps
//     handle their own error rendering.
//   - Carries opts.App in its context, retrievable via FromContext.
//   - Adds the --log-level, --env, and --verbose persistent flags. --log-level
//     is applied to hex/log during PersistentPreRunE.
//
// Consumer apps typically add subcommands and their own PersistentPreRun on
// top. When they wire their own PersistentPreRunE, it is called *after*
// hex's — see WithPreRun.
func Root(opts RootOptions) *cobra.Command {
	root := &cobra.Command{
		Use:           opts.Name,
		Short:         opts.Short,
		Long:          opts.Long,
		SilenceUsage:  !opts.EnableUsageOnError,
		SilenceErrors: !opts.EnableErrorPrint,
	}

	root.CompletionOptions.HiddenDefaultCmd = true

	if opts.App != nil {
		ctx := WithApp(context.Background(), opts.App)
		root.SetContext(ctx)
	}

	AddLogLevelFlag(root, opts.DefaultLogLevel)
	AddEnvFlag(root)
	AddVerboseFlag(root)

	root.PersistentPreRunE = applyLogLevel

	return root
}

// applyLogLevel is Root's default PersistentPreRunE. It reads --log-level
// and --verbose, resolves the winning value (verbose > log-level > current)
// and applies it to hex/log. Consumers who need additional PersistentPreRun
// logic should call WithPreRun to chain their own function after this one.
func applyLogLevel(cmd *cobra.Command, _ []string) error {
	if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
		hexlog.SetLevel(hexlog.DebugLevel)

		return nil
	}

	level, _ := cmd.Flags().GetString("log-level")
	if level == "" {
		return nil
	}

	parsed, err := hexlog.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("invalid --log-level %q: %w", level, err)
	}

	hexlog.SetLevel(parsed)

	return nil
}

// WithPreRun chains fn to run after hex's default PersistentPreRunE on cmd.
// Both functions run in sequence; if hex's returns an error, fn is not
// called. Use this instead of overwriting PersistentPreRunE directly when
// you want to keep the --log-level / --verbose handling hex provides.
func WithPreRun(cmd *cobra.Command, fn func(*cobra.Command, []string) error) {
	prev := cmd.PersistentPreRunE

	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if prev != nil {
			if err := prev(c, args); err != nil {
				return err
			}
		}

		return fn(c, args)
	}
}

// AddLogLevelFlag installs the --log-level persistent flag on cmd with the
// given default. Safe to call on any Cobra command; typically only used on
// the root command.
func AddLogLevelFlag(cmd *cobra.Command, defaultLevel string) {
	cmd.PersistentFlags().String("log-level", defaultLevel, "log level: debug, info, warn, error, fatal")
}

// AddVerboseFlag installs --verbose / -v as a boolean shortcut for
// --log-level=debug.
func AddVerboseFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolP("verbose", "v", false, "shortcut for --log-level=debug")
}

// AddEnvFlag installs --env / -e for consumer apps that ship multiple
// environments (dev, staging, prod). hex reads the flag but has no opinion
// on what values are valid; the consumer's PersistentPreRun interprets it.
func AddEnvFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP("env", "e", "", "override the environment for this command")
}

// Execute runs root and returns a suggested process exit code: 0 on
// success, 1 on any non-nil error. Consumers with richer exit-code logic
// should write their own instead.
func Execute(root *cobra.Command) int {
	if err := root.Execute(); err != nil {
		// Errors are already printed by Cobra when SilenceErrors is false.
		// When true (hex's default) the consumer must render them; we still
		// emit a minimal fallback so no error is completely lost.
		if root.SilenceErrors && !errors.Is(err, errCobraHelp) {
			fmt.Fprintln(os.Stderr, "error:", err)
		}

		return 1
	}

	return 0
}

// errCobraHelp is a sentinel used above to avoid printing Cobra's own help
// errors twice. Cobra doesn't expose one, so we keep this here as a hook
// for later refinement — currently unused by Cobra, but keeps the fallback
// print robust if that changes.
var errCobraHelp = errors.New("hex/cli: cobra help")
