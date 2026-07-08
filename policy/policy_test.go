package policy_test

import (
	"embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/jordanbrauer/hex/policy"
)

//go:embed testdata/*
var fixtures embed.FS

// -- construction ---------------------------------------------------------

func TestNewFromFS_rbacEnforces(t *testing.T) {
	adapter := policy.NewFileAdapterFS(fixtures, "testdata/rbac_policy.csv")

	enf, err := policy.NewFromFS(fixtures, "testdata/rbac_model.conf", adapter)
	if err != nil {
		t.Fatalf("NewFromFS: %v", err)
	}

	tests := []struct {
		sub, obj, act string
		want          bool
	}{
		{"alice", "/data", "read", true},  // alice → admin → read
		{"alice", "/data", "write", true}, // alice → admin → write
		{"bob", "/data", "read", true},    // bob → viewer → read
		{"bob", "/data", "write", false},  // viewer has no write
		{"eve", "/data", "read", false},   // eve has no role
	}

	for _, tt := range tests {
		got, err := enf.Enforce(tt.sub, tt.obj, tt.act)
		if err != nil {
			t.Errorf("Enforce(%s, %s, %s) error: %v", tt.sub, tt.obj, tt.act, err)

			continue
		}

		if got != tt.want {
			t.Errorf("Enforce(%s, %s, %s) = %v, want %v", tt.sub, tt.obj, tt.act, got, tt.want)
		}
	}
}

func TestNewFromString_readsModelSource(t *testing.T) {
	modelBytes, err := fixtures.ReadFile("testdata/acl_model.conf")
	if err != nil {
		t.Fatal(err)
	}

	enf, err := policy.NewFromString(string(modelBytes), policy.NewMemoryAdapterWithPolicies([][]string{
		{"p", "alice", "/data", "read"},
	}))
	if err != nil {
		t.Fatalf("NewFromString: %v", err)
	}

	ok, err := enf.Enforce("alice", "/data", "read")
	if err != nil {
		t.Fatalf("Enforce: %v", err)
	}

	if !ok {
		t.Errorf("alice should have read")
	}
}

