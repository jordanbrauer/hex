-- Introspect the tool registry (empty in this demo — the ai-lua-demo
-- binary doesn't wire any tools). If you extend the demo to register
-- tools via hex/ai/provider.Provider.Tools, this script will print
-- them.
local agent = require("agent")

local tools = agent.tools()
print(string.format("registered tools: %d", #tools))
for i, name in ipairs(tools) do
    print(string.format("  %d. %s", i, name))
end
