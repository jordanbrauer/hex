# ai-lua-demo

Playground that boots the minimum hex application needed to drive
`hex/ai`'s default agent from Lua.

## Build & run

```bash
cd examples/ai-lua

# Single-turn ask (defaults to OpenAI gpt-4o-mini).
OPENAI_API_KEY=sk-... go run . ask.lua

# Swap to Anthropic.
AI_PROVIDER=anthropic AI_MODEL=claude-3-5-sonnet-20241022 \
  ANTHROPIC_API_KEY=sk-ant-... go run . ask.lua

# Read the script from stdin.
OPENAI_API_KEY=sk-... echo '
    local a = require("agent")
    local r, err = a.ask("s", "Say hi in one word.")
    print(err or r.text)
' | go run .

# See framework logs.
LOG_LEVEL=debug OPENAI_API_KEY=sk-... go run . ask.lua
```

## Scripts

- **`ask.lua`** — single call, prints response + usage stats.
- **`chat.lua`** — demonstrates multi-turn call shape (no store wired
  in this demo, so history isn't persisted between runs).
- **`tools.lua`** — introspects `agent.tools()`; empty here because
  the demo doesn't register any.

## Lua module surface

```lua
local agent = require("agent")

-- Basic ask.
local response, err = agent.ask("conversation_id", "prompt")

-- With options.
local response, err = agent.ask("conversation_id", "prompt", {
    tools        = { "tool_name", ... },   -- filter agent's registered tools
    temperature  = 0.7,
    max_tokens   = 500,
    top_p        = 0.95,
})

-- Response table.
--
--   response.text         string
--   response.usage.input_tokens         int
--   response.usage.output_tokens        int
--   response.usage.total_tokens         int
--   response.steps        int   (number of agent steps taken)

-- Introspect the tool registry.
local names = agent.tools()  -- table of strings

-- Clear a conversation's history.
agent.forget("conversation_id")
```

## Wiring in your own app

```go
kernel.Register(
    &configprovider.Provider{Sources: ...},  // includes aiprovider.Configs()
    &luaprovider.Provider{},                 // hex/lua/provider
    &aiprovider.Provider{                    // hex/ai/provider
        Factories: map[string]aiprovider.Factory{
            openai.Name:    openai.Factory,
            anthropic.Name: anthropic.Factory,
        },
        Tools:    []ai.Tool{ /* your tools */ },
    },
    &ailuaprovider.Provider{                 // hex/ai/lua/provider
        Store: ai.NewMemoryConversationStore(),
    },
)
```

The AI/Lua provider resolves both `"lua"` and `"ai"` from the
container, so registration order only matters in that `hex/lua/provider`
and `hex/ai/provider` must both register before `hex/ai/lua/provider`.
Any third-party provider that adds Lua modules or globals does the
same thing — resolve `"lua"`, call `env.PreloadModule` /
`env.SetGlobal` in its own Register.
