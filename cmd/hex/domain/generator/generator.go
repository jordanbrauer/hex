// Package generator is the Generator domain: it describes hex's own
// make generators as data (Blueprint) and records what applying one
// does (Action). It has no dependency on infrastructure; Repository
// (repository.go) is the port to the compiled-in template assets.
package generator

// Action records one filesystem effect a Blueprint application produced
// (or, under Options.DryRun, would produce).
type Action struct {
	// Kind is one of: create, overwrite, wire, skip, mkdir.
	Kind string `json:"kind"`
	// Path is the file or directory the action targets.
	Path string `json:"path"`
	// Detail is optional context (e.g. the symbol wired, or why a write
	// was skipped).
	Detail string `json:"detail,omitempty"`
}

// FileSpec is one template-to-target mapping a Blueprint renders. Target
// is a Go template string evaluated against the same data passed to
// Service.Run/Apply, so the destination can vary per invocation (e.g.
// "app/provider/{{.Package}}.go").
type FileSpec struct {
	Template string
	Target   string
}

// WireSpec describes one marker-comment insertion a Blueprint performs
// after rendering its files — e.g. registering a new provider in
// app/boot.go above `// hex:providers`. File, Marker, Insertion, and
// Detail are Go template strings evaluated against the invocation data.
// Detail labels the recorded action (e.g. "added Payments") and may be
// empty.
type WireSpec struct {
	File      string
	Marker    string
	Insertion string
	Detail    string
}

// Blueprint describes one hex make generator: which files it renders
// and where, and which existing files it wires new registrations into.
// The built-in blueprints (provider, domain, adapter, controller,
// command, migration) are defined by infrastructure/embedfs; a future
// `hex make blueprint` could let consumers register their own via
// Repository.Store.
type Blueprint struct {
	ID          string
	Name        string
	Description string
	Files       []FileSpec
	Wires       []WireSpec
}
