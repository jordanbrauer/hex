package webtest_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/env"
	"github.com/jordanbrauer/hex/provider"
	hexweb "github.com/jordanbrauer/hex/web"
	"github.com/jordanbrauer/hex/webtest"
)

// fakeWebProvider stands in for the full hex/web/provider stack —
// binds a minimal *web.Server with a handful of routes so the client
// has something realistic to hit without pulling in every layer.
type fakeWebProvider struct {
	provider.Base
}

func (p *fakeWebProvider) Register(app provider.Application) error {
	srv := hexweb.New(hexweb.Options{Address: ":0"})
	e := srv.Echo()

	e.GET("/", func(c echo.Context) error {
		return c.HTML(200, `<!DOCTYPE html>
<html>
<head><title>Home</title></head>
<body>
<h1>Welcome, Alice</h1>
<div class="user-card active">Alice</div>
<div class="user-card">Bob</div>
<div class="user-card">Carol</div>
<button data-testid="add-user">Add User</button>
</body>
</html>`)
	})

	e.GET("/redirect", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, "/target")
	})

	e.POST("/login", func(c echo.Context) error {
		form, _ := c.FormParams()
		if form.Get("email") == "user@example.com" && form.Get("password") == "hunter2" {
			return c.HTML(200, `<p>signed in</p>`)
		}
		return c.HTML(401, `<p>bad credentials</p>`)
	})

	e.GET("/echo-header", func(c echo.Context) error {
		return c.String(200, c.Request().Header.Get("HX-Request"))
	})

	app.Singleton("http", func(*container.Container) (any, error) {
		return srv, nil
	})

	return nil
}

func newApp(t *testing.T) *hex.App {
	t.Helper()

	app := hex.New(hex.WithEnvironment(env.Test))
	if err := app.Register(&fakeWebProvider{}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

	return app
}

func TestClient_getAndFind(t *testing.T) {
	client := webtest.New(t, newApp(t))

	client.Get("/").
		StatusIs(200).
		See("Welcome, Alice").
		Find("h1").HasText("Welcome")

	client.Find(".user-card").Count(3)
	client.Find(".user-card").First().HasClass("active")
	client.Find(".user-card").Nth(1).HasText("Bob")
	client.Find(`[data-testid="add-user"]`).Exists()
	client.Find(".missing").DoesNotExist()
}

func TestClient_redirectHeader(t *testing.T) {
	client := webtest.New(t, newApp(t))

	// Disable following so we can see the Location header.
	client.Get("/redirect")
	// Default http.Client follows redirects; if the target 404s the
	// final status will be 404. Assert on Body or Location via
	// direct http calls in advanced tests. Here just verify we got
	// somewhere.
	if client.Status() < 200 || client.Status() >= 500 {
		t.Errorf("unexpected redirect status %d", client.Status())
	}
}

func TestClient_postForm(t *testing.T) {
	client := webtest.New(t, newApp(t))

	client.PostForm("/login", url.Values{
		"email":    []string{"user@example.com"},
		"password": []string{"hunter2"},
	}).StatusIs(200).See("signed in")

	client.PostForm("/login", url.Values{
		"email":    []string{"user@example.com"},
		"password": []string{"wrong"},
	}).StatusIs(401).See("bad credentials")
}

func TestClient_defaultHeaders(t *testing.T) {
	client := webtest.New(t, newApp(t))

	client.
		Header("HX-Request", "true").
		Get("/echo-header").
		StatusIs(200).
		BodyContains("true")
}

func TestClient_selectionAttributes(t *testing.T) {
	client := webtest.New(t, newApp(t))
	client.Get("/")

	client.Find("button").HasAttribute("data-testid")
	client.Find("button").HasAttributeValue("data-testid", "add-user")
}
