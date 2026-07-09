// Package lua exposes hex/db to Lua scripts via a gopher-lua module
// named "db".
//
// The module is installed by hex/db/lua/provider, which resolves the
// shared *hex/lua.Environment from the container (bound by
// hex/lua/provider) and the *sql.DB (bound by hex/db/provider) and
// calls env.PreloadModule("db", ...).
//
// Consumer scripts (including the REPL) can then:
//
//	local db = require("db")
//
//	-- Many rows:
//	local rows, err = db.query("SELECT id, name FROM users WHERE active = ?", true)
//	for _, row in ipairs(rows) do
//	    print(row.id, row.name)
//	end
//
//	-- Single row (returns nil, nil when no rows match):
//	local u, err = db.queryOne("SELECT * FROM users WHERE id = ?", 1)
//	if u ~= nil then print(u.email) end
//
//	-- Writes:
//	local result, err = db.exec(
//	    "UPDATE users SET name = ? WHERE id = ?", "Alice", 1)
//	print(result.rows_affected, result.last_insert_id)
//
//	-- Transactions — the callback receives a `tx` table with the
//	-- same shape (query, queryOne, exec). On error inside the
//	-- callback the tx rolls back; otherwise it commits.
//	local ok, err = db.transaction(function(tx)
//	    tx.exec("INSERT INTO widgets (name) VALUES (?)", "sprocket")
//	    tx.exec("UPDATE counters SET n = n + 1 WHERE k = 'widgets'")
//	end)
//
// Errors always return (nil, "message") — check `err` before using
// the first return value.
//
// Concurrency:
//
//	*sql.DB is safe for concurrent use, but *lua.LState is not. When
//	multiple goroutines share an LState (event handlers, web
//	request handlers), pass Bindings.Mutex to serialise access.
//	For REPL / one-off scripts the default nil mutex is fine.
package lua

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	glua "github.com/yuin/gopher-lua"
)

// Bindings configures the 'db' module. Constructed and installed by
// hex/db/lua/provider; callers wiring the module manually build one
// directly and call Loader.
type Bindings struct {
	// DB is the *sql.DB the module executes against. Required.
	DB *sql.DB

	// Mutex, when non-nil, is locked around every DB call. Use this
	// when multiple goroutines call require("db") functions against
	// the same *lua.LState. Nil is fine for the REPL and other
	// single-threaded contexts.
	Mutex *sync.Mutex

	// Context, when non-nil, is used for every DB call. Defaults to
	// context.Background().
	Context context.Context
}

// Loader is the gopher-lua LGFunction registered against
// env.PreloadModule("db", b.Loader).
func (b *Bindings) Loader(L *glua.LState) int {
	L.Push(b.buildModule(L, nil))

	return 1
}

// buildModule returns a Lua table exposing query/queryOne/exec (and
// transaction when tx is nil — nested transactions are refused).
//
// When tx is non-nil the returned module uses the transaction as its
// executor; that's the value passed to db.transaction's callback.
func (b *Bindings) buildModule(L *glua.LState, tx *sql.Tx) *glua.LTable {
	mod := L.NewTable()

	L.SetField(mod, "query", L.NewFunction(b.wrapExecutor(tx, b.luaQuery)))
	L.SetField(mod, "queryOne", L.NewFunction(b.wrapExecutor(tx, b.luaQueryOne)))
	L.SetField(mod, "exec", L.NewFunction(b.wrapExecutor(tx, b.luaExec)))

	if tx == nil {
		L.SetField(mod, "transaction", L.NewFunction(b.luaTransaction))
	}

	return mod
}

