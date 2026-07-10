// Package command implements `hex make command`. Its package name
// matches its Use string ("command"), same as every other make
// subcommand — import it aliased (e.g. makecommand) to avoid confusion
// with the top-level app/command package.
package command

import (
	_ "embed"
	"errors"
	"strings"

	"github.com/spf13/cobra"

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

// commandData feeds the command templates.
type commandData struct {
	// Name is the subcommand name (e.g. "login" for `myapp auth login`).
	Name string
	// FuncName is the exported function that returns the *cobra.Command
	// (e.g. "Login" for `command.Login(app)`).
	FuncName string
	// Group is the parent command group (e.g. "auth"). Empty means the
	// command sits directly under the root.
	Group string
	// GroupFunc is the group's root function ("Auth" for `command.Auth(app)`).
	GroupFunc string
	// ModulePath is this project's go.mod module path.
	ModulePath string
	// GroupPackage is the group's Go package name (lower-cased Group).
	GroupPackage string
}

// New builds the `hex make command <name>` command.
func New(app *hex.App) *cobra.Command {
	var (
		group string
		flags generator.Flags
	)

	cmd := &cobra.Command{
		Use:     "command <name>",
		Args:    cobra.ExactArgs(1),
		Short:   "Generate a cobra command",
		Long:    long,
		Example: example,
		RunE:    run(app, &group, &flags),
	}

	cmd.Flags().StringVar(&group, "group", "", "parent command group (creates a subcommand)")
	generator.AddFlags(cmd, &flags)

	return cmd
}

func run(app *hex.App, group *string, flags *generator.Flags) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		root, modulePath, err := generator.ProjectRoot()
		if err != nil {
			return err
		}

		name := args[0]
		if name == "" {
			return errors.New("command name is empty")
		}

		data := commandData{
			Name:       generator.GoPackageName(name),
			FuncName:   generator.PascalCase(name),
			Group:      generator.GoPackageName(*group),
			GroupFunc:  generator.PascalCase(*group),
			ModulePath: modulePath,
		}
		data.GroupPackage = data.Group

		opts, err := flags.Options()
		if err != nil {
			return err
		}

		svc, err := generator.Resolve(app)
		if err != nil {
			return err
		}

		ctx := cmd.Context()

		var actions []generator.Action

		if data.Group == "" {
			actions, err = svc.Run(ctx, "command", root, data, opts)
		} else {
			actions, err = makeGroupCommand(ctx, svc, root, data, opts)
		}

		if err != nil {
			return err
		}

		return generator.Report(cmd.OutOrStdout(), actions, opts.DryRun, flags.Format)
	}
}