func TestNewFromFile_readsFromDisk(t *testing.T) {
	// Copy the embedded model to a temp file so we exercise NewFromFile.
	dir := t.TempDir()

	modelBytes, _ := fixtures.ReadFile("testdata/acl_model.conf")
	modelPath := filepath.Join(dir, "acl.conf")

	if err := os.WriteFile(modelPath, modelBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	enf, err := policy.NewFromFile(modelPath, policy.NewMemoryAdapterWithPolicies([][]string{
		{"p", "u1", "obj", "act"},
	}))
	if err != nil {
		t.Fatalf("NewFromFile: %v", err)
	}

	ok, _ := enf.Enforce("u1", "obj", "act")
	if !ok {
		t.Errorf("expected u1 permitted")
	}
}

func TestNew_requiresModelAndAdapter(t *testing.T) {
	if _, err := policy.New(nil, policy.NewMemoryAdapter()); err == nil {
		t.Errorf("New(nil model) returned nil error")
	}

	// Building a valid model to isolate the adapter check.
	modelBytes, _ := fixtures.ReadFile("testdata/acl_model.conf")

	enf, err := policy.NewFromString(string(modelBytes), policy.NewMemoryAdapter())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := policy.New(enf.GetModel(), nil); err == nil {
		t.Errorf("New(nil adapter) returned nil error")
	}
}

// -- memory adapter -------------------------------------------------------

func TestMemoryAdapter_addRemovePolicyAtRuntime(t *testing.T) {
	modelBytes, _ := fixtures.ReadFile("testdata/acl_model.conf")

	enf, err := policy.NewFromString(string(modelBytes), policy.NewMemoryAdapter())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// No policies yet — nothing is allowed.
	ok, _ := enf.Enforce("alice", "/data", "read")
	if ok {
		t.Errorf("empty policy set allowed alice")
	}

	if _, err := enf.AddPolicy("alice", "/data", "read"); err != nil {
		t.Fatalf("AddPolicy: %v", err)
	}

	ok, _ = enf.Enforce("alice", "/data", "read")
	if !ok {
		t.Errorf("after AddPolicy, alice denied")
	}

	if _, err := enf.RemovePolicy("alice", "/data", "read"); err != nil {
		t.Fatalf("RemovePolicy: %v", err)
	}

	ok, _ = enf.Enforce("alice", "/data", "read")
	if ok {
		t.Errorf("after RemovePolicy, alice still allowed")
	}
}

func TestMemoryAdapter_seedPoliciesLoadedOnConstruction(t *testing.T) {
	modelBytes, _ := fixtures.ReadFile("testdata/acl_model.conf")

	adapter := policy.NewMemoryAdapterWithPolicies([][]string{
		{"p", "carol", "/api", "read"},
		{"p", "carol", "/api", "write"},
	})

	enf, err := policy.NewFromString(string(modelBytes), adapter)
	if err != nil {
		t.Fatal(err)
	}

	for _, act := range []string{"read", "write"} {
		ok, err := enf.Enforce("carol", "/api", act)
		if err != nil {
			t.Errorf("Enforce %s: %v", act, err)

			continue
		}

		if !ok {
			t.Errorf("carol should have %s from seed", act)
		}
	}
}

func TestMemoryAdapter_removeFilteredPolicy(t *testing.T) {
	modelBytes, _ := fixtures.ReadFile("testdata/acl_model.conf")

	enf, err := policy.NewFromString(string(modelBytes), policy.NewMemoryAdapter())
	if err != nil {
		t.Fatal(err)
	}

	_, _ = enf.AddPolicy("alice", "/a", "read")
	_, _ = enf.AddPolicy("alice", "/b", "read")
	_, _ = enf.AddPolicy("bob", "/a", "read")

	// Wildcard remove: everything for alice.
	if _, err := enf.RemoveFilteredPolicy(0, "alice"); err != nil {
		t.Fatalf("RemoveFilteredPolicy: %v", err)
	}

	if ok, _ := enf.Enforce("alice", "/a", "read"); ok {
		t.Errorf("alice /a still allowed after filtered remove")
	}

	if ok, _ := enf.Enforce("alice", "/b", "read"); ok {
		t.Errorf("alice /b still allowed after filtered remove")
	}

	// Bob still has his policy.
	if ok, _ := enf.Enforce("bob", "/a", "read"); !ok {
		t.Errorf("bob /a should still be allowed")
	}
}

// -- file adapter ---------------------------------------------------------

func TestFileAdapter_readsAndWrites(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "policy.csv")

	// Seed the file with one policy.
	if err := os.WriteFile(csvPath, []byte("p, alice, /data, read\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	modelBytes, _ := fixtures.ReadFile("testdata/acl_model.conf")

	enf, err := policy.NewFromString(string(modelBytes), policy.NewFileAdapter(csvPath))
	if err != nil {
		t.Fatalf("NewFromString: %v", err)
	}

	if ok, _ := enf.Enforce("alice", "/data", "read"); !ok {
		t.Errorf("seed policy from file not enforced")
	}

	// AddPolicy + SavePolicy persists to the file.
	if _, err := enf.AddPolicy("bob", "/data", "read"); err != nil {
		t.Fatalf("AddPolicy: %v", err)
	}

	if err := enf.SavePolicy(); err != nil {
		t.Fatalf("SavePolicy: %v", err)
	}

	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}

	if !contains(string(data), "bob") {
		t.Errorf("file did not receive bob policy: %s", data)
	}
}

// -- fs adapter (read-only) ----------------------------------------------

func TestFileAdapterFS_readOnly(t *testing.T) {
	adapter := policy.NewFileAdapterFS(fixtures, "testdata/rbac_policy.csv")

	enf, err := policy.NewFromFS(fixtures, "testdata/rbac_model.conf", adapter)
	if err != nil {
		t.Fatalf("NewFromFS: %v", err)
	}

	if ok, _ := enf.Enforce("alice", "/data", "read"); !ok {
		t.Errorf("alice read denied from embedded fs")
	}

	// Writes should fail — this is a build-time immutable policy set.
	if _, err := enf.AddPolicy("mallory", "/data", "read"); err == nil {
		t.Errorf("fs adapter accepted AddPolicy, expected error")
	}
}

func TestFileAdapterFS_missingFileErrorsAtLoad(t *testing.T) {
	adapter := policy.NewFileAdapterFS(fixtures, "testdata/does-not-exist.csv")

	_, err := policy.NewFromFS(fixtures, "testdata/rbac_model.conf", adapter)
	if err == nil {
		t.Errorf("expected error for missing policy file")
	}
}

// tiny helper because I do not want to pull in strings just for this.
func contains(hay, needle string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return true
		}
	}

	return false
}
