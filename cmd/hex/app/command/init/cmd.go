// Package init implements `hex init`. It is imported aliased (e.g.
// initcmd) at call sites to avoid confusion with Go's package-level
// init() function, which this package does not define.
package init

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	aiprovider "github.com/jordanbrauer/hex/ai/provider"
	cacheprovider "github.com/jordanbrauer/hex/cache/provider"
	logprovider "github.com/jordanbrauer/hex/log/provider"
	luaprovider "github.com/jordanbrauer/hex/lua/provider"
	webprovider "github.com/jordanbrauer/hex/web/provider"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/build"
	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
)

//go:embed long.md
var longFile string

//go:embed example.sh
var exampleFile string

var (
	long    = strings.TrimRight(longFile, "\n")
	example = strings.TrimRight(exampleFile, "\n")
)

// initConfig is populated from flags + prompts and threaded into the
// init templates.
type initConfig struct {
	// Name is the project name (also the binary name unless BinaryName
	// overrides it).
	Name string

	// Directory is the absolute path the project scaffolds into.
	Directory string

	// ModulePath is the go.mod module path (e.g. github.com/you/name).
	ModulePath string

	// BinaryName is the compiled binary name; defaults to Name.
	BinaryName string

	// Dialect is "sqlite", "postgres", or "none" for the database.
	Dialect string

	// Optional framework components.
	Cache       bool // memory backend, default off
	Cron        bool
	Web         bool
	Queue       string // "memory" | "sqlite" | "none"
	Telemetry   string // "stdout" | "otlp" | "none"
	Policy      bool
	I18n        bool
	Featureflag bool
	AI          string // "openai" | "anthropic" | "none"
	Frontend    string // "js" | "ts" | "none"

	// Developer tooling. On by default; pass the matching --no-x style
	// flag (e.g. --editorconfig=false) to opt out.
	Editorconfig  bool
	Lint          bool // .golangci.yml
	Goreleaser    bool // .goreleaser.yaml
	GithubActions bool // .github/workflows/release.yml; requires Goreleaser

	// GoVersion is the Go directive (e.g. "1.26"). Defaults to the
	// running toolchain.
	GoVersion string

	// HexVersion is the hex library version to require. Empty means the
	// tool copies its own build info.
	HexVersion string
}

// HasDatabase reports whether the database provider should be scaffolded.
func (c initConfig) HasDatabase() bool { return c.Dialect != "" && c.Dialect != "none" }

// HasQueue reports whether the queue provider should be scaffolded.
func (c initConfig) HasQueue() bool { return c.Queue != "" && c.Queue != "none" }

// HasTelemetry reports whether the telemetry provider should be scaffolded.
func (c initConfig) HasTelemetry() bool { return c.Telemetry != "" && c.Telemetry != "none" }

// HasAI reports whether the ai provider should be scaffolded.
func (c initConfig) HasAI() bool { return c.AI != "" && c.AI != "none" }

// HasFrontend reports whether frontend assets should be scaffolded.
// Frontend is only meaningful when Web is enabled.
func (c initConfig) HasFrontend() bool {
	return c.Web && c.Frontend != "" && c.Frontend != "none"
}

// FrontendTS reports whether the TypeScript-with-build variant is
// scaffolded (vs the no-build JS variant).
func (c initConfig) FrontendTS() bool { return c.Frontend == "ts" }

// HasGithubActions reports whether the release workflow should be
// scaffolded. GithubActions implies Goreleaser (see applyExtras).
func (c initConfig) HasGithubActions() bool { return c.GithubActions && c.Goreleaser }

// GitHubOwner returns the GitHub org/user segment of ModulePath when it
// points at github.com, or a placeholder otherwise.
func (c initConfig) GitHubOwner() string {
	owner, _, ok := githubSlug(c.ModulePath)
	if !ok {
		return "your-org"
	}

	return owner
}

// GitHubRepo returns the GitHub repository segment of ModulePath when it
// points at github.com, or the project name otherwise.
func (c initConfig) GitHubRepo() string {
	_, repo, ok := githubSlug(c.ModulePath)
	if !ok {
		return c.Name
	}

	return repo
}

