-- Type stub for the `config` Lua module installed by hex/lua/provider.

local record config
    string: function(key: string): string
    int: function(key: string): integer
    bool: function(key: string): boolean
    float: function(key: string): number
    duration: function(key: string): number
    stringSlice: function(key: string): {string}
    set: function(key: string, value: any): (boolean, string)
    has: function(key: string): boolean
    namespaces: function(): {string}
end

return config
