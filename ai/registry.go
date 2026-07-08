package ai

import (
	"sort"
	"sync"
)

// ToolRegistry looks up agent tools by name. Any hex module that needs
// to dispatch to a tool by string identifier — the Lua bindings,
// future MCP bridges, an HTTP tool-proxy endpoint — resolves the
// registry from the container and reads it here.
//
// The set of tools a registry contains is orthogonal to what any
// given fantasy.Agent has attached. An agent is typically constructed
// with the full registry contents (see hex/ai/provider.Provider) so
// each fantasy.AgentCall can filter to a subset via ActiveTools.
type ToolRegistry interface {
	// Get returns the tool for name, ok=false when unknown.
	Get(name string) (Tool, bool)

	// Names returns every registered tool name in a stable sorted
	// order. Useful for --list-tools style commands and Lua
	// introspection.
	Names() []string

	// All returns every registered tool in the same order as Names.
	All() []Tool
}

// NewToolRegistry returns a mutable, thread-safe registry seeded with
// the provided tools. Tools whose name collides overwrite silently —
// callers who want to detect duplicates should pass unique names or
// use a wrapping registry.
func NewToolRegistry(tools ...Tool) *MemoryToolRegistry {
	r := &MemoryToolRegistry{tools: make(map[string]Tool, len(tools))}
	for _, t := range tools {
		r.Add(t)
	}

	return r
}

// MemoryToolRegistry is the default in-memory ToolRegistry
// implementation.
type MemoryToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// Add inserts or replaces a tool. Ignores nil.
func (r *MemoryToolRegistry) Add(tool Tool) {
	if tool == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[tool.Info().Name] = tool
}

// Remove deletes a tool by name. Missing names are ignored.
func (r *MemoryToolRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.tools, name)
}

// Get returns the tool for name.
func (r *MemoryToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.tools[name]

	return t, ok
}

// Names returns the registered tool names in stable order.
func (r *MemoryToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// All returns every tool in the same order as Names.
func (r *MemoryToolRegistry) All() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}

	sort.Strings(names)

	tools := make([]Tool, 0, len(names))
	for _, name := range names {
		tools = append(tools, r.tools[name])
	}

	return tools
}
