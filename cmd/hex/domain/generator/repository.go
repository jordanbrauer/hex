package generator

import "context"

// Repository is the port to Blueprint definitions and the raw template
// bytes they reference. The built-in blueprints are backed by
// infrastructure/embedfs — the CLI's compiled-in templates. Store/Delete
// exist so a future `hex make blueprint` could let consumers register
// custom generators without writing Go code; the built-in adapter itself
// need not support them.
type Repository interface {
	// Get returns the blueprint identified by id, or ErrBlueprintNotFound.
	Get(ctx context.Context, id string) (*Blueprint, error)
	// List returns every known blueprint.
	List(ctx context.Context) ([]*Blueprint, error)
	// Store registers or replaces a blueprint.
	Store(ctx context.Context, bp *Blueprint) error
	// Delete removes a blueprint by id.
	Delete(ctx context.Context, id string) error
	// Read returns the raw contents of a template file a Blueprint's
	// FileSpec.Template refers to (e.g. "templates/provider.go.tmpl").
	Read(ctx context.Context, path string) ([]byte, error)
}
