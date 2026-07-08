// Package local implements the disk.Disk interface against the host
// filesystem. It is the default and only backend shipped in v1 (ADR-0008).
//
// A local disk is anchored at a Root directory. All disk paths resolve
// relative to Root; the disk cannot see or touch anything outside it. Path
// traversal (".", "..") is rejected by hex/disk before it reaches this
// driver.
//
// URLs: if Options.URLPrefix is set, URL() returns URLPrefix + "/" + path.
// If not set, URL returns disk.ErrNotSupported.
//
// Visibility: files are created with FileMode; SetVisibility toggles between
// FileMode (private) and PublicFileMode (public).
package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jordanbrauer/hex/disk"
)

// Options configures a Local disk.
type Options struct {
	// Root is the absolute directory this disk is anchored to. Required.
	Root string

	// URLPrefix is prepended to paths when URL() is called. Optional.
	URLPrefix string

	// FileMode is the mode used for created files. Defaults to 0o644.
	FileMode os.FileMode

	// DirMode is the mode used for created directories. Defaults to 0o755.
	DirMode os.FileMode

	// PublicFileMode is used by SetVisibility(VisibilityPublic). Defaults
	// to 0o644. Set higher (e.g. 0o664) if your consumers need a wider
	// public bit.
	PublicFileMode os.FileMode
}

// Local is a filesystem-backed disk.Disk.
type Local struct {
	opts Options
}

// New returns a Local disk rooted at opts.Root. The root directory is
// created if it does not exist. Returns an error if Root is empty or
// creation fails.
func New(opts Options) (*Local, error) {
	if opts.Root == "" {
		return nil, errors.New("disk/local: Options.Root is required")
	}

	if opts.FileMode == 0 {
		opts.FileMode = 0o644
	}

	if opts.DirMode == 0 {
		opts.DirMode = 0o755
	}

	if opts.PublicFileMode == 0 {
		opts.PublicFileMode = 0o644
	}

	abs, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("disk/local: resolve root: %w", err)
	}

	if err := os.MkdirAll(abs, opts.DirMode); err != nil {
		return nil, fmt.Errorf("disk/local: create root: %w", err)
	}

	opts.Root = abs

	return &Local{opts: opts}, nil
}

// Root returns the absolute root directory this disk is anchored to.
func (l *Local) Root() string { return l.opts.Root }

// -- helpers ---------------------------------------------------------------

// resolve joins the disk root with a validated relative path. It re-checks
// the result is still under Root as a defense against exotic inputs the
// package-level cleanPath might have missed.
func (l *Local) resolve(p string) (string, error) {
	clean, err := disk.CleanPath(p)
	if err != nil {
		return "", err
	}

	joined := filepath.Join(l.opts.Root, filepath.FromSlash(clean))

	rel, err := filepath.Rel(l.opts.Root, joined)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", disk.ErrInvalidPath
	}

	return joined, nil
}

// translateErr converts filesystem errors into hex/disk sentinels where
// applicable.
func translateErr(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return disk.ErrNotFound
	}

	return err
}

// -- read ------------------------------------------------------------------

// Get reads the file into memory.
func (l *Local) Get(_ context.Context, p string) ([]byte, error) {
	full, err := l.resolve(p)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(full)

	return data, translateErr(err)
}

// Reader streams the file. Caller must Close.
func (l *Local) Reader(_ context.Context, p string) (io.ReadCloser, error) {
	full, err := l.resolve(p)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(full)

	return f, translateErr(err)
}

// Exists reports whether the path exists.
func (l *Local) Exists(_ context.Context, p string) (bool, error) {
	full, err := l.resolve(p)
	if err != nil {
		return false, err
	}

	if _, err := os.Stat(full); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// Size returns the file size in bytes.
func (l *Local) Size(_ context.Context, p string) (int64, error) {
	full, err := l.resolve(p)
	if err != nil {
		return 0, err
	}

	info, err := os.Stat(full)
	if err != nil {
		return 0, translateErr(err)
	}

	if info.IsDir() {
		return 0, fmt.Errorf("disk/local: %q is a directory", p)
	}

	return info.Size(), nil
}

// LastModified returns the modification time.
func (l *Local) LastModified(_ context.Context, p string) (time.Time, error) {
	full, err := l.resolve(p)
	if err != nil {
		return time.Time{}, err
	}

	info, err := os.Stat(full)
	if err != nil {
		return time.Time{}, translateErr(err)
	}

	return info.ModTime(), nil
}

// -- write -----------------------------------------------------------------

// Put writes content to path, creating parents as needed.
func (l *Local) Put(_ context.Context, p string, content []byte) error {
	full, err := l.resolve(p)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(full), l.opts.DirMode); err != nil {
		return fmt.Errorf("disk/local: mkdir parents: %w", err)
	}

	return os.WriteFile(full, content, l.opts.FileMode)
}

