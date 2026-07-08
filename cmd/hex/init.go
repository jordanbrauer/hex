package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	cacheprovider "github.com/jordanbrauer/hex/cache/provider"
	logprovider "github.com/jordanbrauer/hex/log/provider"
	webprovider "github.com/jordanbrauer/hex/web/provider"
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

func newInitCommand() *cobra.Command {
	var (
		modulePath  string
		dialect     string
		cache       bool
		cron        bool
		web         bool
		queue       string
		telemetry   string
		policy      bool
		i18nFlag    bool
		featureflag bool
		yes         bool
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Scaffold a new hex project",
		Long: "Create a new hex application in the given directory (or the current one).\n" +
			"Run without a name to scaffold into `.`; otherwise a subdirectory is created.\n\n" +
			"Interactive prompts fill in the module path, binary name, and which framework\n" +
			"components to enable. Pass --yes to skip prompts and use flag defaults.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := resolveInitConfig(args, resolveFlags{
				module:      modulePath,
				dialect:     dialect,
				cache:       cache,
				cron:        cron,
				web:         web,
				queue:       queue,
				telemetry:   telemetry,
				policy:      policy,
				i18n:        i18nFlag,
				featureflag: featureflag,
				yes:         yes,
			})
			if err != nil {
				return err
			}

			if err := scaffold(cfg, force); err != nil {
				return err
			}

			printInitSuccess(cfg)

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
	cmd.Flags().BoolVar(&yes, "yes", false, "skip interactive prompts, use flag defaults")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")

	return cmd
}

// resolveFlags carries flag values from newInitCommand into
// resolveInitConfig without expanding the argument list.
type resolveFlags struct {
	module      string
	dialect     string
	cache       bool
	cron        bool
	web         bool
	queue       string
	telemetry   string
	policy      bool
	i18n        bool
	featureflag bool
	yes         bool
}

// resolveInitConfig combines CLI args, flags, and interactive prompts
// into a fully-populated initConfig.
func resolveInitConfig(args []string, f resolveFlags) (initConfig, error) {
	cfg := initConfig{
		Dialect:     f.dialect,
		ModulePath:  f.module,
		Cache:       f.cache,
		Cron:        f.cron,
		Web:         f.web,
		Queue:       f.queue,
		Telemetry:   f.telemetry,
		Policy:      f.policy,
		I18n:        f.i18n,
		Featureflag: f.featureflag,
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
				).
				Value(&extras),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	applyExtras(cfg, extras)

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

	if c.Dialect == "" {
		c.Dialect = "sqlite"
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

	return nil
}

// scaffold materialises the project files at cfg.Directory.
func scaffold(cfg initConfig, force bool) error {
	if err := os.MkdirAll(cfg.Directory, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", cfg.Directory, err)
	}

	if !force {
		if _, err := os.Stat(filepath.Join(cfg.Directory, "go.mod")); err == nil {
			return fmt.Errorf("%s already contains a go.mod (use --force to overwrite)", cfg.Directory)
		}
	}

	g := newGenerator()
	g.force = force

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

	for _, f := range files {
		if err := g.render(f.template, f.target, cfg); err != nil {
			return err
		}
	}

	if err := publishFrameworkConfigs(g, cfg); err != nil {
		return err
	}

	// Empty directories worth committing.
	dirs := []string{
		filepath.Join(cfg.Directory, "domain"),
		filepath.Join(cfg.Directory, "infrastructure"),
		filepath.Join(cfg.Directory, "lib"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}

		if err := writeIfMissing(filepath.Join(d, ".gitkeep"), nil); err != nil {
			return err
		}
	}

	return nil
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
		{"templates/init/justfile.tmpl", filepath.Join(cfg.Directory, "justfile")},
		{"templates/init/gitignore.tmpl", filepath.Join(cfg.Directory, ".gitignore")},
		{"templates/init/env.dist.tmpl", filepath.Join(cfg.Directory, ".env.dist")},
		{"templates/init/README.md.tmpl", filepath.Join(cfg.Directory, "README.md")},
		{"templates/init/go.mod.tmpl", filepath.Join(cfg.Directory, "go.mod")},
	}
}

// publishFrameworkConfigs copies the config files that each enabled
// framework provider ships (via its Configs() fs.FS) into the
// consumer's config/ directory. Files with per-app content — the
// database dsn, the telemetry service_name — are still emitted from
// templates because they need per-app substitution; publish covers
// the universal-defaults cases.
func publishFrameworkConfigs(g *generator, cfg initConfig) error {
	confDir := filepath.Join(cfg.Directory, "config")

	// Log always publishes (log provider is always registered).
	if _, err := g.publishAll(logprovider.Configs(), ".toml", confDir); err != nil {
		return err
	}

	// Cache: universal defaults, no per-app content.
	if cfg.Cache {
		if _, err := g.publishAll(cacheprovider.Configs(), ".toml", confDir); err != nil {
			return err
		}
	}

	// Web: universal defaults, no per-app content.
	if cfg.Web {
		if _, err := g.publishAll(webprovider.Configs(), ".toml", confDir); err != nil {
			return err
		}
	}

	// CUE schemas are NOT published — they stay in the framework
	// module's Configs() and are read at runtime via Sources. Consumer
	// adds their own per-namespace constraints in config/schema.cue.

	return nil
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
		// in app/controller/ and are wired via hex make:controller.
		specs = append(specs,
			fileSpec{base + "/routes.go.tmpl", filepath.Join(provDir, "routes.go")},
			fileSpec{"templates/init/controller.go.tmpl", filepath.Join(cfg.Directory, "app", "controller", "controller.go")},
		)
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

func writeIfMissing(path string, content []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	if content == nil {
		content = []byte{}
	}

	return os.WriteFile(path, content, 0o644)
}

func printInitSuccess(cfg initConfig) {
	rel := cfg.Directory
	if cwd, err := os.Getwd(); err == nil {
		if r, rerr := filepath.Rel(cwd, cfg.Directory); rerr == nil {
			rel = r
		}
	}

	fmt.Println()
	fmt.Println("hex project created at", rel)
	fmt.Println()
	fmt.Println("Next steps:")

	if rel != "." {
		fmt.Println("  cd", rel)
	}

	fmt.Println("  go mod tidy")
	fmt.Println("  go run .")
	fmt.Println()
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
