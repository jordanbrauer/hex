// Package build re-exports hex/build metadata under this app's namespace.
// Consumers of this codebase read build info from here so they do not
// need to import hex/build directly.
//
// Set at compile time via ldflags:
//
//	go build -ldflags "-X github.com/jordanbrauer/hex/examples/swapi/app/build.version=v1.0.0 \
//	                   -X github.com/jordanbrauer/hex/examples/swapi/app/build.commit=$(git rev-parse HEAD) \
//	                   -X github.com/jordanbrauer/hex/examples/swapi/app/build.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
package build

import hexbuild "github.com/jordanbrauer/hex/build"

// ldflags targets. hex/build reads its own versions from ldflags on
// its own package path; this file re-exports them.
//
//nolint:gochecknoglobals
var (
	version = ""
	commit  = ""
	branch  = ""
	date    = ""
	_       = version
	_       = commit
	_       = branch
	_       = date
)

// AppInfo carries the human-readable app identity.
type AppInfo struct {
	Name    string
	Version string
	Commit  string
	Branch  string
}

// Info returns the current build metadata.
func Info() AppInfo {
	return AppInfo{
		Name:    "swapi",
		Version: hexbuild.Version(),
		Commit:  hexbuild.Commit(),
		Branch:  hexbuild.Branch(),
	}
}
