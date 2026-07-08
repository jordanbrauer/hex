package disk

import (
	"path"
	"strings"
)

// CleanPath validates and normalises a disk path.
//
// Exported so driver subpackages (hex/disk/local, future hex/disk/s3) can
// share the same path rules without re-implementing validation.
func CleanPath(p string) (string, error) { return cleanPath(p) }

// cleanPath validates and normalises a disk path.
//
//   - Empty strings, absolute paths, and paths containing "." or ".."
//     segments are rejected (ErrInvalidPath).
//   - Backslashes are converted to forward slashes so Windows callers can
//     use either separator.
//   - Multiple consecutive slashes collapse to one.
//
// Returned paths are always forward-slash-separated and relative.
func cleanPath(p string) (string, error) {
	if p == "" {
		return "", ErrInvalidPath
	}

	p = strings.ReplaceAll(p, "\\", "/")

	if strings.HasPrefix(p, "/") {
		return "", ErrInvalidPath
	}

	// path.Clean is nearly what we want but does not reject "..". Do that
	// ourselves before cleaning.
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return "", ErrInvalidPath
		}
	}

	clean := path.Clean(p)

	// path.Clean("") == "." — we already rejected empty, so a "." result
	// means the caller passed something equivalent to root, which is
	// meaningful only for List/MakeDirectory contexts.
	return clean, nil
}
