// Package plugin discovers repo-local Cobra commands written in Lua,
// Teal, or Fennel and merges them into an existing command tree.
//
// A plugin is a directory containing a config.toml manifest and a
// run.{lua,tl,fnl} entrypoint:
//
//	.hex/command/hello/
//	├── config.toml
//	└── run.lua
//
// config.toml:
//
//	use   = "hello"
//	short = "Say hello"
//
//	[flags]
//	name = { type = "string", usage = "who to greet", value = "world" }
//
// run.lua:
//
//	local name = cmd.flags().get_string("name")
//	print("Hello, " .. name .. "!")
//
// Subdirectories become subcommands by listing their names in the
// parent manifest's `commands` array. Call Discover or LoadInto to
// turn a directory tree into *cobra.Command values; NewRuntimeExecutor
// wires the resulting commands to a real hex/lua Environment.
//
// This package is the concrete instance of the escape hatch
// docs/adr/0007-lua-runtime-only.md reserved for later: hex/lua itself
// stays a bare runtime, and the plugin/discovery convention lives
// here instead.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// FlagConfig describes one flag declared in a plugin's config.toml
// [flags] table. Type is "string"/"str", "bool"/"boolean", or
// "number", optionally suffixed "[]" for the slice variant.
type FlagConfig struct {
	Type  string `toml:"type"`
	Usage string `toml:"usage"`
	Short string `toml:"short"`
	Value any    `toml:"value"`
}

func (f FlagConfig) hasShort() bool     { return strings.TrimSpace(f.Short) != "" }
func (f FlagConfig) isArray() bool      { return strings.HasSuffix(f.Type, "[]") }
func (f FlagConfig) scalarType() string { return strings.TrimSuffix(f.Type, "[]") }

// Register adds the flag described by f to flags under name.
func (f FlagConfig) Register(name string, flags *pflag.FlagSet) {
	switch f.scalarType() {
	case "string", "str":
		if f.isArray() {
			dv := toStringSlice(f.Value)
			if f.hasShort() {
				flags.StringSliceP(name, f.Short, dv, f.Usage)
			} else {
				flags.StringSlice(name, dv, f.Usage)
			}

			return
		}

		dv, _ := f.Value.(string)
		if f.hasShort() {
			flags.StringP(name, f.Short, dv, f.Usage)
		} else {
			flags.String(name, dv, f.Usage)
		}

	case "boolean", "bool":
		if f.isArray() {
			dv := toBoolSlice(f.Value)
			if f.hasShort() {
				flags.BoolSliceP(name, f.Short, dv, f.Usage)
			} else {
				flags.BoolSlice(name, dv, f.Usage)
			}

			return
		}

		dv, _ := f.Value.(bool)
		if f.hasShort() {
			flags.BoolP(name, f.Short, dv, f.Usage)
		} else {
			flags.Bool(name, dv, f.Usage)
		}

	case "number":
		if f.isArray() {
			dv := toFloat64Slice(f.Value)
			if f.hasShort() {
				flags.Float64SliceP(name, f.Short, dv, f.Usage)
			} else {
				flags.Float64Slice(name, dv, f.Usage)
			}

			return
		}

		dv := toFloat64(f.Value)
		if f.hasShort() {
			flags.Float64P(name, f.Short, dv, f.Usage)
		} else {
			flags.Float64(name, dv, f.Usage)
		}
	}
}

// toFloat64 converts a TOML numeric value (int64 or float64) to float64.
func toFloat64(v any) float64 {
	switch n := v.(type) {
	case int64:
		return float64(n)
	case float64:
		return n
	default:
		return 0
	}
}

func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return []string{}
	}

	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}

	return out
}

func toBoolSlice(v any) []bool {
	arr, ok := v.([]any)
	if !ok {
		return []bool{}
	}

	out := make([]bool, 0, len(arr))
	for _, item := range arr {
		if b, ok := item.(bool); ok {
			out = append(out, b)
		}
	}

	return out
}

