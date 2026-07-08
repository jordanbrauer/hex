// Package disk defines a driver-agnostic filesystem abstraction inspired by
// Laravel's Storage facade.
//
// A Disk is a named backend that can read, write, list, and manage files
// under some root. The core interface is portable across local filesystems,
// S3, MinIO, GCS, etc. Consumers resolve disks from the hex container by
// name (e.g. "disk.uploads", "disk.public") so swapping the backend is a
// configuration change.
//
// v1 ships hex/disk/local only. Cloud drivers (hex/disk/s3, hex/disk/minio,
// hex/disk/gcs) come later as opt-in subpackages implementing this
// interface, following the same pattern as hex/db and hex/cache. See
// ADR-0008.
//
// Path semantics:
//
//   - Paths are always forward-slash-separated, regardless of host OS.
//     The local driver translates to filepath.Separator internally.
//   - Paths do not start with "/". They are relative to the disk's root.
//   - "." and ".." segments are rejected as invalid — no path traversal.
//
// Not-found semantics: every read-shaped method returns ErrNotFound when
// the path does not exist. Consumers use errors.Is(err, disk.ErrNotFound)
// to distinguish miss from other failures.
package disk

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound is returned by read-shaped operations when the target path
// does not exist on the disk.
var ErrNotFound = errors.New("disk: not found")

// ErrInvalidPath is returned when a path is empty, absolute, or contains
// traversal segments (".", "..").
var ErrInvalidPath = errors.New("disk: invalid path")

// Disk is the driver-facing interface. Implementations must be safe for
// concurrent use.
//
//nolint:interfacebloat // one big interface matches the Storage-facade mental model
type Disk interface {
	// -- read ---------------------------------------------------------------

	// Get reads the entire file into memory. Prefer Reader for large files.
	Get(ctx context.Context, path string) ([]byte, error)

	// Reader opens the file for streaming reads. The caller must Close it.
	Reader(ctx context.Context, path string) (io.ReadCloser, error)

	// Exists reports whether path resolves to a file or directory.
	Exists(ctx context.Context, path string) (bool, error)

	// Size returns the byte length of the file at path.
	Size(ctx context.Context, path string) (int64, error)

	// LastModified returns the modification time of the file at path.
	LastModified(ctx context.Context, path string) (time.Time, error)

	// -- write --------------------------------------------------------------

	// Put writes content to path, replacing any existing file. Missing parent
	// directories are created automatically.
	Put(ctx context.Context, path string, content []byte) error

	// Writer opens path for streaming writes. Any existing file is truncated
	// when Close is called. Missing parent directories are created.
	Writer(ctx context.Context, path string) (io.WriteCloser, error)

	// Delete removes the file at path. Deleting a missing file is not an
	// error.
	Delete(ctx context.Context, path string) error

	// Copy duplicates src to dst. dst is overwritten if it exists.
	Copy(ctx context.Context, src, dst string) error

	// Move renames src to dst. dst is overwritten if it exists.
	Move(ctx context.Context, src, dst string) error

	// -- directories --------------------------------------------------------

	// List returns the immediate children of the directory at prefix (not
	// recursive). Prefix "" or "." means the disk root. Returned paths are
	// relative to the disk root, forward-slash-separated.
	List(ctx context.Context, prefix string) ([]string, error)

	// MakeDirectory creates path (and any missing parents). Creating an
	// existing directory is not an error.
	MakeDirectory(ctx context.Context, path string) error

	// DeleteDirectory removes path and all its contents.
	DeleteDirectory(ctx context.Context, path string) error

	// -- addressing ---------------------------------------------------------

	// URL returns a publicly reachable URL for path. Local disks may return
	// a file:// URL or a URL prefix + path if the disk was configured with
	// one; cloud disks return whatever their backend exposes as a public
	// URL. Returns ErrNotSupported if the driver has no URL scheme.
	URL(ctx context.Context, path string) (string, error)
}

// ErrNotSupported is returned when a Disk method is called against a driver
// that cannot fulfil the operation (e.g. URL on a driver with no URL
// scheme configured).
var ErrNotSupported = errors.New("disk: operation not supported by this driver")

// TempURLer is an optional interface for drivers that support signed/
// expiring URLs (S3, GCS). Local disks generally do not.
type TempURLer interface {
	TempURL(ctx context.Context, path string, expiry time.Duration) (string, error)
}

// Visibility describes whether a file is public or private on backends that
// distinguish (S3 ACLs, GCS object ACLs). Local backends may map these to
// filesystem permissions.
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

// Visibilizer is an optional interface for drivers that expose per-file
// visibility. Callers must type-check the Disk before invoking.
type Visibilizer interface {
	Visibility(ctx context.Context, path string) (Visibility, error)
	SetVisibility(ctx context.Context, path string, v Visibility) error
}
