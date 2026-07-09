-- Type stub for the `events` Lua module installed by hex/lua/provider.

local record events
    emit: function(name: string, payload: any): (boolean, string)
end

return events
