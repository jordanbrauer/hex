package lua_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"

	"github.com/jordanbrauer/hex/ai"
	ailua "github.com/jordanbrauer/hex/ai/lua"
	hexlua "github.com/jordanbrauer/hex/lua"
)

// stubAgent records every Generate call and returns a canned response.
type stubAgent struct {
	mu    sync.Mutex
	calls []ai.Call
	reply string
	usage ai.Usage
	err   error
}

var _ ai.Agent = (*stubAgent)(nil)

func (s *stubAgent) Generate(_ context.Context, c ai.Call) (*ai.Result, error) {
	s.mu.Lock()
	s.calls = append(s.calls, c)
	s.mu.Unlock()

	if s.err != nil {
		return nil, s.err
	}

	reply := s.reply
	if reply == "" {
		reply = "ok: " + c.Prompt
	}

	return &ai.Result{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.TextContent{Text: reply},
			},
			FinishReason: fantasy.FinishReasonStop,
			Usage:        s.usage,
		},
		TotalUsage: s.usage,
		Steps:      []ai.StepResult{{}},
	}, nil
}

func (s *stubAgent) Stream(context.Context, ai.StreamCall) (*ai.Result, error) {
	return nil, nil
}

func (s *stubAgent) got() []ai.Call {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]ai.Call, len(s.calls))
	copy(out, s.calls)

	return out
}

// setup wires a Lua env with the agent module and returns everything
// needed to poke at it.
func setup(t *testing.T, b *ailua.Bindings) *hexlua.Environment {
	t.Helper()

	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	env.PreloadModule("agent", b.Loader)

	return env
}

func exec(t *testing.T, env *hexlua.Environment, script string) {
	t.Helper()

	if err := env.ExecString(script, "test.lua"); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func TestAgentAsk_simplePrompt(t *testing.T) {
	agent := &stubAgent{reply: "hi human"}

	env := setup(t, &ailua.Bindings{Agent: agent})

	exec(t, env, `
		local agent = require("agent")
		local response, err = agent.ask("session-1", "hello")
		if err ~= nil then error(err) end
		if response.text ~= "hi human" then
			error("bad text: " .. response.text)
		end
	`)

	calls := agent.got()
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}

	if calls[0].Prompt != "hello" {
		t.Errorf("prompt = %q, want %q", calls[0].Prompt, "hello")
	}

	if calls[0].Messages != nil {
		t.Errorf("Messages should be nil for single-turn (no store), got %v", calls[0].Messages)
	}
}

func TestAgentAsk_optionsTable(t *testing.T) {
	agent := &stubAgent{}

	env := setup(t, &ailua.Bindings{Agent: agent})

	exec(t, env, `
		local agent = require("agent")
		local _, err = agent.ask("s", "compute", {
		    temperature = 0.25,
		    max_tokens = 42,
		    tools = { "get_incident", "post_slack" },
		})
		if err ~= nil then error(err) end
	`)

	calls := agent.got()
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}

	c := calls[0]

	if c.Temperature == nil || *c.Temperature != 0.25 {
		t.Errorf("Temperature = %v, want 0.25", c.Temperature)
	}

	if c.MaxOutputTokens == nil || *c.MaxOutputTokens != 42 {
		t.Errorf("MaxOutputTokens = %v, want 42", c.MaxOutputTokens)
	}

	wantTools := []string{"get_incident", "post_slack"}
	if len(c.ActiveTools) != len(wantTools) {
		t.Fatalf("ActiveTools len = %d, want %d", len(c.ActiveTools), len(wantTools))
	}

	for i, name := range wantTools {
		if c.ActiveTools[i] != name {
			t.Errorf("ActiveTools[%d] = %q, want %q", i, c.ActiveTools[i], name)
		}
	}
}

func TestAgentAsk_conversationStorePersistsHistory(t *testing.T) {
	agent := &stubAgent{reply: "second"}
	store := ai.NewMemoryConversationStore()

	env := setup(t, &ailua.Bindings{Agent: agent, Store: store})

	// First turn: no prior history.
	exec(t, env, `
		local agent = require("agent")
		local _, err = agent.ask("thread-1", "first message")
		if err ~= nil then error(err) end
	`)

	// Second turn: should carry the first turn as prior context.
	exec(t, env, `
		local agent = require("agent")
		local _, err = agent.ask("thread-1", "second message")
		if err ~= nil then error(err) end
	`)

	calls := agent.got()
	if len(calls) != 2 {
		t.Fatalf("want 2 calls, got %d", len(calls))
	}

	// First call: no history yet.
	if got := len(calls[0].Messages); got != 0 {
		t.Errorf("first call Messages len = %d, want 0", got)
	}

	// Second call: should see user+assistant from first turn.
	if got := len(calls[1].Messages); got != 2 {
		t.Fatalf("second call Messages len = %d, want 2 (user+assistant)", got)
	}

	if calls[1].Messages[0].Role != fantasy.MessageRoleUser {
		t.Errorf("history[0].Role = %v, want user", calls[1].Messages[0].Role)
	}

	if calls[1].Messages[1].Role != fantasy.MessageRoleAssistant {
		t.Errorf("history[1].Role = %v, want assistant", calls[1].Messages[1].Role)
	}
}

