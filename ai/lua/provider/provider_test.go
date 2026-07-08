package provider_test

import (
	"context"
	"testing"

	"charm.land/fantasy"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/ai"
	ailuaprovider "github.com/jordanbrauer/hex/ai/lua/provider"
	"github.com/jordanbrauer/hex/container"
	hexlua "github.com/jordanbrauer/hex/lua"
	luaprovider "github.com/jordanbrauer/hex/lua/provider"
	"github.com/jordanbrauer/hex/provider"
)

// injectAgent is a tiny provider that binds a caller-supplied ai.Agent
// under "ai" so we can run the LuaAI provider without spinning up a
// real fantasy.Provider. Purely a test helper.
type injectAgent struct {
	provider.Base

	Agent    ai.Agent
	Registry ai.ToolRegistry
}

func (p *injectAgent) Register(app provider.Application) error {
	app.Singleton("ai", func(*container.Container) (any, error) {
		return p.Agent, nil
	})

	if p.Registry != nil {
		app.Singleton("ai.tools", func(*container.Container) (any, error) {
			return p.Registry, nil
		})
	}

	return nil
}

// recorderAgent captures every Generate call.
type recorderAgent struct {
	calls []ai.Call
}

var _ ai.Agent = (*recorderAgent)(nil)

func (r *recorderAgent) Generate(_ context.Context, c ai.Call) (*ai.Result, error) {
	r.calls = append(r.calls, c)

	return &ai.Result{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.TextContent{Text: "ack: " + c.Prompt},
			},
			FinishReason: fantasy.FinishReasonStop,
		},
	}, nil
}

func (r *recorderAgent) Stream(context.Context, ai.StreamCall) (*ai.Result, error) {
	return nil, nil
}

func TestProvider_endToEndBoot(t *testing.T) {
	agent := &recorderAgent{}
	registry := ai.NewToolRegistry(
		ai.NewTool("noop", "does nothing",
			func(_ context.Context, _ struct{}, _ ai.ToolCall) (ai.ToolResponse, error) {
				return ai.ToolResponse{}, nil
			}),
	)

	kernel := hex.New()

	err := kernel.Register(
		&luaprovider.Provider{},
		&injectAgent{Agent: agent, Registry: registry},
		&ailuaprovider.Provider{},
	)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := kernel.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	t.Cleanup(func() { _ = kernel.Shutdown(context.Background()) })

	// Resolve the env and prove that agent.ask + agent.tools() work.
	envAny, err := kernel.Make("lua")
	if err != nil {
		t.Fatalf("Make lua: %v", err)
	}

	env, ok := envAny.(*hexlua.Environment)
	if !ok {
		t.Fatalf("lua binding = %T, want *hexlua.Environment", envAny)
	}

	err = env.ExecString(`
		local agent = require("agent")

		local tools = agent.tools()
		if #tools ~= 1 or tools[1] ~= "noop" then
			error("bad tools: " .. table.concat(tools, ","))
		end

		local response, err = agent.ask("session-x", "ping")
		if err ~= nil then error(err) end

		if response.text ~= "ack: ping" then
			error("bad text: " .. response.text)
		end
	`, "boot_test.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if len(agent.calls) != 1 {
		t.Fatalf("agent calls = %d, want 1", len(agent.calls))
	}
}
