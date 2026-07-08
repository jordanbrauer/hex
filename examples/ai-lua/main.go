// Command ai-lua-demo boots a minimal hex application with the AI +
// Lua + AI/Lua service providers wired, then executes a Lua script
// from stdin (or the argument given).
//
// The script has the 'agent' module preloaded. It can call agent.ask,
// agent.tools, agent.forget and print the results.
//
// Usage:
//
//	OPENAI_API_KEY=sk-... go run . <examples/ai-lua/ask.lua
//	OPENAI_API_KEY=sk-... go run . examples/ai-lua/ask.lua
//
// Environment overrides:
//
//	AI_PROVIDER      openai | anthropic (default openai)
//	AI_MODEL         model id (default gpt-4o-mini for openai)
//	OPENAI_API_KEY   read by hex/ai/openai
//	ANTHROPIC_API_KEY read by hex/ai/anthropic
//
// Swap the openai import + Factories entry for Anthropic (or any
// hex/ai/<name> subpackage) to point at a different provider.
package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"testing/fstest"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/ai/anthropic"
	ailuaprovider "github.com/jordanbrauer/hex/ai/lua/provider"
	"github.com/jordanbrauer/hex/ai/openai"
	aiprovider "github.com/jordanbrauer/hex/ai/provider"
	configprovider "github.com/jordanbrauer/hex/config/provider"
	"github.com/jordanbrauer/hex/container"
	hexlog "github.com/jordanbrauer/hex/log"
	hexlua "github.com/jordanbrauer/hex/lua"
	luaprovider "github.com/jordanbrauer/hex/lua/provider"
)

func main() {
	// Silence framework INFO chatter so the Lua script's print output
	// is easier to see. Override with LOG_LEVEL=debug to trace.
	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		if parsed, err := hexlog.ParseLevel(lvl); err == nil {
			hexlog.Init(hexlog.WithLevel(parsed))
		}
	} else {
		hexlog.Init(hexlog.WithLevel(hexlog.ErrorLevel))
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	providerName := envOr("AI_PROVIDER", "openai")
	modelID := envOr("AI_MODEL", defaultModel(providerName))

	kernel := hex.New()

	err := kernel.Register(
		&configprovider.Provider{
			Sources: []fs.FS{
				aiprovider.Configs(),
				inlineConfig(providerName, modelID),
			},
		},
		&luaprovider.Provider{},
		&aiprovider.Provider{
			Factories: map[string]aiprovider.Factory{
				openai.Name:    openai.Factory,
				anthropic.Name: anthropic.Factory,
			},
		},
		&ailuaprovider.Provider{},
	)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}

	ctx := context.Background()

	if err := kernel.Bootstrap(ctx); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	defer func() { _ = kernel.Shutdown(ctx) }()

	env, err := container.Make[*hexlua.Environment](kernel, "lua")
	if err != nil {
		return fmt.Errorf("resolve lua: %w", err)
	}

	source, name, err := readScript()
	if err != nil {
		return err
	}

	return env.ExecString(source, name)
}

// inlineConfig returns a fs.FS containing a minimal ai.toml that
// overrides the framework defaults with the caller's chosen provider
// + model. Real apps would use a real config directory.
func inlineConfig(providerName, modelID string) fs.FS {
	toml := fmt.Sprintf(`provider = %q
model = %q
`, providerName, modelID)

	return fstest.MapFS{
		"ai.toml": &fstest.MapFile{Data: []byte(toml)},
	}
}

func defaultModel(provider string) string {
	switch provider {
	case "anthropic":
		return "claude-3-5-sonnet-20241022"
	default:
		return "gpt-4o-mini"
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

// readScript reads Lua source from either os.Args[1] or stdin.
func readScript() (string, string, error) {
	if len(os.Args) >= 2 {
		data, err := os.ReadFile(os.Args[1])
		if err != nil {
			return "", "", err
		}

		return string(data), os.Args[1], nil
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", "", err
	}

	return string(data), "<stdin>", nil
}
