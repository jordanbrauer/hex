package lua_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/jordanbrauer/hex/db"
	dblua "github.com/jordanbrauer/hex/db/lua"
	hexlua "github.com/jordanbrauer/hex/lua"
)

// newEnv opens an in-memory SQLite DB seeded with a small users
// table, wires up a fresh *hex/lua.Environment with the db module
// preloaded, and returns both. Cleanup registered via t.Cleanup.
func newEnv(t *testing.T) (*hexlua.Environment, *sql.DB) {
	t.Helper()

	conn, err := db.Open(context.Background(), db.Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	seed(t, conn)

	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	bindings := &dblua.Bindings{DB: conn}
	env.PreloadModule("db", bindings.Loader)

	return env, conn
}

func seed(t *testing.T, conn *sql.DB) {
	t.Helper()

	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, active INTEGER NOT NULL DEFAULT 1)`,
		`INSERT INTO users (name, active) VALUES ('alice', 1), ('bob', 1), ('carol', 0)`,
	}

	for _, stmt := range stmts {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}
}

func TestQuery_returnsAllMatchingRows(t *testing.T) {
	env, _ := newEnv(t)

	err := env.ExecString(`
		local db = require("db")
		local rows, err = db.query("SELECT id, name FROM users WHERE active = ? ORDER BY id", 1)
		if err ~= nil then error(err) end
		if #rows ~= 2 then error("expected 2 rows, got " .. #rows) end
		if rows[1].name ~= "alice" then error("row 1: " .. rows[1].name) end
		if rows[2].name ~= "bob" then error("row 2: " .. rows[2].name) end
	`, "query_test.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestQueryOne_returnsSingleRow(t *testing.T) {
	env, _ := newEnv(t)

	err := env.ExecString(`
		local db = require("db")
		local u, err = db.queryOne("SELECT * FROM users WHERE name = ?", "alice")
		if err ~= nil then error(err) end
		if u == nil then error("expected user, got nil") end
		if u.name ~= "alice" then error("wrong user: " .. tostring(u.name)) end
		if u.id ~= 1 then error("wrong id: " .. tostring(u.id)) end
	`, "queryOne_test.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestQueryOne_returnsNilWhenNoMatch(t *testing.T) {
	env, _ := newEnv(t)

	err := env.ExecString(`
		local db = require("db")
		local u, err = db.queryOne("SELECT * FROM users WHERE id = ?", 999)
		if err ~= nil then error(err) end
		if u ~= nil then error("expected nil, got " .. tostring(u.name)) end
	`, "queryOne_nil_test.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestExec_returnsAffectedRowsAndInsertID(t *testing.T) {
	env, conn := newEnv(t)

	err := env.ExecString(`
		local db = require("db")
		local r, err = db.exec("INSERT INTO users (name, active) VALUES (?, ?)", "dave", 1)
		if err ~= nil then error(err) end
		if r.rows_affected ~= 1 then error("expected 1 row affected, got " .. tostring(r.rows_affected)) end
		if r.last_insert_id == nil then error("no last_insert_id") end
	`, "exec_test.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	// Confirm from the Go side.
	var count int
	if err := conn.QueryRow("SELECT COUNT(*) FROM users WHERE name = 'dave'").Scan(&count); err != nil {
		t.Fatalf("verify: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 dave row, got %d", count)
	}
}

func TestTransaction_commitsOnSuccess(t *testing.T) {
	env, conn := newEnv(t)

	err := env.ExecString(`
		local db = require("db")
		local ok, err = db.transaction(function(tx)
			tx.exec("INSERT INTO users (name, active) VALUES (?, 1)", "x")
			tx.exec("INSERT INTO users (name, active) VALUES (?, 1)", "y")
		end)
		if err ~= nil then error(err) end
		if ok ~= true then error("expected ok=true") end
	`, "tx_commit_test.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	var count int
	if err := conn.QueryRow("SELECT COUNT(*) FROM users WHERE name IN ('x','y')").Scan(&count); err != nil {
		t.Fatalf("verify: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 committed rows, got %d", count)
	}
}

func TestTransaction_rollsBackOnCallbackError(t *testing.T) {
	env, conn := newEnv(t)

	err := env.ExecString(`
		local db = require("db")
		local ok, err = db.transaction(function(tx)
			tx.exec("INSERT INTO users (name, active) VALUES (?, 1)", "should_not_persist")
			error("intentional failure")
		end)
		if ok ~= nil then error("expected ok=nil after rollback, got " .. tostring(ok)) end
		if err == nil then error("expected err to be set") end
	`, "tx_rollback_test.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	var count int
	if err := conn.QueryRow("SELECT COUNT(*) FROM users WHERE name = 'should_not_persist'").Scan(&count); err != nil {
		t.Fatalf("verify: %v", err)
	}

	if count != 0 {
		t.Errorf("rollback failed; row still present, count=%d", count)
	}
}

func TestQuery_returnsSqlErrorAsString(t *testing.T) {
	env, _ := newEnv(t)

	err := env.ExecString(`
		local db = require("db")
		local rows, err = db.query("SELECT * FROM does_not_exist")
		if rows ~= nil then error("expected nil rows on sql error") end
		if err == nil then error("expected err message") end
	`, "sql_error_test.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestQuery_handlesNullAsLuaNil(t *testing.T) {
	env, conn := newEnv(t)

	// Add a column that can be NULL and a row with it unset.
	if _, err := conn.Exec(`ALTER TABLE users ADD COLUMN email TEXT`); err != nil {
		t.Fatalf("alter: %v", err)
	}
	if _, err := conn.Exec(`UPDATE users SET email = 'alice@example.com' WHERE name = 'alice'`); err != nil {
		t.Fatalf("update: %v", err)
	}

	err := env.ExecString(`
		local db = require("db")
		local alice, err = db.queryOne("SELECT * FROM users WHERE name = 'alice'")
		if err ~= nil then error(err) end
		if alice.email ~= "alice@example.com" then error("alice email: " .. tostring(alice.email)) end

		local bob, err2 = db.queryOne("SELECT * FROM users WHERE name = 'bob'")
		if err2 ~= nil then error(err2) end
		if bob.email ~= nil then error("bob email should be nil, got: " .. tostring(bob.email)) end
	`, "null_test.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestPushError_message(t *testing.T) {
	// Sanity check for the (nil, "msg") contract via a nonexistent
	// column, which surfaces a scannable error message.
	env, _ := newEnv(t)

	err := env.ExecString(`
		local db = require("db")
		local _, err = db.query("SELECT wat FROM users")
		if err == nil then error("expected err") end
		-- The message should be non-empty and mention db.query
		if #err == 0 then error("empty err") end
	`, "push_err_test.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	_ = strings.Contains // silence unused import if we later refactor
}