// githubSlug extracts owner/repo from a github.com/owner/repo[/...]
// module path.
func githubSlug(modulePath string) (owner, repo string, ok bool) {
	const prefix = "github.com/"
	if !strings.HasPrefix(modulePath, prefix) {
		return "", "", false
	}

	rest := strings.TrimPrefix(modulePath, prefix)
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}

// New builds the `hex init [name]` command.
func New(app *hex.App) *cobra.Command {
	var (
		modulePath    string
		dialect       string
		cache         bool
		cron          bool
		web           bool
		queue         string
		telemetry     string
		policy        bool
		i18nFlag      bool
		featureflag   bool
		aiFlag        string
		frontend      string
		editorconfig  bool
		lint          bool
		goreleaser    bool
		githubActions bool
		yes           bool
		flags         generator.Flags
	)

	cmd := &cobra.Command{
		Use:     "init [name]",
		Short:   "Scaffold a new hex project",
		Long:    long,
		Example: example,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := resolveInitConfig(args, resolveFlags{
				module:        modulePath,
				dialect:       dialect,
				cache:         cache,
				cron:          cron,
				web:           web,
				queue:         queue,
				telemetry:     telemetry,
				policy:        policy,
				i18n:          i18nFlag,
				featureflag:   featureflag,
				ai:            aiFlag,
				frontend:      frontend,
				editorconfig:  editorconfig,
				lint:          lint,
				goreleaser:    goreleaser,
				githubActions: githubActions,
				yes:           yes,
			})
			if err != nil {
				return err
			}

			opts, err := flags.Options()
			if err != nil {
				return err
			}

			svc, err := generator.Resolve(app)
			if err != nil {
				return err
			}

			actions, err := scaffold(cmd.Context(), svc, cfg, opts)
			if err != nil {
				return err
			}

			if err := generator.Report(cmd.OutOrStdout(), actions, opts.DryRun, flags.Format); err != nil {
				return err
			}

			if flags.Format != "json" && !opts.DryRun {
				printInitSuccess(cmd, cfg)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&modulePath, "module", "", "Go module path (default: github.com/<user>/<name>)")
	cmd.Flags().StringVar(&dialect, "db", "sqlite", "database dialect: sqlite, postgres, none")
	cmd.Flags().BoolVar(&cache, "cache", false, "scaffold a cache provider (memory backend)")
	cmd.Flags().BoolVar(&cron, "cron", false, "scaffold a cron scheduler provider")
	cmd.Flags().BoolVar(&web, "web", false, "scaffold a web (echo) server provider")
	cmd.Flags().StringVar(&queue, "queue", "none", "queue backend: memory or none")
	cmd.Flags().StringVar(&telemetry, "telemetry", "none", "telemetry exporter: stdout, otlp, none")
	cmd.Flags().BoolVar(&policy, "policy", false, "scaffold a policy (Casbin) provider")
	cmd.Flags().BoolVar(&i18nFlag, "i18n", false, "scaffold an i18n (gotext) provider")
	cmd.Flags().BoolVar(&featureflag, "featureflag", false, "scaffold a featureflag (GOFF) provider")
	cmd.Flags().StringVar(&aiFlag, "ai", "none", "AI provider: openai, anthropic, none")
	cmd.Flags().StringVar(&frontend, "frontend", "none", "frontend stack: js (no build), ts (Laravel Mix), none (default)")
	cmd.Flags().BoolVar(&editorconfig, "editorconfig", true, "scaffold an .editorconfig")
	cmd.Flags().BoolVar(&lint, "lint", true, "scaffold a .golangci.yml lint config")
	cmd.Flags().BoolVar(&goreleaser, "goreleaser", true, "scaffold a .goreleaser.yaml release config")
	cmd.Flags().BoolVar(&githubActions, "github", false, "scaffold a GitHub Actions release workflow (implies --goreleaser)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip interactive prompts, use flag defaults")
	generator.AddFlags(cmd, &flags)

	return cmd
}

// resolveFlags carries flag values from New into resolveInitConfig
// without expanding the argument list.
type resolveFlags struct {
	module        string
	dialect       string
	cache         bool
	cron          bool
	web           bool
	queue         string
	telemetry     string
	policy        bool
	i18n          bool
	featureflag   bool
	ai            string
	frontend      string
	editorconfig  bool
	lint          bool
	goreleaser    bool
	githubActions bool
	yes           bool
}

// resolveInitConfig combines CLI args, flags, and interactive prompts
// into a fully-populated initConfig.
func resolveInitConfig(args []string, f resolveFlags) (initConfig, error) {
	cfg := initConfig{
		Dialect:       f.dialect,
		ModulePath:    f.module,
		Cache:         f.cache,
		Cron:          f.cron,
		Web:           f.web,
		Queue:         f.queue,
		Telemetry:     f.telemetry,
		Policy:        f.policy,
		I18n:          f.i18n,
		Featureflag:   f.featureflag,
		AI:            f.ai,
		Frontend:      f.frontend,
		Editorconfig:  f.editorconfig,
		Lint:          f.lint,
		Goreleaser:    f.goreleaser,
		GithubActions: f.githubActions,
	}

	// Resolve the target directory + project name.
	cwd, err := os.Getwd()
	if err != nil {
		return cfg, err
	}

	if len(args) == 0 || args[0] == "." {
		cfg.Directory = cwd
		cfg.Name = filepath.Base(cwd)
	} else {
		name := args[0]
		if strings.ContainsAny(name, `/\`) {
			return cfg, errors.New("project name may not contain path separators")
		}

		cfg.Name = name
		cfg.Directory = filepath.Join(cwd, name)
	}

	if cfg.BinaryName == "" {
		cfg.BinaryName = cfg.Name
	}

	// Default module path.
	if cfg.ModulePath == "" {
		cfg.ModulePath = defaultModulePath(cfg.Name)
	}

	cfg.GoVersion = runningGoVersion()

	if cfg.HexVersion == "" {
		cfg.HexVersion = hexRequireVersion()
	}

	cfg.applyToolingDefaults()

	if f.yes {
		return cfg, cfg.validate()
	}

	if err := runInitPrompts(&cfg); err != nil {
		return cfg, err
	}

	return cfg, cfg.validate()
}

// runInitPrompts asks the user to confirm the module path, binary name,
// and which components to enable via huh.
func runInitPrompts(cfg *initConfig) error {
	// Multi-select for optional components. Start with whatever flags
	// were set as pre-selected.
	extras := []string{}
	if cfg.Cache {
		extras = append(extras, "cache")
	}

	if cfg.Cron {
		extras = append(extras, "cron")
	}

	if cfg.Web {
		extras = append(extras, "web")
	}

	if cfg.HasQueue() {
		extras = append(extras, "queue")
	}

	if cfg.HasTelemetry() {
		extras = append(extras, "telemetry")
	}

	if cfg.Policy {
		extras = append(extras, "policy")
	}

	if cfg.I18n {
		extras = append(extras, "i18n")
	}

	if cfg.Featureflag {
		extras = append(extras, "featureflag")
	}

	if cfg.HasAI() {
		extras = append(extras, "ai")
	}

	if cfg.HasFrontend() {
		if cfg.FrontendTS() {
			extras = append(extras, "frontend-ts")
		} else {
			extras = append(extras, "frontend-js")
		}
	}

	// Developer tooling defaults to on (editorconfig, lint, goreleaser);
	// github actions defaults to off. Pre-select from whatever the
	// flags resolved to.
	tooling := []string{}
	if cfg.Editorconfig {
		tooling = append(tooling, "editorconfig")
	}

	if cfg.Lint {
		tooling = append(tooling, "lint")
	}

	if cfg.Goreleaser {
		tooling = append(tooling, "goreleaser")
	}

	if cfg.GithubActions {
		tooling = append(tooling, "github")
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Go module path").
				Description("used by go.mod and import paths").
				Value(&cfg.ModulePath),
			huh.NewInput().
				Title("Binary name").
				Description("compiled binary; defaults to project name").
				Value(&cfg.BinaryName),
			huh.NewSelect[string]().
				Title("Database dialect").
				Options(
					huh.NewOption("SQLite (embedded, no infra)", "sqlite"),
					huh.NewOption("Postgres", "postgres"),
					huh.NewOption("None (skip db setup)", "none"),
				).
				Value(&cfg.Dialect),
			huh.NewMultiSelect[string]().
				Title("Framework components").
				Description("space to toggle; enter to confirm").
				Options(
					huh.NewOption("cache      (in-memory KV)", "cache"),
					huh.NewOption("cron       (job scheduler)", "cron"),
					huh.NewOption("web        (echo HTTP server)", "web"),
					huh.NewOption("queue      (message queue)", "queue"),
					huh.NewOption("telemetry  (OpenTelemetry tracing + metrics)", "telemetry"),
					huh.NewOption("policy     (Casbin authorization)", "policy"),
					huh.NewOption("i18n       (gotext translations)", "i18n"),
					huh.NewOption("featureflag (GOFF)", "featureflag"),
					huh.NewOption("ai         (charm/fantasy LLM agent)", "ai"),
					huh.NewOption("frontend-js  (htmx + alpine + tailwind, no build)", "frontend-js"),
					huh.NewOption("frontend-ts  (htmx + alpine + tailwind + laravel-mix TS build)", "frontend-ts"),
				).
				Value(&extras),
			huh.NewMultiSelect[string]().
				Title("Developer tooling").
				Description("space to toggle; enter to confirm").
				Options(
					huh.NewOption("editorconfig (.editorconfig)", "editorconfig"),
					huh.NewOption("lint         (.golangci.yml)", "lint"),
					huh.NewOption("goreleaser   (.goreleaser.yaml)", "goreleaser"),
					huh.NewOption("github       (release workflow; implies goreleaser)", "github"),
				).
				Value(&tooling),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	applyExtras(cfg, extras)
	applyTooling(cfg, tooling)

	return nil
}

// applyExtras flips the cfg booleans / strings based on the multi-select
// result. Driver-selecting components get their default backend.
func applyExtras(cfg *initConfig, extras []string) {
	set := make(map[string]bool, len(extras))
	for _, e := range extras {
		set[e] = true
	}

	cfg.Cache = set["cache"]
	cfg.Cron = set["cron"]
	cfg.Web = set["web"]
	cfg.Policy = set["policy"]
	cfg.I18n = set["i18n"]
	cfg.Featureflag = set["featureflag"]

	if set["queue"] {
		if cfg.Queue == "" || cfg.Queue == "none" {
			cfg.Queue = "memory"
		}
	} else {
		cfg.Queue = "none"
	}

	if set["telemetry"] {
		if cfg.Telemetry == "" || cfg.Telemetry == "none" {
			cfg.Telemetry = "stdout"
		}
	} else {
		cfg.Telemetry = "none"
	}

	if set["ai"] {
		if cfg.AI == "" || cfg.AI == "none" {
			cfg.AI = "openai"
		}
	} else {
		cfg.AI = "none"
	}

	switch {
	case set["frontend-ts"]:
		cfg.Frontend = "ts"
	case set["frontend-js"]:
		cfg.Frontend = "js"
	default:
		cfg.Frontend = "none"
	}

	// Frontend requires web. Auto-enable web if the user picked a
	// frontend but forgot the web option.
	if cfg.Frontend != "" && cfg.Frontend != "none" {
		cfg.Web = true
	}
}

// applyTooling flips the cfg dev-tooling booleans based on the
// multi-select result. GithubActions requires Goreleaser — auto-enable
// it if the user picked github but forgot goreleaser.
func applyTooling(cfg *initConfig, tooling []string) {
	set := make(map[string]bool, len(tooling))
	for _, t := range tooling {
		set[t] = true
	}

	cfg.Editorconfig = set["editorconfig"]
	cfg.Lint = set["lint"]
	cfg.Goreleaser = set["goreleaser"]
	cfg.GithubActions = set["github"]

	if cfg.GithubActions {
		cfg.Goreleaser = true
	}
}

// validate rejects an initConfig with obviously bad inputs.
func (c initConfig) validate() error {
	if c.Name == "" {
		return errors.New("project name is empty")
	}

	if c.ModulePath == "" {
		return errors.New("module path is empty")
	}

	if c.BinaryName == "" {
		return errors.New("binary name is empty")
	}

	switch c.Dialect {
	case "sqlite", "postgres", "none", "":
	default:
		return fmt.Errorf("unknown --db value %q (want sqlite, postgres, or none)", c.Dialect)
	}

	switch c.Queue {
	case "", "none", "memory":
	default:
		return fmt.Errorf("unknown --queue value %q (want memory or none)", c.Queue)
	}

	switch c.Telemetry {
	case "", "none", "stdout", "otlp":
	default:
		return fmt.Errorf("unknown --telemetry value %q (want stdout, otlp, or none)", c.Telemetry)
	}

	switch c.AI {
	case "", "none", "openai", "anthropic":
	default:
		return fmt.Errorf("unknown --ai value %q (want openai, anthropic, or none)", c.AI)
	}

	switch c.Frontend {
	case "", "none", "js", "ts":
	default:
		return fmt.Errorf("unknown --frontend value %q (want js, ts, or none)", c.Frontend)
	}

	if c.Frontend != "" && c.Frontend != "none" && !c.Web {
		return fmt.Errorf("--frontend %q requires --web", c.Frontend)
	}

	return nil
}

// applyToolingDefaults auto-enables Goreleaser when GithubActions is
// requested via flags (the --yes / non-interactive path doesn't go
// through applyTooling).
func (c *initConfig) applyToolingDefaults() {
	if c.GithubActions {
		c.Goreleaser = true
	}
}

// scaffold materialises the project files at cfg.Directory using svc,
// honouring opts (dry-run / force / output format).
func scaffold(ctx context.Context, svc *generator.Service, cfg initConfig, opts generator.Options) ([]generator.Action, error) {
	var actions []generator.Action

	if !opts.Force {
		if _, err := os.Stat(filepath.Join(cfg.Directory, "go.mod")); err == nil {
			return actions, fmt.Errorf("%s already contains a go.mod (use --force to overwrite)", cfg.Directory)
		}
	}

	if act, err := svc.Mkdirp(cfg.Directory, opts); err != nil {
		return actions, err
	} else if act != nil {
		actions = append(actions, *act)
	}

	files := coreFiles(cfg)

	if cfg.HasDatabase() {
		files = append(files, databaseFiles(cfg)...)
	}

	if cfg.Cache {
		files = append(files, componentFiles("cache", cfg)...)
	}

	if cfg.Cron {
		files = append(files, componentFiles("cron", cfg)...)
	}

	if cfg.Web {
		files = append(files, componentFiles("web", cfg)...)
	}

	if cfg.HasQueue() {
		files = append(files, componentFiles("queue", cfg)...)
	}

	if cfg.HasTelemetry() {
		files = append(files, componentFiles("telemetry", cfg)...)
	}

	if cfg.Policy {
		files = append(files, componentFiles("policy", cfg)...)
	}

	if cfg.I18n {
		files = append(files, componentFiles("i18n", cfg)...)
	}

	if cfg.Featureflag {
		files = append(files, componentFiles("featureflag", cfg)...)
	}

	if cfg.HasAI() {
		files = append(files, componentFiles("ai", cfg)...)
	}

	if cfg.HasFrontend() {
		files = append(files, frontendFiles(cfg)...)
	}

	if cfg.Editorconfig {
		files = append(files, fileSpec{"templates/init/editorconfig.tmpl", filepath.Join(cfg.Directory, ".editorconfig")})
	}

	if cfg.Lint {
		files = append(files, fileSpec{"templates/init/golangci.yml.tmpl", filepath.Join(cfg.Directory, ".golangci.yml")})
	}

	if cfg.Goreleaser {
		files = append(files, fileSpec{"templates/init/goreleaser.yaml.tmpl", filepath.Join(cfg.Directory, ".goreleaser.yaml")})
	}

	if cfg.HasGithubActions() {
		files = append(files, fileSpec{"templates/init/github/release.yml.tmpl", filepath.Join(cfg.Directory, ".github", "workflows", "release.yml")})
	}

	for _, f := range files {
		act, err := svc.RenderFile(ctx, f.template, f.target, cfg, opts)
		if err != nil {
			return actions, err
		}

		actions = append(actions, act)
	}

	publishActions, err := publishFrameworkConfigs(svc, cfg, opts)
	actions = append(actions, publishActions...)

	if err != nil {
		return actions, err
	}

	// Empty directories worth committing.
	dirs := []string{
		filepath.Join(cfg.Directory, "domain"),
		filepath.Join(cfg.Directory, "infrastructure"),
		filepath.Join(cfg.Directory, "lib"),
	}

	for _, d := range dirs {
		if act, err := svc.Mkdirp(d, opts); err != nil {
			return actions, err
		} else if act != nil {
			actions = append(actions, *act)
		}

		keep := filepath.Join(d, ".gitkeep")
		if _, err := os.Stat(keep); err == nil {
			continue
		}

		act, err := svc.WriteRaw(keep, []byte{}, opts)
		if err != nil {
			return actions, err
		}

		actions = append(actions, act)
	}

	return actions, nil
}

// fileSpec is a template→target rendering job.
type fileSpec struct {
	template, target string
}

// coreFiles is the always-generated set. Log config is not templated;
// it's published verbatim from hex/log/provider.Configs() by
// publishFrameworkConfigs.
func coreFiles(cfg initConfig) []fileSpec {
	return []fileSpec{
		{"templates/init/main.go.tmpl", filepath.Join(cfg.Directory, "main.go")},
		{"templates/init/boot.go.tmpl", filepath.Join(cfg.Directory, "app", "boot.go")},
		{"templates/init/root.go.tmpl", filepath.Join(cfg.Directory, "app", "command", "root.go")},
		{"templates/init/build.go.tmpl", filepath.Join(cfg.Directory, "app", "build", "build.go")},
		{"templates/init/config.toml.tmpl", filepath.Join(cfg.Directory, "config", "app.toml")},
		{"templates/init/schema.cue.tmpl", filepath.Join(cfg.Directory, "config", "schema.cue")},
		{"templates/init/config_embed.go.tmpl", filepath.Join(cfg.Directory, "config", "config.go")},
		{"templates/init/provider_config.go.tmpl", filepath.Join(cfg.Directory, "app", "provider", "config.go")},
		{"templates/init/provider_log.go.tmpl", filepath.Join(cfg.Directory, "app", "provider", "log.go")},
		{"templates/init/provider_lua.go.tmpl", filepath.Join(cfg.Directory, "app", "provider", "lua.go")},
		{"templates/init/provider_repl_bindings.go.tmpl", filepath.Join(cfg.Directory, "app", "provider", "repl_bindings.go")},
		{"templates/init/justfile.tmpl", filepath.Join(cfg.Directory, "justfile")},
		{"templates/init/gitignore.tmpl", filepath.Join(cfg.Directory, ".gitignore")},
		{"templates/init/env.dist.tmpl", filepath.Join(cfg.Directory, ".env.dist")},
		{"templates/init/README.md.tmpl", filepath.Join(cfg.Directory, "README.md")},
		{"templates/init/AGENTS.md.tmpl", filepath.Join(cfg.Directory, "AGENTS.md")},
		{"templates/init/go.mod.tmpl", filepath.Join(cfg.Directory, "go.mod")},
		{"templates/init/air.toml.tmpl", filepath.Join(cfg.Directory, ".air.toml")},
	}
}

// publishFrameworkConfigs copies the config files that each enabled
// framework provider ships (via its Configs() fs.FS) into the
// consumer's config/ directory. Files with per-app content — the
// database dsn, the telemetry service_name — are still emitted from
// templates because they need per-app substitution; publish covers
// the universal-defaults cases.
func publishFrameworkConfigs(svc *generator.Service, cfg initConfig, opts generator.Options) ([]generator.Action, error) {
	var actions []generator.Action

	confDir := filepath.Join(cfg.Directory, "config")

	// Log always publishes (log provider is always registered).
	acts, err := svc.PublishAll(logprovider.Configs(), ".toml", confDir, opts)
	actions = append(actions, acts...)

	if err != nil {
		return actions, err
	}

	// Lua always publishes (Lua provider is always registered so
	// `<app> repl` works out of the box).
	acts, err = svc.PublishAll(luaprovider.Configs(), ".toml", confDir, opts)
	actions = append(actions, acts...)

	if err != nil {
		return actions, err
	}

	// Cache: universal defaults, no per-app content.
	if cfg.Cache {
		acts, err = svc.PublishAll(cacheprovider.Configs(), ".toml", confDir, opts)
		actions = append(actions, acts...)

		if err != nil {
			return actions, err
		}
	}

	// Web: universal defaults, no per-app content.
	if cfg.Web {
		acts, err = svc.PublishAll(webprovider.Configs(), ".toml", confDir, opts)
		actions = append(actions, acts...)

		if err != nil {
			return actions, err
		}
	}

	// AI: consumer's ai.toml is template-generated with per-app
	// provider + model; framework's ai.toml stays as fallback via
	// Sources.
	_ = aiprovider.Configs // silence unused import when we skip publishing

	// CUE schemas are NOT published — they stay in the framework
	// module's Configs() and are read at runtime via Sources. Consumer
	// adds their own per-namespace constraints in config/schema.cue.

	return actions, nil
}

func databaseFiles(cfg initConfig) []fileSpec {
	return []fileSpec{
		{"templates/init/database_provider.go.tmpl", filepath.Join(cfg.Directory, "app", "provider", "database.go")},
		{"templates/init/database.toml.tmpl", filepath.Join(cfg.Directory, "config", "database.toml")},
		{"templates/init/db_migrations.go.tmpl", filepath.Join(cfg.Directory, "database", "migrations.go")},
		{"templates/init/initial_migration.up.sql.tmpl", filepath.Join(cfg.Directory, "database", "migrations", "00000000000000_init.up.sql")},
		{"templates/init/initial_migration.down.sql.tmpl", filepath.Join(cfg.Directory, "database", "migrations", "00000000000000_init.down.sql")},
	}
}

// frontendFiles emits the frontend toolchain + web/views/public/
// scaffolding when --frontend is enabled. Shared between js and ts
// modes; TS-only files are gated inside the template via .FrontendTS.
func frontendFiles(cfg initConfig) []fileSpec {
	base := "templates/init/frontend"
	dir := cfg.Directory

	specs := []fileSpec{
		{base + "/package.json.tmpl", filepath.Join(dir, "package.json")},
		{base + "/biome.json.tmpl", filepath.Join(dir, "biome.json")},
		{base + "/vitest.config.js.tmpl", filepath.Join(dir, "vitest.config.js")},

		{base + "/resources_css_app.css.tmpl", filepath.Join(dir, "resources", "css", "app.css")},
		{base + "/web_views_layouts_main.gotmpl.tmpl", filepath.Join(dir, "web", "views", "layouts", "main.gotmpl")},
		{base + "/web_views_pages_home.gotmpl.tmpl", filepath.Join(dir, "web", "views", "pages", "home.gotmpl")},
		{base + "/web_views_embed.go.tmpl", filepath.Join(dir, "web", "views", "views.go")},
		{base + "/provider_view.go.tmpl", filepath.Join(dir, "app", "provider", "view.go")},
		{base + "/controller_home.go.tmpl", filepath.Join(dir, "app", "controller", "home.go")},
		{base + "/public_gitkeep.tmpl", filepath.Join(dir, "public", ".gitkeep")},
	}

	if cfg.FrontendTS() {
		specs = append(specs,
			fileSpec{base + "/tsconfig.json.tmpl", filepath.Join(dir, "tsconfig.json")},
			fileSpec{base + "/vite.config.js.tmpl", filepath.Join(dir, "vite.config.js")},
			fileSpec{base + "/resources_js_app.ts.tmpl", filepath.Join(dir, "resources", "js", "app.ts")},
		)
	} else {
		specs = append(specs,
			fileSpec{base + "/public_js_app.js.tmpl", filepath.Join(dir, "public", "js", "app.js")},
		)
	}

	return specs
}

// componentFiles returns the templates for an opt-in component. Config
// TOMLs for cache/web are NOT emitted here — they're published from
// each framework provider's Configs() in publishFrameworkConfigs.
// Telemetry and queue keep templated TOMLs because they carry per-app
// values (service_name / driver choice).
func componentFiles(name string, cfg initConfig) []fileSpec {
	base := "templates/init/components/" + name
	provDir := filepath.Join(cfg.Directory, "app", "provider")
	confDir := filepath.Join(cfg.Directory, "config")

	specs := []fileSpec{
		{base + "/provider.go.tmpl", filepath.Join(provDir, name+".go")},
	}

	// Config file per namespace read by the provider. Filename matches
	// the namespace hex/config parses out (namespace = filename minus
	// .toml).
	switch name {
	case "queue":
		specs = append(specs, fileSpec{base + "/config.toml.tmpl", filepath.Join(confDir, "queue.toml")})
	case "telemetry":
		specs = append(specs, fileSpec{base + "/config.toml.tmpl", filepath.Join(confDir, "telemetry.toml")})
	case "web":
		// Routes provider for app-owned HTTP routes; controllers live
		// in app/controller/ and are wired via hex make controller.
		specs = append(specs,
			fileSpec{base + "/routes.go.tmpl", filepath.Join(provDir, "routes.go")},
			fileSpec{"templates/init/controller.go.tmpl", filepath.Join(cfg.Directory, "app", "controller", "controller.go")},
		)
	case "ai":
		specs = append(specs, fileSpec{base + "/config.toml.tmpl", filepath.Join(confDir, "ai.toml")})
	}

	// Extra assets per component.
	switch name {
	case "policy":
		specs = append(specs,
			fileSpec{base + "/rbac_model.conf.tmpl", filepath.Join(cfg.Directory, "policy", "rbac_model.conf")},
			fileSpec{base + "/rbac_policy.csv.tmpl", filepath.Join(cfg.Directory, "policy", "rbac_policy.csv")},
			fileSpec{base + "/policy.go.tmpl", filepath.Join(cfg.Directory, "policy", "policy.go")},
		)
	case "i18n":
		specs = append(specs,
			fileSpec{base + "/messages_en.po.tmpl", filepath.Join(cfg.Directory, "locales", "en", "LC_MESSAGES", "messages.po")},
			fileSpec{base + "/locales.go.tmpl", filepath.Join(cfg.Directory, "locales", "locales.go")},
		)
	case "featureflag":
		specs = append(specs,
			fileSpec{base + "/flags.yaml.tmpl", filepath.Join(cfg.Directory, "flags", "flags.yaml")},
			fileSpec{base + "/flags.go.tmpl", filepath.Join(cfg.Directory, "flags", "flags.go")},
		)
	}

	return specs
}

func printInitSuccess(cmd *cobra.Command, cfg initConfig) {
	rel := cfg.Directory
	if cwd, err := os.Getwd(); err == nil {
		if r, rerr := filepath.Rel(cwd, cfg.Directory); rerr == nil {
			rel = r
		}
	}

	out := cmd.OutOrStdout()

	fmt.Fprintln(out)
	fmt.Fprintln(out, "hex project created at", rel)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Next steps:")

	if rel != "." {
		fmt.Fprintln(out, "  cd", rel)
	}

	fmt.Fprintln(out, "  go mod tidy")
	fmt.Fprintln(out, "  go run .")
	fmt.Fprintln(out)
}

// -- default helpers -----------------------------------------------------

func defaultModulePath(name string) string {
	u, err := user.Current()
	if err != nil || u.Username == "" {
		return "example.com/" + name
	}

	return "github.com/" + u.Username + "/" + name
}

func runningGoVersion() string {
	return "1.26"
}

// fallbackHexVersion is the last-resort require version used when neither
// the running CLI's own build version nor a proxy lookup is available
// (e.g. fully offline dev builds). Bump when cutting a new release.
const fallbackHexVersion = "v0.0.1"

// hexRequireVersion returns the hex library version to require in a
// scaffolded go.mod. It mirrors the running CLI's own build version, since
// cmd/hex and the hex library are versioned together from the same tag.
//
// Dev builds (no ldflags, no module version — e.g. `go run ./cmd/hex`) have
// no build version to mirror, so we ask the Go toolchain for the latest
// published version instead. "latest" is not itself valid go.mod syntax, so
// it must be resolved to a concrete version here rather than written as-is.
func hexRequireVersion() string {
	if v := build.Version(); v != build.UnknownVersion {
		return v
	}

	if v, err := latestPublishedHexVersion(); err == nil && v != "" {
		return v
	}

	return fallbackHexVersion
}

// latestPublishedHexVersion shells out to `go list` to resolve the newest
// version of the hex module known to the configured module proxy.
func latestPublishedHexVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Version}}", "github.com/jordanbrauer/hex@latest").Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}
