package provider_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	glua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/env"
	hexlua "github.com/jordanbrauer/hex/lua"
	"github.com/jordanbrauer/hex/lua/provider"
	hexprovider "github.com/jordanbrauer/hex/provider"
)

// domainProvider stands in for a consumer provider that registers a
// domain-specific Lua module on the shared env.
type domainProvider struct {
	hexprovider.Base
}

func (p *domainProvider) Register(app hexprovider.Application) error {
	env, err := container.Make[*hexlua.Environment](app, "lua")
	if err != nil {
		return err
	}

	env.PreloadModule("users", func(L *glua.LState) int {
		mod := L.SetFuncs(L.NewTable(), map[string]glua.LGFunction{
			"count": func(L *glua.LState) int {
				L.Push(glua.LNumber(42))

				return 1
			},
		})
		L.Push(mod)

		return 1
	})

	return nil
}

func newTestApp(t *testing.T, environment env.Environment, providers ...hexprovider.Service) *hex.App {
	t.Helper()

	app := hex.New(hex.WithEnvironment(environment))

	if err := app.Register(providers...); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

	return app
}

func TestReplCommand_seesDomainModulesFromContainer(t *testing.T) {
	// Uses --mode lua because Teal typechecks require() at compile
	// time and needs a .d.tl stub for Go-registered modules to
	// resolve. Stub generation is pi-fox.2 (Phase 3.5); until it
	// lands, users touching Go modules from the REPL should either
	// pass --mode lua or set repl.mode = "lua" in config.
	app := newTestApp(t, env.Test, &provider.Provider{}, &domainProvider{})

	cmd := provider.ReplCommand(app)

	var out, errOut bytes.Buffer
	cmd.SetArgs([]string{"--mode", "lua"})
	cmd.SetIn(strings.NewReader(`require("users").count()` + "\nexit\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(out.String(), "42") {
		t.Errorf("expected 42 in output (from users.count() bound by domain provider), got:\n%s", out.String())
	}

	if errOut.Len() > 0 {
		t.Errorf("stderr should be empty, got:\n%s", errOut.String())
	}
}

func TestReplCommand_tealHintsForMissingModuleStub(t *testing.T) {
	// Teal mode + Go-registered module without a stub. Users need to
	// know they can either drop to Lua mode or wait for pi-fox.2.
	app := newTestApp(t, env.Test, &provider.Provider{}, &domainProvider{})

	cmd := provider.ReplCommand(app)

	var out, errOut bytes.Buffer
	cmd.SetIn(strings.NewReader(`require("users")` + "\nexit\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(errOut.String(), "no type information") {
		t.Errorf("expected Teal type-info error, got:\n%s", errOut.String())
	}

	if !strings.Contains(errOut.String(), "--mode lua") {
		t.Errorf("expected hint about --mode lua, got:\n%s", errOut.String())
	}
}

func TestReplCommand_defaultsToTealMode(t *testing.T) {
	app := newTestApp(t, env.Test, &provider.Provider{})

	cmd := provider.ReplCommand(app)

	var out, errOut bytes.Buffer
	cmd.SetIn(strings.NewReader("exit\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(out.String(), "teal mode") {
		t.Errorf("expected teal mode in banner, got:\n%s", out.String())
	}
}

func TestReplCommand_modeFlagOverrides(t *testing.T) {
	app := newTestApp(t, env.Test, &provider.Provider{})

	cmd := provider.ReplCommand(app)

	var out, errOut bytes.Buffer
	cmd.SetArgs([]string{"--mode", "lua"})
	cmd.SetIn(strings.NewReader("exit\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(out.String(), "lua mode") {
		t.Errorf("--mode lua did not switch mode, got:\n%s", out.String())
	}
}

func TestReplCommand_prodBanner(t *testing.T) {
	app := newTestApp(t, env.Production, &provider.Provider{})

	cmd := provider.ReplCommand(app)

	var out, errOut bytes.Buffer
	cmd.SetIn(strings.NewReader("exit\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(out.String(), "PRODUCTION") {
		t.Errorf("expected PRODUCTION banner, got:\n%s", out.String())
	}
}

func TestReplCommand_noProdBannerInTest(t *testing.T) {
	app := newTestApp(t, env.Test, &provider.Provider{})

	cmd := provider.ReplCommand(app)

	var out, errOut bytes.Buffer
	cmd.SetIn(strings.NewReader("exit\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if strings.Contains(out.String(), "PRODUCTION") {
		t.Errorf("test env should not show PRODUCTION banner, got:\n%s", out.String())
	}
}

func TestReplCommand_failsWithoutLuaProvider(t *testing.T) {
	// If the lua provider isn't registered, "lua" isn't in the
	// container; the command should fail fast rather than silently.
	app := hex.New(hex.WithEnvironment(env.Test))

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

	cmd := provider.ReplCommand(app)

	var out, errOut bytes.Buffer
	cmd.SetIn(strings.NewReader("exit\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SilenceUsage = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error when lua provider not registered")
	}
}
