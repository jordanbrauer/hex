// Package policy is a thin wrapper around github.com/casbin/casbin/v2 that
// gives hex applications a portable authorization primitive.
//
// Casbin evaluates access decisions against a model (a DSL config that
// declares request/policy shapes, role definitions, matchers, and
// effect) combined with an adapter (the storage backend for policy
// rules). One engine supports ACL, RBAC, ABAC, RESTful, and hybrid
// models — the model file decides which one you get.
//
// hex/policy does not re-invent Casbin's API. It exposes:
//
//   - type aliases for Enforcer, Model, and Adapter so consumers get the
//     full Casbin API without importing casbin/v2 directly;
//   - hex-owned constructors that accept model definitions from strings,
//     paths, or embed.FS and pair them with an adapter of your choice;
//   - a memory adapter for tests and single-process deployments;
//   - a file adapter for CSV-backed policies (Casbin's built-in).
//
// See ADR-0011.
//
// # Model config from embed.FS
//
//	//go:embed policy/rbac.conf policy/policy.csv
//	var fs embed.FS
//
//	enf, err := policy.NewFromFS(fs, "policy/rbac.conf", policy.NewFileAdapterFS(fs, "policy/policy.csv"))
//	if err != nil { return err }
//
//	ok, _ := enf.Enforce("alice", "data1", "read")
//
// # Model config from a string
//
//	enf, err := policy.NewFromString(rbacModel, policy.NewMemoryAdapter())
package policy

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	casbinpersist "github.com/casbin/casbin/v2/persist"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
)

// Enforcer is the type alias for Casbin's SyncedEnforcer. Consumers get the
// full Casbin surface (AddPolicy, RemovePolicy, LoadPolicy, GetRolesForUser,
// etc.) through this alias.
type Enforcer = casbin.SyncedEnforcer

// Model is the type alias for a parsed Casbin model.
type Model = casbinmodel.Model

// Adapter is the type alias for Casbin's storage adapter interface.
// Implementations back the policies with memory, files, databases, etc.
type Adapter = casbinpersist.Adapter

// New constructs an Enforcer from an already-parsed Model and an Adapter.
// Use this when you have built the model yourself; for typical usage
// prefer NewFromString, NewFromFile, or NewFromFS.
func New(m Model, a Adapter) (*Enforcer, error) {
	if m == nil {
		return nil, errors.New("policy: model is required")
	}

	if a == nil {
		return nil, errors.New("policy: adapter is required")
	}

	enf, err := casbin.NewSyncedEnforcer(m, a)
	if err != nil {
		return nil, fmt.Errorf("policy: new enforcer: %w", err)
	}

	// Load initial policies from the adapter.
	if err := enf.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("policy: load policy: %w", err)
	}

	return enf, nil
}

// NewFromString parses a model config from source and returns an Enforcer
// backed by adapter. Consumers who embed a .conf via //go:embed can call
// this with string(embeddedBytes).
func NewFromString(source string, adapter Adapter) (*Enforcer, error) {
	m, err := casbinmodel.NewModelFromString(source)
	if err != nil {
		return nil, fmt.Errorf("policy: parse model: %w", err)
	}

	return New(m, adapter)
}

// NewFromFile reads and parses a model config file, returning an Enforcer
// backed by adapter.
func NewFromFile(path string, adapter Adapter) (*Enforcer, error) {
	m, err := casbinmodel.NewModelFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("policy: parse model %s: %w", path, err)
	}

	return New(m, adapter)
}

// NewFromFS reads a model config file from an fs.FS (typically an
// //go:embed FS) and returns an Enforcer backed by adapter.
func NewFromFS(f fs.FS, modelPath string, adapter Adapter) (*Enforcer, error) {
	data, err := fs.ReadFile(f, modelPath)
	if err != nil {
		return nil, fmt.Errorf("policy: read model %s: %w", modelPath, err)
	}

	return NewFromString(string(data), adapter)
}

// -- adapters --------------------------------------------------------------

// NewMemoryAdapter returns an in-memory policy adapter with no seed policies.
// Add policies via Enforcer.AddPolicy at runtime.
func NewMemoryAdapter() Adapter {
	return NewMemoryAdapterWithPolicies(nil)
}

// NewMemoryAdapterWithPolicies returns an in-memory adapter pre-loaded
// with the given policy rows. Each row is a []string matching the policy
// definition in the model (e.g. []string{"p", "alice", "data1", "read"}).
func NewMemoryAdapterWithPolicies(rows [][]string) Adapter {
	return &memoryAdapter{seed: rows}
}

// NewFileAdapter returns Casbin's CSV file adapter for the given path.
// Reads and writes happen synchronously on the file.
func NewFileAdapter(path string) Adapter {
	return fileadapter.NewAdapter(path)
}

// NewFileAdapterFS returns a read-only adapter backed by a file in an
// fs.FS (typically an //go:embed FS). Writes (AddPolicy/RemovePolicy)
// will not persist through this adapter — it is intended for shipping
// baseline policies immutable at build time.
func NewFileAdapterFS(f fs.FS, path string) Adapter {
	data, err := fs.ReadFile(f, path)
	if err != nil {
		// Adapters run at LoadPolicy time; returning a broken adapter with
		// a stored error lets the caller surface it there instead of
		// panicking at construction.
		return &fsAdapter{err: fmt.Errorf("policy: read %s: %w", path, err)}
	}

	return &fsAdapter{data: data}
}