// executor is either a *sql.DB or *sql.Tx — the standard interface
// query/queryOne/exec need to run against either.
type executor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// wrapExecutor curries the executor (either the raw DB or an active
// tx) into each of query/queryOne/exec so the callback signatures
// stay simple LGFunctions.
func (b *Bindings) wrapExecutor(tx *sql.Tx, fn func(L *glua.LState, x executor) int) glua.LGFunction {
	return func(L *glua.LState) int {
		var x executor = b.DB
		if tx != nil {
			x = tx
		}

		if b.DB == nil && tx == nil {
			return pushError(L, "db: no connection (register hex/db/provider first)")
		}

		if b.Mutex != nil {
			b.Mutex.Lock()
			defer b.Mutex.Unlock()
		}

		return fn(L, x)
	}
}

// luaQuery: db.query(sql, ...args) -> array of row-tables, err
func (b *Bindings) luaQuery(L *glua.LState, x executor) int {
	query := L.CheckString(1)
	args := collectArgs(L, 2)

	ctx := b.ctx()

	rows, err := x.QueryContext(ctx, query, args...)
	if err != nil {
		return pushError(L, fmt.Sprintf("db.query: %v", err))
	}
	defer rows.Close()

	tbl, err := rowsToTable(L, rows)
	if err != nil {
		return pushError(L, fmt.Sprintf("db.query: %v", err))
	}

	L.Push(tbl)
	L.Push(glua.LNil)

	return 2
}

// luaQueryOne: db.queryOne(sql, ...args) -> row-table or nil, err
func (b *Bindings) luaQueryOne(L *glua.LState, x executor) int {
	query := L.CheckString(1)
	args := collectArgs(L, 2)

	ctx := b.ctx()

	rows, err := x.QueryContext(ctx, query, args...)
	if err != nil {
		return pushError(L, fmt.Sprintf("db.queryOne: %v", err))
	}
	defer rows.Close()

	if !rows.Next() {
		L.Push(glua.LNil)
		L.Push(glua.LNil)

		return 2
	}

	cols, err := rows.Columns()
	if err != nil {
		return pushError(L, fmt.Sprintf("db.queryOne: columns: %v", err))
	}

	row, err := scanRow(L, rows, cols)
	if err != nil {
		return pushError(L, fmt.Sprintf("db.queryOne: %v", err))
	}

	L.Push(row)
	L.Push(glua.LNil)

	return 2
}

// luaExec: db.exec(sql, ...args) -> {rows_affected, last_insert_id}, err
func (b *Bindings) luaExec(L *glua.LState, x executor) int {
	query := L.CheckString(1)
	args := collectArgs(L, 2)

	ctx := b.ctx()

	result, err := x.ExecContext(ctx, query, args...)
	if err != nil {
		return pushError(L, fmt.Sprintf("db.exec: %v", err))
	}

	tbl := L.NewTable()

	if n, err := result.RowsAffected(); err == nil {
		L.SetField(tbl, "rows_affected", glua.LNumber(n))
	}

	if id, err := result.LastInsertId(); err == nil {
		L.SetField(tbl, "last_insert_id", glua.LNumber(id))
	}

	L.Push(tbl)
	L.Push(glua.LNil)

	return 2
}

// luaTransaction: db.transaction(function(tx) ... end) -> ok, err
//
// The callback receives a `tx` table with query/queryOne/exec bound
// to the active *sql.Tx. If the callback errors or throws a Lua
// error the tx rolls back; otherwise it commits.
func (b *Bindings) luaTransaction(L *glua.LState) int {
	if b.DB == nil {
		return pushError(L, "db.transaction: no connection")
	}

	fn := L.CheckFunction(1)

	ctx := b.ctx()

	// Mutex is intentionally NOT held across the callback — inner
	// tx.query / tx.exec calls acquire it themselves. Holding it
	// through the callback would deadlock those.
	tx, err := b.DB.BeginTx(ctx, nil)
	if err != nil {
		return pushError(L, fmt.Sprintf("db.transaction: begin: %v", err))
	}

	txModule := b.buildModule(L, tx)

	L.Push(fn)
	L.Push(txModule)

	if err := L.PCall(1, glua.MultRet, nil); err != nil {
		_ = tx.Rollback()

		return pushError(L, fmt.Sprintf("db.transaction: callback: %v", err))
	}

	if err := tx.Commit(); err != nil {
		return pushError(L, fmt.Sprintf("db.transaction: commit: %v", err))
	}

	L.Push(glua.LBool(true))
	L.Push(glua.LNil)

	return 2
}