func toFloat64Slice(v any) []float64 {
	arr, ok := v.([]any)
	if !ok {
		return []float64{}
	}

	out := make([]float64, 0, len(arr))
	for _, item := range arr {
		out = append(out, toFloat64(item))
	}

	return out
}

// Manifest is a plugin's config.toml.
type Manifest struct {
	Use      string                `toml:"use"`
	Aliases  []string              `toml:"aliases"`
	Short    string                `toml:"short"`
	Long     string                `toml:"long"`
	Commands []string              `toml:"commands"`
	Flags    map[string]FlagConfig `toml:"flags"`
}

// Plugin is a loaded plugin directory: its manifest plus the
// filesystem location it was read from.
type Plugin struct {
	Manifest

	dir string
}

// runEntrypoints and argsEntrypoints are the recognized entrypoint
// filenames, one per supported language. Exactly one of each may be
// present in a plugin directory.
var (
	runEntrypoints  = []string{"run.lua", "run.tl", "run.fnl"}
	argsEntrypoints = []string{"args.lua", "args.tl", "args.fnl"}
)

// NewPlugin reads dir/config.toml and returns the loaded Plugin.
func NewPlugin(dir string) (*Plugin, error) {
	data, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		return nil, fmt.Errorf("plugin: read %s: %w", dir, err)
	}

	p := &Plugin{dir: dir}

	if err := toml.Unmarshal(data, &p.Manifest); err != nil {
		return nil, fmt.Errorf("plugin: parse %s: %w", filepath.Join(dir, "config.toml"), err)
	}

	if p.Use == "" {
		return nil, fmt.Errorf("plugin %s: config.toml missing required `use`", dir)
	}

	return p, nil
}

// findEntrypoint returns the single file among candidates that exists
// in dir, "" if none do, or an error if more than one does.
func findEntrypoint(dir string, candidates []string) (string, error) {
	var found []string

	for _, name := range candidates {
		p := filepath.Join(dir, name)

		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			found = append(found, p)
		}
	}

	switch len(found) {
	case 0:
		return "", nil
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("plugin %s: ambiguous entrypoint, found %v (want exactly one)", dir, found)
	}
}

// Executor runs a plugin entrypoint file (run.* or args.*) for one
// invocation of cmd with args. NewRuntimeExecutor builds the default
// implementation, which executes the file against a fresh hex/lua
// Environment.
type Executor func(path string, cmd *cobra.Command, args []string) error

// Command recursively builds a *cobra.Command for p and its child
// plugins (declared via Manifest.Commands), wiring run.* to RunE and
// args.* to Args through exec.
func (p *Plugin) Command(exec Executor) (*cobra.Command, error) {
	runPath, err := findEntrypoint(p.dir, runEntrypoints)
	if err != nil {
		return nil, err
	}

	argsPath, err := findEntrypoint(p.dir, argsEntrypoints)
	if err != nil {
		return nil, err
	}

	if runPath == "" && len(p.Commands) == 0 {
		return nil, fmt.Errorf("plugin %s: missing run.{lua,tl,fnl} (or a non-empty `commands` list)", p.dir)
	}

	cmd := &cobra.Command{
		Use:          p.Use,
		Aliases:      p.Aliases,
		Short:        p.Short,
		Long:         p.Long,
		SilenceUsage: true,
	}

	for name, fc := range p.Flags {
		fc.Register(name, cmd.Flags())
	}

	if runPath != "" {
		cmd.RunE = func(c *cobra.Command, args []string) error {
			return exec(runPath, c, args)
		}
	}

	if argsPath != "" {
		cmd.Args = func(c *cobra.Command, args []string) error {
			return exec(argsPath, c, args)
		}
	}

	for _, name := range p.Commands {
		childDir := filepath.Join(p.dir, name)

		info, statErr := os.Stat(childDir)
		if statErr != nil || !info.IsDir() {
			return nil, fmt.Errorf("plugin %s: child command %q not found", p.dir, name)
		}

		child, err := NewPlugin(childDir)
		if err != nil {
			return nil, err
		}

		childCmd, err := child.Command(exec)
		if err != nil {
			return nil, err
		}

		cmd.AddCommand(childCmd)
	}

	return cmd, nil
}
