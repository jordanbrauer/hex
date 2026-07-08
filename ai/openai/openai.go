// Package openai constructs a fantasy.Provider for OpenAI's language
// models from hex configuration. Consumer factories register Factory
// against the "openai" name in their hex/ai/provider.Provider.Factories
// map; hex init --ai openai scaffolds this wiring automatically.
//
// Configuration namespace:
//
//	[openai]                        # under the top-level "ai" namespace when
//	                                # merged with hex/ai/provider/config
//	api_key = ""                    # bind to OPENAI_API_KEY via env.yaml
//	base_url = ""                   # optional custom endpoint
//	organization = ""               # optional
//	project = ""                    # optional
package openai

import (
	"context"
	"errors"
	"os"

	"charm.land/fantasy"
	fantasyopenai "charm.land/fantasy/providers/openai"

	"github.com/jordanbrauer/hex/config"
)

// Name identifies this provider in the hex/ai config namespace.
// Matches fantasyopenai.Name for parity.
const Name = fantasyopenai.Name

// Factory builds a fantasy.Provider from the "ai.openai.*" config
// namespace. Consumer factories add this under the key "openai"
// in hex/ai/provider.Provider.Factories.
func Factory(_ context.Context, store *config.Store) (fantasy.Provider, error) {
	apiKey := store.String("ai.openai.api_key")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if apiKey == "" {
		return nil, errors.New("ai/openai: api_key missing (set ai.openai.api_key or OPENAI_API_KEY env var)")
	}

	opts := []fantasyopenai.Option{
		fantasyopenai.WithAPIKey(apiKey),
	}

	if v := store.String("ai.openai.base_url"); v != "" {
		opts = append(opts, fantasyopenai.WithBaseURL(v))
	}

	if v := store.String("ai.openai.organization"); v != "" {
		opts = append(opts, fantasyopenai.WithOrganization(v))
	}

	if v := store.String("ai.openai.project"); v != "" {
		opts = append(opts, fantasyopenai.WithProject(v))
	}

	return fantasyopenai.New(opts...)
}
