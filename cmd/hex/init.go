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

	// Dialect is "sqlite", "postgres", or "none".
	Dialect string

	// GoVersion is the Go directive (e.g. "1.26"). Defaults to the
	// running toolchain.
	GoVersion string

	// HexVersion is the hex library version to require. Empty means the
	// tool copies its own build info.
	HexVersion string
}

func newInitCommand() *cobra.Command {
	var (
		modulePath string
		dialect    string
		yes        bool
		force      bool
	)

	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Scaffold a new hex project",
		Long: "Create a new hex application in the given directory (or the current one).\n" +
			"Run without a name to scaffold into `.`; otherwise a subdirectory is created.\n\n" +
			"Interactive prompts fill in the module path, database dialect, and binary name.\n" +
			"Pass --yes to skip prompts and take defaults.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := resolveInitConfig(args, modulePath, dialect, yes)
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
	cmd.Flags().StringVar(&dialect, "dialect", "", "database dialect: sqlite, postgres, none")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip interactive prompts, use defaults")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")

	return cmd
}

// resolveInitConfig combines CLI args, flags, and interactive prompts
// into a fully-populated initConfig.
func resolveInitConfig(args []string, flagModule, flagDialect string, yes bool) (initConfig, error) {
	cfg := initConfig{
		Dialect:    flagDialect,
		ModulePath: flagModule,
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

	if cfg.Dialect == "" {
		cfg.Dialect = "sqlite"
	}

	cfg.GoVersion = runningGoVersion()
	cfg.HexVersion = "" // resolved later against latest tag

	if yes {
		return cfg, cfg.validate()
	}

	if err := runInitPrompts(&cfg); err != nil {
		return cfg, err
	}

	return cfg, cfg.validate()
}

// runInitPrompts asks the user to confirm the module path, binary name,
// and dialect via huh.
func runInitPrompts(cfg *initConfig) error {
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
		),
	)

	return form.Run()
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
	case "sqlite", "postgres", "none":
	default:
		return fmt.Errorf("unknown dialect %q (want sqlite, postgres, or none)", c.Dialect)
	}

	return nil
}

// scaffold materialises the project files at cfg.Directory.
func scaffold(cfg initConfig, force bool) error {
	if err := os.MkdirAll(cfg.Directory, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", cfg.Directory, err)
	}

	// Refuse to scaffold into a non-empty directory unless --force. The
	// naive check: does go.mod already exist? Users almost never want to
	// clobber that.
	if !force {
		if _, err := os.Stat(filepath.Join(cfg.Directory, "go.mod")); err == nil {
			return fmt.Errorf("%s already contains a go.mod (use --force to overwrite)", cfg.Directory)
		}
	}

	g := newGenerator()
	g.force = force

	// Files rendered from templates.
	files := []struct{ template, target string }{
		{"templates/init/main.go.tmpl", filepath.Join(cfg.Directory, "main.go")},
		{"templates/init/boot.go.tmpl", filepath.Join(cfg.Directory, "app", "boot.go")},
		{"templates/init/root.go.tmpl", filepath.Join(cfg.Directory, "app", "command", "root.go")},
		{"templates/init/build.go.tmpl", filepath.Join(cfg.Directory, "app", "build", "build.go")},
		{"templates/init/config.toml.tmpl", filepath.Join(cfg.Directory, "config", "app.toml")},
		{"templates/init/config_embed.go.tmpl", filepath.Join(cfg.Directory, "config", "config.go")},
		{"templates/init/provider_config.go.tmpl", filepath.Join(cfg.Directory, "app", "provider", "config.go")},
		{"templates/init/provider_log.go.tmpl", filepath.Join(cfg.Directory, "app", "provider", "log.go")},
		{"templates/init/justfile.tmpl", filepath.Join(cfg.Directory, "justfile")},
		{"templates/init/gitignore.tmpl", filepath.Join(cfg.Directory, ".gitignore")},
		{"templates/init/env.dist.tmpl", filepath.Join(cfg.Directory, ".env.dist")},
		{"templates/init/README.md.tmpl", filepath.Join(cfg.Directory, "README.md")},
		{"templates/init/go.mod.tmpl", filepath.Join(cfg.Directory, "go.mod")},
	}

	if cfg.Dialect != "none" {
		files = append(files,
			struct{ template, target string }{
				"templates/init/database_provider.go.tmpl",
				filepath.Join(cfg.Directory, "app", "provider", "database.go"),
			},
			struct{ template, target string }{
				"templates/init/database.toml.tmpl",
				filepath.Join(cfg.Directory, "config", "database.toml"),
			},
			struct{ template, target string }{
				"templates/init/db_migrations.go.tmpl",
				filepath.Join(cfg.Directory, "database", "migrations.go"),
			},
			struct{ template, target string }{
				"templates/init/initial_migration.up.sql.tmpl",
				filepath.Join(cfg.Directory, "database", "migrations", "00000000000000_init.up.sql"),
			},
			struct{ template, target string }{
				"templates/init/initial_migration.down.sql.tmpl",
				filepath.Join(cfg.Directory, "database", "migrations", "00000000000000_init.down.sql"),
			},
		)
	}

	for _, f := range files {
		if err := g.render(f.template, f.target, cfg); err != nil {
			return err
		}
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
	// runtime.Version() returns "go1.26.4" — the go.mod directive wants "1.26".
	// We keep this simple and fall back to "1.26" (hex's minimum). Consumers
	// who need a specific version can edit go.mod after init.
	return "1.26"
}
