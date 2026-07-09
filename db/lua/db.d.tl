-- Type stub for the `db` Lua module installed by hex/db/provider.
-- Read at REPL / hex-run startup so Teal typechecks require("db").

local record Result
    rows_affected: integer
    last_insert_id: integer
end

local record Executor
    query: function(sql: string, ...: any): ({{string:any}}, string)
    queryOne: function(sql: string, ...: any): ({string:any}, string)
    exec: function(sql: string, ...: any): (Result, string)
end

local record Module
    query: function(sql: string, ...: any): ({{string:any}}, string)
    queryOne: function(sql: string, ...: any): ({string:any}, string)
    exec: function(sql: string, ...: any): (Result, string)
    transaction: function(function(Executor)): (boolean, string)
end

return Module
