// Package lua exposes hex/queue to Lua scripts as the "queue"
// module.
//
//	local queue = require("queue")
//
//	local id, err = queue.publish("send_email", '{"to":"a@b.c"}')
//
// v1 is publish-only. Subscribing from Lua ("queue.subscribe(topic,
// function(msg) ... end)") is a follow-up: bridging a
// long-running Go goroutine to a Lua callback needs LState
// serialisation (LStates are not thread-safe), which the REPL and
// scripting use cases don't need yet. Go-side subscribers see
// Lua-published messages immediately.
//
// Message bodies are strings on the Lua side; hex/queue stores raw
// []byte. Structured payloads should be JSON-encoded before
// publish — a gopher-json module is on the follow-up list.
package lua

import (
	"context"
	"fmt"

	glua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex/queue"
)

// Bindings configures the 'queue' module.
type Bindings struct {
	// Queue is the backend to publish against. Required.
	Queue queue.Queue

	// Context, when non-nil, is used for every publish. Defaults
	// to context.Background().
	Context context.Context
}

// Loader is the gopher-lua LGFunction registered against
// env.PreloadModule("queue", b.Loader).
func (b *Bindings) Loader(L *glua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]glua.LGFunction{
		"publish": b.luaPublish,
	})
	L.Push(mod)

	return 1
}

func (b *Bindings) ctx() context.Context {
	if b.Context != nil {
		return b.Context
	}

	return context.Background()
}

// luaPublish: queue.publish(topic, body) -> (id, nil) | (nil, err)
func (b *Bindings) luaPublish(L *glua.LState) int {
	if b.Queue == nil {
		L.Push(glua.LNil)
		L.Push(glua.LString("queue.publish: no backend configured"))

		return 2
	}

	topic := L.CheckString(1)
	body := L.CheckString(2)

	id, err := b.Queue.Publish(b.ctx(), topic, []byte(body))
	if err != nil {
		L.Push(glua.LNil)
		L.Push(glua.LString(fmt.Sprintf("queue.publish: %v", err)))

		return 2
	}

	L.Push(glua.LString(id))
	L.Push(glua.LNil)

	return 2
}
