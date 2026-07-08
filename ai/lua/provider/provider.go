// Package provider is the service provider that installs the hex/ai
// 'agent' Lua module into the shared hex/lua environment.
//
// Add to app/boot.go AFTER both hex/ai/provider (binds "ai") and
// hex/lua/provider (binds "lua"):
//
//	provider.Config(),
//	provider.Log(),
//	provider.Lua(),        // hex/lua/provider
//	provider.AI(),         // hex/ai/provider
//	provider.LuaAI(),      // this package — bridges the two
//
// Register order: Lua and AI must both have Registered before this
// provider's Register runs (both bindings must exist in the container).
package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jordanbrauer/hex/ai"
	ailua "github.com/jordanbrauer/hex/ai/lua"
	"github.com/jordanbrauer/hex/container"
	hexlua "github.com/jordanbrauer/hex/lua"
	"github.com/jordanbrauer/hex/provider"
)

// Provider wires the 'agent' Lua module into hex/lua's shared env.
type Provider struct {
	provider.Base

	// LuaBinding is the container key for the *hex/lua.Environment.
	// Defaults to "lua".
	LuaBinding string

	// AgentBinding is the container key for the ai.Agent. Defaults to
	// "ai".
	AgentBinding string

	// RegistryBinding is the container key for the ai.ToolRegistry.
	// Defaults to "ai.tools". Falls back to Registry when the binding
	// is missing.
	RegistryBinding string

	// ModuleName is the Lua module name registered via
	// env.PreloadModule. Defaults to "agent". Change when the
	// consumer wants to expose multiple agents under distinct names.
	ModuleName string

	// Registry is a fallback exposed to Lua via agent.tools() when the
	// container has no RegistryBinding. Typical usage relies on the
	// container-bound registry from hex/ai/provider; this field is
	// only useful when wiring the Lua module without the standard AI
	// provider.
	Registry ai.ToolRegistry

	// Store persists conversation history keyed by the first argument
	// to agent.ask. Optional; when nil each call is single-turn.
	Store ai.ConversationStore

	// Mutex is locked around every Generate call so concurrent
	// agent.ask invocations from event handlers do not race on the
	// shared *lua.LState. When nil, the provider allocates its own
	// mutex — this is the correct default for event-bus dispatched
	// scripts (see PLAT-3545).
	Mutex *sync.Mutex
}

// Register resolves both bindings and installs the module.
func (p *Provider) Register(app provider.Application) error {
	luaBinding := p.LuaBinding
	if luaBinding == "" {
		luaBinding = "lua"
	}

	agentBinding := p.AgentBinding
	if agentBinding == "" {
		agentBinding = "ai"
	}

	moduleName := p.ModuleName
	if moduleName == "" {
		moduleName = "agent"
	}

	registryBinding := p.RegistryBinding
	if registryBinding == "" {
		registryBinding = "ai.tools"
	}

	env, err := container.Make[*hexlua.Environment](app, luaBinding)
	if err != nil {
		return fmt.Errorf("ai/lua/provider: resolve %q: %w", luaBinding, err)
	}

	if env == nil {
		return errors.New("ai/lua/provider: hex/lua environment is nil (register hex/lua/provider first)")
	}

	agent, err := container.Make[ai.Agent](app, agentBinding)
	if err != nil {
		return fmt.Errorf("ai/lua/provider: resolve %q: %w", agentBinding, err)
	}

	registry := p.Registry
	if r, err := container.Make[ai.ToolRegistry](app, registryBinding); err == nil {
		registry = r
	}

	mu := p.Mutex
	if mu == nil {
		mu = &sync.Mutex{}
	}

	bindings := &ailua.Bindings{
		Agent:    agent,
		Registry: registry,
		Store:    p.Store,
		Mutex:    mu,
	}

	env.PreloadModule(moduleName, bindings.Loader)

	return nil
}

// Compile-time check the provider does not need Shutdowner semantics —
// the shared Lua env is closed by hex/lua/provider; conversation store
// cleanup is the store's own concern.
var _ provider.Service = (*Provider)(nil)

// Silence unused-import in test scaffolds that may not exercise ctx.
var _ = context.Background
