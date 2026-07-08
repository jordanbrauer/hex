// Package provider is the default hex/policy service provider.
//
// It constructs a Casbin Enforcer from a caller-supplied model + adapter
// and binds it into the container. The model and adapter come from the
// consumer's factory because they are typically embed.FS references
// specific to the app.
package provider

import (
	"errors"
	"io/fs"

	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/policy"
	"github.com/jordanbrauer/hex/provider"
)

// Provider wires a *policy.Enforcer into the container.
type Provider struct {
	provider.Base

	// Binding is the container name. Defaults to "enforcer".
	Binding string

	// One of ModelSource / ModelPath / ModelFS+ModelFile must be set.
	// Precedence: ModelSource > ModelPath > ModelFS+ModelFile.

	// ModelSource is the model DSL as an already-loaded string.
	ModelSource string

	// ModelPath is a filesystem path to a Casbin .conf model.
	ModelPath string

	// ModelFS is an fs.FS containing the model.
	ModelFS fs.FS

	// ModelFile is the path within ModelFS to the model.
	ModelFile string

	// Adapter is required. Consumers construct via policy.NewMemoryAdapter,
	// NewFileAdapter, NewFileAdapterFS, or their own implementation.
	Adapter policy.Adapter

	enf *policy.Enforcer
}

// Register constructs the Enforcer and binds it.
func (p *Provider) Register(app provider.Application) error {
	if p.Adapter == nil {
		return errors.New("policy/provider: Adapter is required")
	}

	binding := p.Binding
	if binding == "" {
		binding = "enforcer"
	}

	enf, err := p.buildEnforcer()
	if err != nil {
		return err
	}

	p.enf = enf

	app.Singleton(binding, func(*container.Container) (any, error) {
		return p.enf, nil
	})

	return nil
}

func (p *Provider) buildEnforcer() (*policy.Enforcer, error) {
	switch {
	case p.ModelSource != "":
		return policy.NewFromString(p.ModelSource, p.Adapter)
	case p.ModelPath != "":
		return policy.NewFromFile(p.ModelPath, p.Adapter)
	case p.ModelFS != nil && p.ModelFile != "":
		return policy.NewFromFS(p.ModelFS, p.ModelFile, p.Adapter)
	default:
		return nil, errors.New("policy/provider: no model source configured (set ModelSource, ModelPath, or ModelFS+ModelFile)")
	}
}