func TestAgentAsk_forgetClearsHistory(t *testing.T) {
	agent := &stubAgent{}
	store := ai.NewMemoryConversationStore()

	env := setup(t, &ailua.Bindings{Agent: agent, Store: store})

	exec(t, env, `
		local agent = require("agent")
		local _, err = agent.ask("thread-1", "hi")
		if err ~= nil then error(err) end

		agent.forget("thread-1")

		local _, err2 = agent.ask("thread-1", "hi again")
		if err2 ~= nil then error(err2) end
	`)

	calls := agent.got()
	if len(calls) != 2 {
		t.Fatalf("want 2 calls, got %d", len(calls))
	}

	if got := len(calls[1].Messages); got != 0 {
		t.Errorf("second call after forget: Messages len = %d, want 0", got)
	}
}

func TestAgentTools_listsRegistryNames(t *testing.T) {
	agent := &stubAgent{}

	getWeather := ai.NewTool("get_weather", "look up weather",
		func(_ context.Context, _ struct{}, _ ai.ToolCall) (ai.ToolResponse, error) {
			return ai.ToolResponse{}, nil
		})
	postSlack := ai.NewTool("post_slack", "post to slack",
		func(_ context.Context, _ struct{}, _ ai.ToolCall) (ai.ToolResponse, error) {
			return ai.ToolResponse{}, nil
		})

	registry := ai.NewToolRegistry(getWeather, postSlack)

	env := setup(t, &ailua.Bindings{Agent: agent, Registry: registry})

	// agent.tools() should return both, sorted alphabetically.
	exec(t, env, `
		local agent = require("agent")
		local tools = agent.tools()
		if #tools ~= 2 then
			error("want 2 tools, got " .. #tools)
		end
		if tools[1] ~= "get_weather" then
			error("tools[1] = " .. tools[1])
		end
		if tools[2] ~= "post_slack" then
			error("tools[2] = " .. tools[2])
		end
	`)
}

func TestAgentAsk_errorFromAgent(t *testing.T) {
	agent := &stubAgent{err: errFake("network down")}

	env := setup(t, &ailua.Bindings{Agent: agent})

	// Lua should receive (nil, error string) and NOT panic.
	exec(t, env, `
		local agent = require("agent")
		local response, err = agent.ask("s", "hi")
		if response ~= nil then error("response should be nil on error") end
		if err == nil or not string.find(err, "network down") then
			error("bad err: " .. tostring(err))
		end
	`)
}

func TestAgentAsk_mutexSerialisesConcurrentCalls(t *testing.T) {
	// Two goroutines. Each runs its own script that calls agent.ask.
	// Without a mutex around LState use, gopher-lua would race.
	agent := &stubAgent{}
	mu := &sync.Mutex{}

	// Two SEPARATE envs (each goroutine owns its LState) using the
	// same agent + mutex around Generate. This models the pattern
	// where hex/lua/provider runs one env per handler dispatch.
	envA := hexlua.New()
	envB := hexlua.New()
	t.Cleanup(func() { _ = envA.Close(); _ = envB.Close() })

	b := &ailua.Bindings{Agent: agent, Mutex: mu}
	envA.PreloadModule("agent", b.Loader)
	envB.PreloadModule("agent", b.Loader)

	script := `
		local agent = require("agent")
		for i = 1, 10 do
			local _, err = agent.ask("s", "call " .. i)
			if err ~= nil then error(err) end
		end
	`

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		if err := envA.ExecString(script, "a.lua"); err != nil {
			t.Errorf("envA: %v", err)
		}
	}()

	go func() {
		defer wg.Done()

		if err := envB.ExecString(script, "b.lua"); err != nil {
			t.Errorf("envB: %v", err)
		}
	}()

	wg.Wait()

	if got := len(agent.got()); got != 20 {
		t.Errorf("total calls = %d, want 20", got)
	}
}

// errFake is a stand-in error type used only in tests.
type errFake string

func (e errFake) Error() string { return string(e) }

// Silences unused-imports on strings when I trim helpers.
var _ = strings.Contains
