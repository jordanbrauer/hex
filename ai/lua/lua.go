// Package lua exposes hex/ai's default agent to Lua scripts via a
// gopher-lua module named "agent".
//
// The module is loaded by a service provider that resolves the shared
// *hex/lua.Environment from the container (bound by hex/lua/provider),
// resolves the default ai.Agent, and calls env.PreloadModule("agent",
// ...). Consumers add the provider to their boot.go alongside the base
// Lua provider:
//
//	provider.Lua(),        // hex/lua/provider
//	provider.LuaAI(),      // hex/ai/lua/provider — this package
//
// After boot, any Lua script running in the environment can:
//
//	local agent = require("agent")
//
//	-- Simple ask, no conversation history:
//	local response, err = agent.ask("session-1", "Summarise this incident.")
//
//	-- With options — subset of tools, temperature, max tokens:
//	local response, err = agent.ask(thread_ts, event.text, {
//	    tools       = { "get_incident", "post_slack" },
//	    temperature = 0.3,
//	    max_tokens  = 500,
//	})
//
//	-- Introspection:
//	for _, name in ipairs(agent.tools()) do
//	    print("available:", name)
//	end
//
//	-- Reset a conversation:
//	agent.forget(thread_ts)
//
// Response tables include:
//
//	response.text         string        -- flattened assistant text
//	response.usage.input_tokens        int
//	response.usage.output_tokens       int
//	response.usage.total_tokens        int
//	response.model        string        -- (best-effort from steps)
//
// Errors return (nil, "message").
//
// Concurrency:
//
//	*lua.LState is not thread-safe. When multiple goroutines may call
//	agent.ask() against the same environment (event bus handlers,
//	web request handlers, etc.), pass Bindings.Mutex so this package
//	guards the Generate call. See PLAT-3545 notes.
package lua

import (
	"context"
	"fmt"
	"sync"

	"charm.land/fantasy"
	glua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex/ai"
)

// Bindings configures the 'agent' module. Constructed and installed by
// hex/ai/lua/provider; callers who want to wire the module manually
// (outside the provider lifecycle) build one directly and call Loader.
type Bindings struct {
	// Agent is the fantasy.Agent to call. Required.
	Agent ai.Agent

	// Registry is consulted for tool introspection (agent.tools()) and
	// for validating ActiveTools passed from Lua. Optional; when nil
	// the module trusts whatever tool names Lua supplies.
	Registry ai.ToolRegistry

	// Store persists multi-turn conversation history keyed by the
	// first argument to agent.ask. Optional; when nil each call is a
	// fresh single-turn interaction (Prompt only, no Messages).
	Store ai.ConversationStore

	// Mutex, when non-nil, is locked around every Generate call. Use
	// this to serialise concurrent agent.ask invocations that share
	// the same *lua.LState (event bus subscribers, web handlers).
	Mutex *sync.Mutex

	// Context, when non-nil, is used for every agent call. Defaults
	// to context.Background(). Consumers who need per-call ctx (e.g.
	// from an HTTP request) install a fresh Bindings per request or
	// stash the ctx in the LState registry themselves.
	Context context.Context
}

// Loader is the gopher-lua LGFunction registered against
// env.PreloadModule("agent", b.Loader).
func (b *Bindings) Loader(L *glua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]glua.LGFunction{
		"ask":    b.luaAsk,
		"tools":  b.luaTools,
		"forget": b.luaForget,
	})
	L.Push(mod)

	return 1
}

// luaAsk: agent.ask(conversation_id, text, opts?) -> table, err
func (b *Bindings) luaAsk(L *glua.LState) int {
	if b.Agent == nil {
		return pushError(L, "agent.ask: no agent configured")
	}

	convID := L.CheckString(1)
	text := L.CheckString(2)

	call := ai.Call{Prompt: text}

	if L.GetTop() >= 3 {
		if opts, ok := L.Get(3).(*glua.LTable); ok {
			applyOptsToCall(&call, opts)
		}
	}

	ctx := b.Context
	if ctx == nil {
		ctx = context.Background()
	}

	// Prepend prior history if a store is configured.
	if b.Store != nil {
		history, err := b.Store.Load(ctx, convID)
		if err != nil {
			return pushError(L, fmt.Sprintf("agent.ask: load history: %v", err))
		}

		call.Messages = history
	}

	if b.Mutex != nil {
		b.Mutex.Lock()
		defer b.Mutex.Unlock()
	}

	result, err := b.Agent.Generate(ctx, call)
	if err != nil {
		return pushError(L, fmt.Sprintf("agent.ask: %v", err))
	}

	if b.Store != nil {
		updated := appendTurn(call.Messages, text, result)
		if saveErr := b.Store.Save(ctx, convID, updated); saveErr != nil {
			// History save failure is not fatal — the model already
			// answered. Surface via error but keep the response
			// available for the caller by returning them BOTH.
			L.Push(marshalResult(L, result))
			L.Push(glua.LString(fmt.Sprintf("agent.ask: save history: %v", saveErr)))

			return 2
		}
	}

	L.Push(marshalResult(L, result))
	L.Push(glua.LNil)

	return 2
}

