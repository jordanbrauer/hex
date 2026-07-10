// Package build re-exports hex/build metadata under the hex CLI's own
// namespace, the same way any scaffolded hex app does.
package build

import hexbuild "github.com/jordanbrauer/hex/build"

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
		Name:    "hex",
		Version: hexbuild.Version(),
		Commit:  hexbuild.Commit(),
		Branch:  hexbuild.Branch(),
	}
}
