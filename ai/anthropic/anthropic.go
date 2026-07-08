// Package anthropic constructs a fantasy.Provider for Anthropic's
// language models from hex configuration. Consumer factories register
// Factory against the "anthropic" name in their
// hex/ai/provider.Provider.Factories map; hex init --ai anthropic
// scaffolds this wiring automatically.
//
// Configuration namespace:
//
//	[anthropic]                     # under the top-level "ai" namespace when
//	                                # merged with hex/ai/provider/config
//	api_key = ""                    # bind to ANTHROPIC_API_KEY via env.yaml
//	base_url = ""                   # optional custom endpoint
package anthropic

import (
	"context"
	"errors"
	"os"

	"charm.land/fantasy"
	fantasyanthropic "charm.land/fantasy/providers/anthropic"

	"github.com/jordanbrauer/hex/config"
)

// Name identifies this provider in the hex/ai config namespace.
const Name = "anthropic"

// Factory builds a fantasy.Provider from the "ai.anthropic.*" config
// namespace. Consumer factories add this under the key "anthropic"
// in hex/ai/provider.Provider.Factories.
func Factory(_ context.Context, store *config.Store) (fantasy.Provider, error) {
	apiKey := store.String("ai.anthropic.api_key")
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	if apiKey == "" {
		return nil, errors.New("ai/anthropic: api_key missing (set ai.anthropic.api_key or ANTHROPIC_API_KEY env var)")
	}

	opts := []fantasyanthropic.Option{
		fantasyanthropic.WithAPIKey(apiKey),
	}

	if v := store.String("ai.anthropic.base_url"); v != "" {
		opts = append(opts, fantasyanthropic.WithBaseURL(v))
	}

	return fantasyanthropic.New(opts...)
}
