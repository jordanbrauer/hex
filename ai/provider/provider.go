// Package provider is the default hex/ai service provider.
//
// It reads ai.provider + ai.model from hex/config, dispatches to the
// consumer-supplied Factory for that provider name, constructs a
// fantasy.Agent, binds Provider under "ai.provider" and Agent under
// "ai" in the container, and installs the agent as hex/ai's default.
//
// Consumer factories in app/provider/ai.go supply the map of provider
// names to factories:
//
//	func AI() *aiprovider.Provider {
//	    return &aiprovider.Provider{
//	        Factories: map[string]aiprovider.Factory{
//	            "openai":    openai.Factory,
//	            "anthropic": anthropic.Factory,
//	        },
//	        // Optional: Tools, extra AgentOptions, hooks.
//	    }
//	}
//
// Configs returns the embedded framework defaults + CUE schema for
// the "ai" namespace.
package provider

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"charm.land/fantasy"

	"github.com/jordanbrauer/hex/ai"
	"github.com/jordanbrauer/hex/config"
	"github.com/jordanbrauer/hex/container"
	hexlog "github.com/jordanbrauer/hex/log"
	"github.com/jordanbrauer/hex/provider"
)

//go:embed config
var configFS embed.FS

// Configs returns the embedded default TOML + CUE files this provider
// contributes to hex/config. Add it to hex/config.Provider.Sources.
func Configs() fs.FS {
	sub, err := fs.Sub(configFS, "config")
	if err != nil {
		panic("ai/provider: embedded config subdir missing: " + err.Error())
	}

	return sub
}

// Factory constructs a fantasy.Provider from configuration. Each
// hex/ai/<provider> subpackage exports one Factory (openai.Factory,
// anthropic.Factory, ...).
type Factory func(ctx context.Context, store *config.Store) (fantasy.Provider, error)

// Provider wires the LLM stack into the container.
type Provider struct {
	provider.Base

	// Binding for the constructed Agent. Defaults to "ai".
	Binding string

	// ProviderBinding for the constructed fantasy.Provider. Defaults
	// to "ai.provider". Consumers who need to make additional
	// LanguageModel calls (multiple models from the same provider)
	// resolve this binding.
	ProviderBinding string

	// Namespace is the config namespace read for ai settings. Defaults
	// to "ai".
	Namespace string

	// Factories maps a config value (from ai.provider) to a factory
	// function that constructs a fantasy.Provider. Consumer supplies
	// this in their app/provider/ai.go factory, importing the
	// hex/ai/<provider> subpackages they want available.
	Factories map[string]Factory

	// Tools are attached to the default agent. Consumers can also add
	// tools after Bootstrap by resolving the agent from the container
	// and calling agent-specific APIs.
	Tools []ai.Tool

	// AgentOptions are appended to the fantasy.NewAgent call verbatim.
	// System prompt + tools from Tools are already threaded through;
	// use this for advanced settings (temperature, top_p, custom stop
	// conditions, etc.).
	AgentOptions []fantasy.AgentOption

	// Configure, if non-nil, runs after the agent is built but before
	// it is installed as the default. Return an error to abort
	// Bootstrap.
	Configure func(ai.Agent) error

	agent    ai.Agent
	provider fantasy.Provider
}

// Register selects the configured factory, constructs the agent, and
// installs everything into the container + hex/ai default.
func (p *Provider) Register(app provider.Application) error {
	binding := p.Binding
	if binding == "" {
		binding = "ai"
	}

	providerBinding := p.ProviderBinding
	if providerBinding == "" {
		providerBinding = "ai.provider"
	}

	ns := p.Namespace
	if ns == "" {
		ns = "ai"
	}

	if p.Factories == nil {
		return errors.New("ai/provider: Factories map is empty (import at least one hex/ai/<name> subpackage)")
	}

	store, err := container.Make[*config.Store](app, "config")
	if err != nil {
		return fmt.Errorf("ai/provider: resolve config: %w", err)
	}

	name := store.String(ns + ".provider")
	if name == "" {
		return fmt.Errorf("ai/provider: %s.provider is unset", ns)
	}

	factory, ok := p.Factories[name]
	if !ok {
		return fmt.Errorf("ai/provider: no factory registered for %q (known: %v)", name, factoryNames(p.Factories))
	}

	ctx := context.Background()

	fantasyProvider, err := factory(ctx, store)
	if err != nil {
		return fmt.Errorf("ai/provider: factory %q: %w", name, err)
	}

	modelID := store.String(ns + ".model")
	if modelID == "" {
		return fmt.Errorf("ai/provider: %s.model is unset", ns)
	}

	model, err := fantasyProvider.LanguageModel(ctx, modelID)
	if err != nil {
		return fmt.Errorf("ai/provider: language model %q: %w", modelID, err)
	}

	opts := make([]fantasy.AgentOption, 0, 2+len(p.AgentOptions))

	if sp := store.String(ns + ".system_prompt"); sp != "" {
		opts = append(opts, ai.WithSystemPrompt(sp))
	}

	if len(p.Tools) > 0 {
		opts = append(opts, ai.WithTools(p.Tools...))
	}

	opts = append(opts, p.AgentOptions...)

	agent := ai.NewAgent(model, opts...)

	if p.Configure != nil {
		if err := p.Configure(agent); err != nil {
			return fmt.Errorf("ai/provider: Configure: %w", err)
		}
	}

	p.agent = agent
	p.provider = fantasyProvider

	ai.SetDefault(agent)

	app.Singleton(binding, func(*container.Container) (any, error) {
		return p.agent, nil
	})

	app.Singleton(providerBinding, func(*container.Container) (any, error) {
		return p.provider, nil
	})

	hexlog.Info("ai/provider: ready",
		hexlog.String("provider", name),
		hexlog.String("model", modelID),
		hexlog.Int("tools", len(p.Tools)),
	)

	return nil
}

// Shutdown clears the package-level default so tests / repeated
// bootstraps don't see stale state.
func (p *Provider) Shutdown(ctx context.Context, app provider.Application) error {
	if ai.Default() == p.agent {
		ai.SetDefault(nil)
	}

	return nil
}

// factoryNames returns the sorted list of registered factory keys for
// error messages.
func factoryNames(m map[string]Factory) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}

	return names
}

// Compile-time confirmation the provider participates in shutdown.
var _ provider.Shutdowner = (*Provider)(nil)