// Writer opens a streaming write. Close truncates any existing file.
func (l *Local) Writer(_ context.Context, p string) (io.WriteCloser, error) {
	full, err := l.resolve(p)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(full), l.opts.DirMode); err != nil {
		return nil, fmt.Errorf("disk/local: mkdir parents: %w", err)
	}

	f, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, l.opts.FileMode)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// Delete removes a file. Missing files are not an error.
func (l *Local) Delete(_ context.Context, p string) error {
	full, err := l.resolve(p)
	if err != nil {
		return err
	}

	err = os.Remove(full)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

// Copy duplicates src to dst, overwriting dst.
func (l *Local) Copy(ctx context.Context, src, dst string) error {
	in, err := l.Reader(ctx, src)
	if err != nil {
		return err
	}

	defer in.Close()

	out, err := l.Writer(ctx, dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()

		return err
	}

	return out.Close()
}

// Move renames src to dst.
func (l *Local) Move(_ context.Context, src, dst string) error {
	from, err := l.resolve(src)
	if err != nil {
		return err
	}

	to, err := l.resolve(dst)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(to), l.opts.DirMode); err != nil {
		return fmt.Errorf("disk/local: mkdir parents: %w", err)
	}

	if err := os.Rename(from, to); err != nil {
		return translateErr(err)
	}

	return nil
}

// -- directories -----------------------------------------------------------

// List returns immediate children of prefix.
func (l *Local) List(_ context.Context, prefix string) ([]string, error) {
	full := l.opts.Root
	base := ""

	if prefix != "" && prefix != "." {
		resolved, err := l.resolve(prefix)
		if err != nil {
			return nil, err
		}

		full = resolved
		base = strings.ReplaceAll(prefix, "\\", "/")
	}

	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, translateErr(err)
	}

	out := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if base != "" {
			name = strings.TrimRight(base, "/") + "/" + name
		}

		out = append(out, name)
	}

	return out, nil
}

// MakeDirectory creates a directory and any missing parents.
func (l *Local) MakeDirectory(_ context.Context, p string) error {
	full, err := l.resolve(p)
	if err != nil {
		return err
	}

	return os.MkdirAll(full, l.opts.DirMode)
}

// DeleteDirectory removes a directory tree.
func (l *Local) DeleteDirectory(_ context.Context, p string) error {
	full, err := l.resolve(p)
	if err != nil {
		return err
	}

	err = os.RemoveAll(full)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

// -- addressing ------------------------------------------------------------

// URL returns URLPrefix + "/" + path, or ErrNotSupported if no prefix.
func (l *Local) URL(_ context.Context, p string) (string, error) {
	clean, err := disk.CleanPath(p)
	if err != nil {
		return "", err
	}

	if l.opts.URLPrefix == "" {
		return "", disk.ErrNotSupported
	}

	return strings.TrimRight(l.opts.URLPrefix, "/") + "/" + clean, nil
}

// -- visibility ------------------------------------------------------------

// Visibility reads the file mode and reports public/private based on the
// world-readable bit.
func (l *Local) Visibility(_ context.Context, p string) (disk.Visibility, error) {
	full, err := l.resolve(p)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(full)
	if err != nil {
		return "", translateErr(err)
	}

	if info.Mode().Perm()&0o004 != 0 {
		return disk.VisibilityPublic, nil
	}

	return disk.VisibilityPrivate, nil
}

// SetVisibility toggles between FileMode (private) and PublicFileMode (public).
func (l *Local) SetVisibility(_ context.Context, p string, v disk.Visibility) error {
	full, err := l.resolve(p)
	if err != nil {
		return err
	}

	mode := l.opts.FileMode
	if v == disk.VisibilityPublic {
		mode = l.opts.PublicFileMode
	}

	return translateErr(os.Chmod(full, mode))
}

// compile-time proof
var (
	_ disk.Disk        = (*Local)(nil)
	_ disk.Visibilizer = (*Local)(nil)
)
