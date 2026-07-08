-- Single-turn ask against the default agent.
local agent = require("agent")

local response, err = agent.ask("demo-session", "Write a haiku about queues.")
if err ~= nil then
    error(err)
end

print(response.text)
print(string.format(
    "usage: %d in, %d out, %d total (%d steps)",
    response.usage.input_tokens,
    response.usage.output_tokens,
    response.usage.total_tokens,
    response.steps
))
