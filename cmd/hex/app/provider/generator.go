package provider

import (
	"github.com/jordanbrauer/hex/container"
	hexprovider "github.com/jordanbrauer/hex/provider"

	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
	"github.com/jordanbrauer/hex/cmd/hex/infrastructure/embedfs"
)

// generatorProvider binds a domain/generator.Service into the container
// as "generator", backed by the compiled-in blueprint templates
// (infrastructure/embedfs). Every `hex make:*` command resolves it from
// the container instead of constructing its own — the same pattern
// hex/lua/provider uses for the shared Lua environment.
type generatorProvider struct {
	hexprovider.Base
}

// Generator returns the provider that wires domain/generator.Service into
// the container.
func Generator() hexprovider.Service {
	return &generatorProvider{}
}

func (p *generatorProvider) Register(app hexprovider.Application) error {
	svc := generator.NewService(embedfs.New())

	app.Singleton("generator", func(*container.Container) (any, error) {
		return svc, nil
	})

	return nil
}
