-- Type stub for the `cache` Lua module installed by hex/cache/provider.

local record cache
    get: function(key: string): (string, boolean, string)
    set: function(key: string, value: string, ttl_seconds: number): (boolean, string)
    delete: function(key: string): (boolean, string)
    has: function(key: string): (boolean, string)
    clear: function(): (boolean, string)
    increment: function(key: string, delta: integer): (integer, string)
end

return cache
