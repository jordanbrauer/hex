-- Multi-turn conversation. Since hex/ai/lua only wires a
-- ConversationStore when the provider is configured with one, this
-- script only demonstrates that the API accepts a conversation id and
-- returns coherent single-turn output. See the provider docs for
-- persistence.
local agent = require("agent")

local function ask(text)
    local response, err = agent.ask("chat-1", text, {
        temperature = 0.4,
        max_tokens = 200,
    })
    if err ~= nil then
        error(err)
    end
    return response.text
end

print("Q: What is the airspeed velocity of an unladen swallow?")
print("A: " .. ask("What is the airspeed velocity of an unladen swallow?"))
