// Package embedfs is the built-in adapter for domain/generator: it
// serves Blueprint definitions and template bytes from the CLI's own
// compiled-in templates/mantemplates directories. It also exposes the
// man-page intros the latter carries, since nothing else in the CLI
// touches that embed. Per-command help text (long.md/example.sh) is
// embedded directly by each command package instead of centralized
// here.
package embedfs

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
)

//go:embed all:templates
var templatesFS embed.FS

//go:embed mantemplates
var manTemplatesFS embed.FS

// blueprints are the built-in hex make generators, keyed by ID. Command
// variants with conditional, multi-target wiring (make command --group,
// make controller's route wiring) aren't modeled here — they render/wire
// through Service directly; see app/command.
var blueprints = map[string]*generator.Blueprint{
	"provider": {
		ID:          "provider",
		Name:        "Service provider",
		Description: "A hex.App service provider, wired into app/boot.go.",
		Files: []generator.FileSpec{
			{Template: "templates/provider.go.tmpl", Target: "app/provider/{{.Package}}.go"},
		},
		Wires: []generator.WireSpec{
			{
				File:      "app/boot.go",
				Marker:    "// hex:providers",
				Insertion: "&provider.{{.Name}}{},",
				Detail:    "added {{.Name}}",
			},
		},
	},
	"domain": {
		ID:          "domain",
		Name:        "Domain package",
		Description: "Entity, repository interface, service, and errors for a domain.",
		Files: []generator.FileSpec{
			{Template: "templates/domain/entity.go.tmpl", Target: "domain/{{.Package}}/{{.Package}}.go"},
			{Template: "templates/domain/repository.go.tmpl", Target: "domain/{{.Package}}/repository.go"},
			{Template: "templates/domain/service.go.tmpl", Target: "domain/{{.Package}}/service.go"},
			{Template: "templates/domain/errors.go.tmpl", Target: "domain/{{.Package}}/errors.go"},
		},
	},
	"adapter": {
		ID:          "adapter",
		Name:        "Infrastructure adapter",
		Description: "A SQL adapter implementing a domain's Repository interface.",
		Files: []generator.FileSpec{
			{Template: "templates/adapter.go.tmpl", Target: "infrastructure/{{.Dialect}}/{{.Domain}}_repository.go"},
		},
	},
	"controller": {
		ID:          "controller",
		Name:        "HTTP controller",
		Description: "A RESTful controller struct; route wiring is applied separately.",
		Files: []generator.FileSpec{
			{Template: "templates/controller.go.tmpl", Target: "app/controller/{{.Package}}.go"},
		},
	},
	"command": {
		ID:          "command",
		Name:        "Top-level cobra command",
		Description: "A cobra subcommand wired into app/command/root.go.",
		Files: []generator.FileSpec{
			{Template: "templates/command.go.tmpl", Target: "app/command/{{.Name}}.go"},
		},
		Wires: []generator.WireSpec{
			{
				File:      "app/command/root.go",
				Marker:    "// hex:commands",
				Insertion: "{{.FuncName}}(app),",
				Detail:    "added {{.FuncName}}",
			},
		},
	},
	"migration": {
		ID:          "migration",
		Name:        "SQL migration",
		Description: "A timestamped up/down SQL migration pair.",
		Files: []generator.FileSpec{
			{Template: "templates/migration.up.sql.tmpl", Target: "database/migrations/{{.Timestamp}}_{{.Name}}.up.sql"},
			{Template: "templates/migration.down.sql.tmpl", Target: "database/migrations/{{.Timestamp}}_{{.Name}}.down.sql"},
		},
	},
}

// blueprintOrder fixes List's output order.
var blueprintOrder = []string{"provider", "domain", "adapter", "controller", "command", "migration"}

// Repository implements generator.Repository over the embedded
// templates tree. The built-in blueprint set is fixed at compile time —
// Store/Delete report an error; a future on-disk adapter would back
// user-defined custom blueprints instead.
type Repository struct{}

var _ generator.Repository = Repository{}

// New returns the embedded-template-backed Repository.
func New() Repository { return Repository{} }

// Get returns the built-in blueprint identified by id.
func (Repository) Get(_ context.Context, id string) (*generator.Blueprint, error) {
	bp, ok := blueprints[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", generator.ErrBlueprintNotFound, id)
	}

	return bp, nil
}

// List returns every built-in blueprint, in a fixed order.
func (Repository) List(_ context.Context) ([]*generator.Blueprint, error) {
	out := make([]*generator.Blueprint, 0, len(blueprintOrder))
	for _, id := range blueprintOrder {
		out = append(out, blueprints[id])
	}

	return out, nil
}

// Store is not supported: the built-in blueprint set is compiled in.
func (Repository) Store(_ context.Context, _ *generator.Blueprint) error {
	return errors.New("embedfs: blueprints are compiled in; Store is not supported")
}

// Delete is not supported: the built-in blueprint set is compiled in.
func (Repository) Delete(_ context.Context, _ string) error {
	return errors.New("embedfs: blueprints are compiled in; Delete is not supported")
}

// Read returns the raw contents of an embedded template file.
func (Repository) Read(_ context.Context, path string) ([]byte, error) {
	return fs.ReadFile(templatesFS, path)
}

// ManTemplate returns an embedded hand-authored manpage prose block by
// name (e.g. "hex.1.intro.md"), used by `hex gen-man`.
func ManTemplate(name string) ([]byte, error) {
	return manTemplatesFS.ReadFile("mantemplates/" + name)
}
