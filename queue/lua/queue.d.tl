-- Type stub for the `queue` Lua module installed by hex/queue/provider.

local record queue
    publish: function(topic: string, body: string): (string, string)
end

return queue
