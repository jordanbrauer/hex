package sqlite_test

import (
	"context"
	"database/sql"
	"embed"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/jordanbrauer/hex/db"
	hexsqlite "github.com/jordanbrauer/hex/db/sqlite"
)

//go:embed testdata/migrations/*.sql
var migrations embed.FS

func openMem(t *testing.T) *sql.DB {
	t.Helper()

	conn, err := db.Open(context.Background(), db.Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	t.Cleanup(func() { conn.Close() })

	return conn
}

func tableExists(t *testing.T, conn *sql.DB, name string) bool {
	t.Helper()

	var got string
	err := conn.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name=?", name,
	).Scan(&got)

	if err == sql.ErrNoRows {
		return false
	}

	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}

	return got == name
}

func columnExists(t *testing.T, conn *sql.DB, table, col string) bool {
	t.Helper()

	rows, err := conn.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("pragma table_info: %v", err)
	}

	defer rows.Close()

	for rows.Next() {
		var (
			cid, notnull, pk int
			name             string
			ctype            string
			dflt             sql.NullString
		)

		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan pragma: %v", err)
		}

		if name == col {
			return true
		}
	}

	return false
}

func TestMigrate_appliesAllUp(t *testing.T) {
	conn := openMem(t)

	if err := hexsqlite.Migrate(conn, migrations, "testdata/migrations"); err != nil {
		t.Fatalf("Migrate error = %v", err)
	}

	if !tableExists(t, conn, "widgets") {
		t.Errorf("widgets table not created")
	}

	if !columnExists(t, conn, "widgets", "color") {
		t.Errorf("widgets.color column not added")
	}
}

func TestMigrate_isIdempotent(t *testing.T) {
	conn := openMem(t)

	if err := hexsqlite.Migrate(conn, migrations, "testdata/migrations"); err != nil {
		t.Fatalf("first Migrate error = %v", err)
	}

	if err := hexsqlite.Migrate(conn, migrations, "testdata/migrations"); err != nil {
		t.Errorf("second Migrate returned error: %v", err)
	}
}

func TestMigrateDown_reversesAll(t *testing.T) {
	conn := openMem(t)

	if err := hexsqlite.Migrate(conn, migrations, "testdata/migrations"); err != nil {
		t.Fatalf("Up: %v", err)
	}

	if err := hexsqlite.MigrateDown(conn, migrations, "testdata/migrations"); err != nil {
		t.Fatalf("Down: %v", err)
	}

	if tableExists(t, conn, "widgets") {
		t.Errorf("widgets table still present after MigrateDown")
	}
}

func TestMigrateSteps_partialUp(t *testing.T) {
	conn := openMem(t)

	if err := hexsqlite.MigrateSteps(conn, migrations, "testdata/migrations", 1); err != nil {
		t.Fatalf("Steps(1) error = %v", err)
	}

	if !tableExists(t, conn, "widgets") {
		t.Errorf("widgets table missing after Steps(1)")
	}

	// Second migration must NOT have applied yet.
	if columnExists(t, conn, "widgets", "color") {
		t.Errorf("widgets.color present after only Steps(1)")
	}

	// Advance one more.
	if err := hexsqlite.MigrateSteps(conn, migrations, "testdata/migrations", 1); err != nil {
		t.Fatalf("Steps(1) again error = %v", err)
	}

	if !columnExists(t, conn, "widgets", "color") {
		t.Errorf("widgets.color missing after two Steps")
	}
}

func TestMigrate_missingDirFails(t *testing.T) {
	conn := openMem(t)

	if err := hexsqlite.Migrate(conn, migrations, "does/not/exist"); err == nil {
		t.Errorf("Migrate with missing dir returned nil error")
	}
}
