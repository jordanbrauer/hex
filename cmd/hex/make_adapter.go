package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

// adapterData feeds the adapter template.
type adapterData struct {
	Domain     string // package name of the domain ("user")
	DomainType string // exported type name ("User")
	Dialect    string // "sqlite" or "postgres"
	ModulePath string
}

func newMakeAdapterCommand() *cobra.Command {
	var (
		dialect string
		flags   genFlags
	)

	cmd := &cobra.Command{
		Use:   "make:adapter <domain>",
		Args:  cobra.ExactArgs(1),
		Short: "Generate an infrastructure adapter for a domain repository",
		Long:  helpLong("make_adapter"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, modulePath, err := projectRoot()
			if err != nil {
				return err
			}

			domain := args[0]
			if domain == "" {
				return errors.New("domain name is empty")
			}

			if dialect != "sqlite" && dialect != "postgres" {
				return fmt.Errorf("unsupported dialect %q (want sqlite or postgres)", dialect)
			}

			data := adapterData{
				Domain:     goPackageName(domain),
				DomainType: pascalCase(domain),
				Dialect:    dialect,
				ModulePath: modulePath,
			}

			target := filepath.Join(root, "infrastructure", dialect, data.Domain+"_repository.go")

			g, err := newGeneratorFromFlags(flags)
			if err != nil {
				return err
			}

			if err := g.render("templates/adapter.go.tmpl", target, data); err != nil {
				return err
			}

			return g.report()
		},
	}

	cmd.Flags().StringVar(&dialect, "dialect", "sqlite", "SQL dialect: sqlite or postgres")
	setExample(cmd, "make_adapter")
	addGeneratorFlags(cmd, &flags)

	return cmd
}
