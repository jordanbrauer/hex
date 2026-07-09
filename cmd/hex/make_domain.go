package main

import (
	"errors"
	"path/filepath"

	"github.com/spf13/cobra"
)

// domainData feeds the four domain templates.
type domainData struct {
	Package    string // "user"
	Name       string // "User"
	Plural     string // "Users"
	ModulePath string
}

func newMakeDomainCommand() *cobra.Command {
	var flags genFlags

	cmd := &cobra.Command{
		Use:   "make:domain <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Generate a domain package",
		Long:  helpLong("make_domain"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, modulePath, err := projectRoot()
			if err != nil {
				return err
			}

			name := args[0]
			if name == "" {
				return errors.New("domain name is empty")
			}

			data := domainData{
				Package:    goPackageName(name),
				Name:       pascalCase(name),
				Plural:     pluralise(pascalCase(name)),
				ModulePath: modulePath,
			}

			dir := filepath.Join(root, "domain", data.Package)

			files := []struct{ tpl, target string }{
				{"templates/domain/entity.go.tmpl", filepath.Join(dir, data.Package+".go")},
				{"templates/domain/repository.go.tmpl", filepath.Join(dir, "repository.go")},
				{"templates/domain/service.go.tmpl", filepath.Join(dir, "service.go")},
				{"templates/domain/errors.go.tmpl", filepath.Join(dir, "errors.go")},
			}

			g, err := newGeneratorFromFlags(flags)
			if err != nil {
				return err
			}

			for _, f := range files {
				if err := g.render(f.tpl, f.target, data); err != nil {
					return err
				}
			}

			return g.report()
		},
	}

	setExample(cmd, "make_domain")
	addGeneratorFlags(cmd, &flags)

	return cmd
}
