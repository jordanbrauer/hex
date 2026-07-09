package lua_test

import (
	"testing"

	"github.com/jordanbrauer/hex/events"
	eventslua "github.com/jordanbrauer/hex/events/lua"
	hexlua "github.com/jordanbrauer/hex/lua"
)

func TestEmit_reachesGoSubscribers(t *testing.T) {
	bus := events.New()

	var receivedName any
	var receivedPayload any

	// Subscribe from Go, emit from Lua.
	bus.On("user.created", func(data ...any) error {
		receivedName = "user.created"

		if len(data) > 0 {
			receivedPayload = data[0]
		}

		return nil
	})

	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	env.PreloadModule("events", (&eventslua.Bindings{Emitter: bus}).Loader)

	err := env.ExecString(`
		local events = require("events")
		local ok, err = events.emit("user.created", { id = 1, name = "alice" })
		if err ~= nil then error(err) end
		if not ok then error("ok=false") end
	`, "events_emit.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if receivedName != "user.created" {
		t.Errorf("subscriber not fired: %v", receivedName)
	}

	payload, ok := receivedPayload.(map[string]any)
	if !ok {
		t.Fatalf("payload not a map: %T %v", receivedPayload, receivedPayload)
	}

	if payload["name"] != "alice" {
		t.Errorf("name: %v", payload["name"])
	}

	if payload["id"] != int64(1) {
		t.Errorf("id: %T %v", payload["id"], payload["id"])
	}
}

func TestEmit_withoutPayload(t *testing.T) {
	bus := events.New()

	var fired bool
	bus.On("ping", func(data ...any) error {
		fired = true

		return nil
	})

	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	env.PreloadModule("events", (&eventslua.Bindings{Emitter: bus}).Loader)

	err := env.ExecString(`
		local events = require("events")
		events.emit("ping")
	`, "events_ping.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if !fired {
		t.Errorf("subscriber not fired")
	}
}
