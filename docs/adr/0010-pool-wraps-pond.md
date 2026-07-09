# hex/pool wraps alitto/pond

`hex/pool` is a thin wrapper around github.com/alitto/pond. Pond provides dynamic worker sizing, task groups with context, panic recovery, metrics (running/queued/completed/failed), and non-blocking task submission — all things we would end up rewriting.

This matches the pattern established for other subsystems: hex/cron wraps robfig/cron, hex/log wraps charmbracelet/log, hex/web wraps labstack/echo, hex/lua wraps yuin/gopher-lua. The wrapper exists to give the hex codebase a consistent naming convention, provider-friendly lifecycle (Stop/StopAndWait), and one place to swap the implementation later.

We rejected rolling our own (goroutines + a semaphore channel) because pond's dynamic sizing, task groups, and metrics surface are non-trivial and well-tested — reproducing them would burn time on solved problems.

## Not integrated into hex/queue yet

Queue subscriptions currently spawn one goroutine per subscription. Adding a per-subscription concurrency knob (`SubscribeOptions{Concurrency: N}`) that uses hex/pool is a natural next step, but we defer it until real consumers surface the actual access pattern. Consumers who want concurrent queue handling today wrap their handler in a `pool.Submit` call themselves.