// ctx returns the context to use for a DB call, defaulting to
// context.Background when Bindings.Context is nil.
func (b *Bindings) ctx() context.Context {
	if b.Context != nil {
		return b.Context
	}

	return context.Background()
}

// collectArgs pulls positional args from L starting at start,
// converting each to its natural Go type for use as a sql placeholder
// argument.
func collectArgs(L *glua.LState, start int) []any {
	top := L.GetTop()
	if top < start {
		return nil
	}

	args := make([]any, 0, top-start+1)
	for i := start; i <= top; i++ {
		args = append(args, luaToGo(L.Get(i)))
	}

	return args
}

// luaToGo converts a Lua value to a Go value suitable for database
// placeholder binding. Nil-ish values (LNil, absent) become nil so
// SQL NULL is honoured.
func luaToGo(v glua.LValue) any {
	switch t := v.(type) {
	case glua.LBool:
		return bool(t)
	case glua.LNumber:
		// Preserve int-ness when the number has no fractional part.
		// Most drivers accept either, but ints are what users
		// generally mean when they pass 1, 2, 3.
		n := float64(t)
		if n == float64(int64(n)) {
			return int64(n)
		}

		return n
	case glua.LString:
		return string(t)
	case *glua.LNilType:
		return nil
	case glua.LChannel, *glua.LFunction, *glua.LUserData, *glua.LTable:
		// Best-effort string representation for exotic values. Not
		// used by well-typed SQL bindings.
		return v.String()
	default:
		if v == glua.LNil {
			return nil
		}

		return v.String()
	}
}

// rowsToTable materialises an *sql.Rows into a Lua array-of-tables,
// keyed by column name. Each cell is converted via valueToLua.
func rowsToTable(L *glua.LState, rows *sql.Rows) (*glua.LTable, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	tbl := L.NewTable()
	idx := 0

	for rows.Next() {
		row, err := scanRow(L, rows, cols)
		if err != nil {
			return nil, err
		}

		idx++
		tbl.RawSetInt(idx, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tbl, nil
}

// scanRow scans a single row into a fresh Lua table keyed by column
// name. Cell values are converted via valueToLua.
func scanRow(L *glua.LState, rows *sql.Rows, cols []string) (*glua.LTable, error) {
	holders := make([]any, len(cols))
	for i := range holders {
		var v any

		holders[i] = &v
	}

	if err := rows.Scan(holders...); err != nil {
		return nil, err
	}

	row := L.NewTable()
	for i, col := range cols {
		L.SetField(row, col, valueToLua(*(holders[i].(*any))))
	}

	return row, nil
}

// valueToLua converts a Go value (typically an interface{} scanned
// out of *sql.Rows) to a Lua value. Handles the common driver types;
// unknown types fall back to fmt.Sprintf %v.
func valueToLua(v any) glua.LValue {
	if v == nil {
		return glua.LNil
	}

	switch t := v.(type) {
	case bool:
		return glua.LBool(t)
	case int64:
		return glua.LNumber(t)
	case int32:
		return glua.LNumber(t)
	case int:
		return glua.LNumber(t)
	case float64:
		return glua.LNumber(t)
	case float32:
		return glua.LNumber(t)
	case string:
		return glua.LString(t)
	case []byte:
		return glua.LString(t)
	case time.Time:
		return glua.LString(t.Format(time.RFC3339Nano))
	default:
		return glua.LString(fmt.Sprintf("%v", t))
	}
}

// pushError pushes (nil, "message") — the standard error return
// contract for module functions.
func pushError(L *glua.LState, msg string) int {
	L.Push(glua.LNil)
	L.Push(glua.LString(msg))

	return 2
}

// silence unused import in some build tags
var _ = errors.New
