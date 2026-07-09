package lua_test

import (
	"context"
	"sync"
	"testing"
	"time"

	hexlua "github.com/jordanbrauer/hex/lua"
	"github.com/jordanbrauer/hex/queue"
	queuelua "github.com/jordanbrauer/hex/queue/lua"
	"github.com/jordanbrauer/hex/queue/memory"
)

func TestPublish_reachesGoSubscriber(t *testing.T) {
	q := memory.New(memory.Options{})
	t.Cleanup(func() { _ = q.Close(context.Background()) })

	var (
		mu       sync.Mutex
		received string
		done     = make(chan struct{})
	)

	sub, err := q.Subscribe(context.Background(), "emails", func(ctx context.Context, msg *queue.Message) error {
		mu.Lock()
		received = string(msg.Body)
		mu.Unlock()
		close(done)

		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	t.Cleanup(func() { _ = sub.Close(context.Background()) })

	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	env.PreloadModule("queue", (&queuelua.Bindings{Queue: q}).Loader)

	err = env.ExecString(`
		local queue = require("queue")
		local id, err = queue.publish("emails", "hello")
		if err ~= nil then error(err) end
		if id == nil or #id == 0 then error("missing id") end
	`, "queue_publish.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber never fired")
	}

	mu.Lock()
	defer mu.Unlock()

	if received != "hello" {
		t.Errorf("received=%q", received)
	}
}
