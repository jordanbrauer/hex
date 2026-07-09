-- Type stub for the `log` Lua module installed by hex/lua/provider.

local record log
    debug: function(msg: string, attrs: {string:any})
    info: function(msg: string, attrs: {string:any})
    warn: function(msg: string, attrs: {string:any})
    error: function(msg: string, attrs: {string:any})
end

return log
