package db_test

import (
	"context"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/jordanbrauer/hex/db"
)

func TestOpen_requiresDriver(t *testing.T) {
	_, err := db.Open(context.Background(), db.Config{DSN: ":memory:"})
	if err == nil {
		t.Errorf("Open with no Driver returned nil error")
	}
}

func TestOpen_requiresDSN(t *testing.T) {
	_, err := db.Open(context.Background(), db.Config{Driver: "sqlite"})
	if err == nil {
		t.Errorf("Open with no DSN returned nil error")
	}
}

func TestOpen_unknownDriverFails(t *testing.T) {
	_, err := db.Open(context.Background(), db.Config{Driver: "nope", DSN: "x"})
	if err == nil {
		t.Errorf("Open with unknown driver returned nil error")
	}
}

func TestOpen_sqliteInMemory(t *testing.T) {
	conn, err := db.Open(context.Background(), db.Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	})
	if err != nil {
		t.Fatalf("Open error = %v", err)
	}

	defer conn.Close()

	if err := conn.Ping(); err != nil {
		t.Errorf("Ping error = %v", err)
	}
}

func TestOpen_appliesPragmas(t *testing.T) {
	conn, err := db.Open(context.Background(), db.Config{
		Driver:  "sqlite",
		DSN:     ":memory:",
		Pragmas: []string{"PRAGMA foreign_keys = ON"},
	})
	if err != nil {
		t.Fatalf("Open error = %v", err)
	}

	defer conn.Close()

	var enabled int
	if err := conn.QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatalf("query pragma: %v", err)
	}

	if enabled != 1 {
		t.Errorf("foreign_keys = %d, want 1", enabled)
	}
}

func TestOpen_badPragmaReturnsError(t *testing.T) {
	_, err := db.Open(context.Background(), db.Config{
		Driver:  "sqlite",
		DSN:     ":memory:",
		Pragmas: []string{"THIS IS NOT VALID SQL"},
	})
	if err == nil {
		t.Errorf("Open with bad pragma returned nil error")
	}
}

func TestOpen_poolConfigApplied(t *testing.T) {
	conn, err := db.Open(context.Background(), db.Config{
		Driver:          "sqlite",
		DSN:             ":memory:",
		MaxOpenConns:    3,
		MaxIdleConns:    2,
		ConnMaxLifetime: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Open error = %v", err)
	}

	defer conn.Close()

	stats := conn.Stats()
	if stats.MaxOpenConnections != 3 {
		t.Errorf("MaxOpenConnections = %d, want 3", stats.MaxOpenConnections)
	}
}

func TestTune_appliesToExistingConn(t *testing.T) {
	conn, err := db.Open(context.Background(), db.Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	})
	if err != nil {
		t.Fatal(err)
	}

	defer conn.Close()

	if err := db.Tune(context.Background(), conn, db.Config{
		MaxOpenConns: 5,
	}); err != nil {
		t.Errorf("Tune error = %v", err)
	}

	if conn.Stats().MaxOpenConnections != 5 {
		t.Errorf("MaxOpenConnections after Tune = %d, want 5", conn.Stats().MaxOpenConnections)
	}
}
