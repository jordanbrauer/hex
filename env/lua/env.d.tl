-- Type stub for the `env` Lua module installed by hex/lua/provider.

local record env
    name: string
    current: function(): string
    is_development: function(): boolean
    is_test: function(): boolean
    is_production: function(): boolean
end

return env
