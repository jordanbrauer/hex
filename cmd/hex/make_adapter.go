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
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "make:adapter <domain>",
		Short: "Generate an infrastructure adapter for a domain repository",
		Long: `Create infrastructure/<dialect>/<domain>_repository.go — a stub
implementation of domain/<domain>.Repository backed by the given SQL
dialect.

The generator produces panic("not implemented") stubs for the standard
Repository methods (Store, Get, List, Delete) that make:domain scaffolds.
If you have extended Repository with additional methods, add them by
hand.`,
		Args: cobra.ExactArgs(1),
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

			g := newGenerator()
			g.force = force

			return g.render("templates/adapter.go.tmpl", target, data)
		},
	}

	cmd.Flags().StringVar(&dialect, "dialect", "sqlite", "SQL dialect: sqlite or postgres")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")

	return cmd
}
