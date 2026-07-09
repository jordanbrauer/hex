package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/webtest"

	"github.com/jordanbrauer/hex/examples/swapi/app"
)

// TestApp boots the whole swapi app in-process and drives every route
// through hex/webtest. Fast, hermetic, no external state — the DB is
// the swapi.db file checked into examples/swapi/data.
func TestApp(t *testing.T) {
	// The web + db providers resolve paths relative to cwd. When
	// `go test` runs it sets cwd to the package dir, but be explicit
	// so the assertion is stable across `go test ./examples/...` too.
	_, thisFile, _, _ := runtime.Caller(0)
	if err := os.Chdir(filepath.Dir(thisFile)); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	kernel := hex.New()
	if err := app.Boot(kernel); err != nil {
		t.Fatalf("boot: %v", err)
	}

	ctx := context.Background()

	if err := kernel.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	t.Cleanup(func() { _ = kernel.Shutdown(ctx) })

	client := webtest.New(t, kernel)

	t.Run("home", func(t *testing.T) {
		client.Get("/").
			StatusIs(200).
			See("SWAPI").
			See("A long time ago")

		// Counts match the fixture DB.
		client.Find(".counts .n").Count(6)
	})

	t.Run("films index", func(t *testing.T) {
		client.Get("/films").
			StatusIs(200).
			See("A New Hope").
			See("The Empire Strikes Back").
			See("Return of the Jedi").
			Find("tr.film").Count(6)
	})

	t.Run("films show", func(t *testing.T) {
		// Episode 4 is A New Hope (id=1 in the fixture).
		client.Get("/films/1").
			StatusIs(200).
			See("A New Hope").
			See("Opening crawl").
			See("George Lucas")

		client.Find(".crawl p").Exists()
	})

	t.Run("people index", func(t *testing.T) {
		client.Get("/people").
			StatusIs(200).
			See("Luke Skywalker").
			See("Darth Vader").
			Find("tr.person").Count(82)
	})

	t.Run("people show", func(t *testing.T) {
		client.Get("/people/1").
			StatusIs(200).
			See("Homeworld")

		client.Find("ul.films li").Exists()
	})

	t.Run("planets", func(t *testing.T) {
		client.Get("/planets").
			StatusIs(200).
			See("Tatooine").
			Find("tr.planet").Count(60)
	})

	t.Run("species", func(t *testing.T) {
		client.Get("/species").
			StatusIs(200).
			See("Wookie").
			Find("tr.species").Count(37)
	})

	t.Run("starships", func(t *testing.T) {
		client.Get("/starships").
			StatusIs(200).
			See("X-wing").
			Find("tr.starship").Count(36)
	})

	t.Run("vehicles", func(t *testing.T) {
		client.Get("/vehicles").
			StatusIs(200).
			Find("tr.vehicle").Count(39)
	})

	t.Run("missing film 404s", func(t *testing.T) {
		client.Get("/films/9999").StatusIs(404)
	})
}
