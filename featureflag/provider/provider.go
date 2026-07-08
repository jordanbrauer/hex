// Package provider is the default hex/featureflag service provider.
//
// It constructs a GOFF client from a caller-supplied embed.FS or file
// path and installs it as the package-level default so consumers can
// call featureflag.Bool(...) etc. without threading a *Client around.
package provider

import (
	"context"
	"errors"
	"io/fs"

	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/featureflag"
	"github.com/jordanbrauer/hex/provider"
)

// Provider wires a *featureflag.Client into the container.
type Provider struct {
	provider.Base

	// Binding is the container name. Defaults to "flags".
	Binding string

	// FS + File describe an embedded flag file (typical).
	FS   fs.FS
	File string

	// Path is an on-disk flag file. Ignored when FS+File are set.
	Path string

	// Options tune the underlying GOFF client (polling, retrievers, etc.).
	Options featureflag.Options

	// SkipDefault, when true, prevents installing this client via
	// featureflag.SetDefault. Default is to install.
	SkipDefault bool

	client *featureflag.Client
}

// Boot constructs the client and binds it. Runs at Boot because
// featureflag.NewFromFile/FS may open network connections (e.g. when
// consumers extend Options.Retrievers with HTTP or S3 sources).
func (p *Provider) Boot(ctx context.Context, app provider.Application) error {
	binding := p.Binding
	if binding == "" {
		binding = "flags"
	}

	client, err := p.buildClient()
	if err != nil {
		return err
	}

	p.client = client

	if !p.SkipDefault {
		featureflag.SetDefault(client)
	}

	app.Singleton(binding, func(*container.Container) (any, error) {
		return p.client, nil
	})

	return nil
}

// Shutdown closes the client. GOFF stops its background pollers.
func (p *Provider) Shutdown(ctx context.Context, app provider.Application) error {
	if p.client == nil {
		return nil
	}

	p.client.Close()

	return nil
}

func (p *Provider) buildClient() (*featureflag.Client, error) {
	switch {
	case p.FS != nil && p.File != "":
		return featureflag.NewFromFS(p.FS, p.File, p.Options)
	case p.Path != "":
		return featureflag.NewFromFile(p.Path, p.Options)
	default:
		return nil, errors.New("featureflag/provider: no flag source configured (set FS+File or Path)")
	}
}

// Compile-time confirmation the provider participates in shutdown.
var _ provider.Shutdowner = (*Provider)(nil)
