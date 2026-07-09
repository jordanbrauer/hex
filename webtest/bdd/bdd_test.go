package bdd_test

import (
	"context"
	"embed"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/bdd"
	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/env"
	"github.com/jordanbrauer/hex/provider"
	hexweb "github.com/jordanbrauer/hex/web"
	"github.com/jordanbrauer/hex/webtest"
	webtestbdd "github.com/jordanbrauer/hex/webtest/bdd"
)

//go:embed testdata/features/*.feature
var features embed.FS

type demoWebProvider struct{ provider.Base }

func (p *demoWebProvider) Register(app provider.Application) error {
	srv := hexweb.New(hexweb.Options{Address: ":0"})

	srv.Echo().GET("/", func(c echo.Context) error {
		return c.HTML(http.StatusOK, `<!DOCTYPE html>
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

	app.Singleton("http", func(*container.Container) (any, error) {
		return srv, nil
	})

	return nil
}

func TestBDD_endToEnd(t *testing.T) {
	app := hex.New(hex.WithEnvironment(env.Test))

	if err := app.Register(&demoWebProvider{}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

	suite := bdd.NewSuiteFS(t, features, "testdata/features/*.feature")

	webtestbdd.Register(suite, func() *webtest.Client {
		return webtest.New(t, app)
	})

	suite.Run()
}