// luaTools: agent.tools() -> {name, name, ...}
func (b *Bindings) luaTools(L *glua.LState) int {
	tbl := L.NewTable()

	if b.Registry == nil {
		L.Push(tbl)

		return 1
	}

	for i, name := range b.Registry.Names() {
		tbl.RawSetInt(i+1, glua.LString(name))
	}

	L.Push(tbl)

	return 1
}

// luaForget: agent.forget(conversation_id) -> err | nil
func (b *Bindings) luaForget(L *glua.LState) int {
	if b.Store == nil {
		L.Push(glua.LNil)

		return 1
	}

	id := L.CheckString(1)

	ctx := b.Context
	if ctx == nil {
		ctx = context.Background()
	}

	if err := b.Store.Delete(ctx, id); err != nil {
		L.Push(glua.LString(fmt.Sprintf("agent.forget: %v", err)))

		return 1
	}

	L.Push(glua.LNil)

	return 1
}

// applyOptsToCall reads the options table Lua passes to agent.ask and
// threads recognised fields into the Call. Unknown keys are ignored so
// scripts can carry extra metadata without upsetting the module.
func applyOptsToCall(call *ai.Call, opts *glua.LTable) {
	if v := opts.RawGetString("temperature"); v.Type() == glua.LTNumber {
		f := float64(v.(glua.LNumber))
		call.Temperature = &f
	}

	if v := opts.RawGetString("top_p"); v.Type() == glua.LTNumber {
		f := float64(v.(glua.LNumber))
		call.TopP = &f
	}

	if v := opts.RawGetString("max_tokens"); v.Type() == glua.LTNumber {
		n := int64(v.(glua.LNumber))
		call.MaxOutputTokens = &n
	}

	if v := opts.RawGetString("tools"); v.Type() == glua.LTTable {
		var names []string

		v.(*glua.LTable).ForEach(func(_, val glua.LValue) {
			if s, ok := val.(glua.LString); ok {
				names = append(names, string(s))
			}
		})

		call.ActiveTools = names
	}
}

// marshalResult flattens a fantasy.AgentResult into a Lua table with the
// fields Lua scripts actually need. Steps are omitted from the top
// level — a caller who wants the raw trace can extend Bindings with a
// custom marshaller.
func marshalResult(L *glua.LState, result *ai.Result) *glua.LTable {
	tbl := L.NewTable()

	tbl.RawSetString("text", glua.LString(result.Response.Content.Text()))

	usage := L.NewTable()
	usage.RawSetString("input_tokens", glua.LNumber(result.TotalUsage.InputTokens))
	usage.RawSetString("output_tokens", glua.LNumber(result.TotalUsage.OutputTokens))
	usage.RawSetString("total_tokens", glua.LNumber(result.TotalUsage.TotalTokens))
	tbl.RawSetString("usage", usage)

	// Steps count as a diagnostic aid.
	tbl.RawSetString("steps", glua.LNumber(len(result.Steps)))

	return tbl
}

// appendTurn builds the new history slice: prior messages, the user's
// current turn as a user message, and the assistant's response(s)
// derived from the final step's content.
func appendTurn(history []ai.Message, userText string, result *ai.Result) []ai.Message {
	msgs := make([]ai.Message, 0, len(history)+2)
	msgs = append(msgs, history...)

	msgs = append(msgs, ai.Message{
		Role: fantasy.MessageRoleUser,
		Content: []ai.MessagePart{
			fantasy.TextPart{Text: userText},
		},
	})

	if txt := result.Response.Content.Text(); txt != "" {
		msgs = append(msgs, ai.Message{
			Role: fantasy.MessageRoleAssistant,
			Content: []ai.MessagePart{
				fantasy.TextPart{Text: txt},
			},
		})
	}

	return msgs
}

// pushError pushes (nil, "message") on the Lua stack and returns 2 so
// module functions can return early with a single call.
func pushError(L *glua.LState, msg string) int {
	L.Push(glua.LNil)
	L.Push(glua.LString(msg))

	return 2
}
