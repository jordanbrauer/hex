package local_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jordanbrauer/hex/disk"
	"github.com/jordanbrauer/hex/disk/local"
)

func newDisk(t *testing.T) (*local.Local, string) {
	t.Helper()

	root := t.TempDir()

	d, err := local.New(local.Options{Root: root})
	if err != nil {
		t.Fatalf("local.New: %v", err)
	}

	return d, root
}

func TestNew_requiresRoot(t *testing.T) {
	if _, err := local.New(local.Options{}); err == nil {
		t.Errorf("New with empty Root returned nil error")
	}
}

func TestNew_createsRootDir(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "does", "not", "exist", "yet")

	d, err := local.New(local.Options{Root: root})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := os.Stat(d.Root()); err != nil {
		t.Errorf("root not created: %v", err)
	}
}

func TestPutGet_roundTrip(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	if err := d.Put(ctx, "hello.txt", []byte("world")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := d.Get(ctx, "hello.txt")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if string(got) != "world" {
		t.Errorf("Get = %q, want world", got)
	}
}

func TestPut_createsParents(t *testing.T) {
	d, root := newDisk(t)

	if err := d.Put(context.Background(), "deep/nested/dir/file.txt", []byte("x")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "deep", "nested", "dir", "file.txt")); err != nil {
		t.Errorf("file not created under parents: %v", err)
	}
}

func TestPut_overwrites(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	_ = d.Put(ctx, "f", []byte("v1"))
	_ = d.Put(ctx, "f", []byte("v2"))

	got, _ := d.Get(ctx, "f")
	if string(got) != "v2" {
		t.Errorf("Get after overwrite = %q, want v2", got)
	}
}

func TestGet_missingReturnsNotFound(t *testing.T) {
	d, _ := newDisk(t)

	_, err := d.Get(context.Background(), "nope.txt")
	if !errors.Is(err, disk.ErrNotFound) {
		t.Errorf("Get missing error = %v, want ErrNotFound", err)
	}
}

func TestExists(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	ok, _ := d.Exists(ctx, "no")
	if ok {
		t.Errorf("Exists(missing) = true")
	}

	_ = d.Put(ctx, "yes", []byte{})

	ok, _ = d.Exists(ctx, "yes")
	if !ok {
		t.Errorf("Exists(present) = false")
	}
}

func TestReader_streams(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	_ = d.Put(ctx, "file", []byte("streamed"))

	r, err := d.Reader(ctx, "file")
	if err != nil {
		t.Fatalf("Reader: %v", err)
	}

	defer r.Close()

	data, _ := io.ReadAll(r)
	if string(data) != "streamed" {
		t.Errorf("read = %q, want streamed", data)
	}
}

func TestWriter_streams(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	w, err := d.Writer(ctx, "streamed.txt")
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	if _, err := io.WriteString(w, "hello"); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	got, _ := d.Get(ctx, "streamed.txt")
	if string(got) != "hello" {
		t.Errorf("streamed content = %q, want hello", got)
	}
}

func TestSize(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	_ = d.Put(ctx, "f", []byte("12345"))

	n, err := d.Size(ctx, "f")
	if err != nil {
		t.Fatalf("Size: %v", err)
	}

	if n != 5 {
		t.Errorf("Size = %d, want 5", n)
	}
}

func TestLastModified(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	before := time.Now()

	_ = d.Put(ctx, "f", []byte{})

	mt, err := d.LastModified(ctx, "f")
	if err != nil {
		t.Fatalf("LastModified: %v", err)
	}

	if mt.Before(before.Add(-2 * time.Second)) {
		t.Errorf("LastModified = %v, want >= %v", mt, before)
	}
}

func TestDelete(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	_ = d.Put(ctx, "f", []byte("x"))

	if err := d.Delete(ctx, "f"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if ok, _ := d.Exists(ctx, "f"); ok {
		t.Errorf("Delete did not remove file")
	}

	// Missing file is not an error.
	if err := d.Delete(ctx, "gone"); err != nil {
		t.Errorf("Delete(missing) = %v, want nil", err)
	}
}

func TestCopy(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	_ = d.Put(ctx, "src.txt", []byte("data"))

	if err := d.Copy(ctx, "src.txt", "dst/copy.txt"); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	got, _ := d.Get(ctx, "dst/copy.txt")
	if string(got) != "data" {
		t.Errorf("copy contents = %q, want data", got)
	}

	// Source still present.
	if ok, _ := d.Exists(ctx, "src.txt"); !ok {
		t.Errorf("Copy removed source")
	}
}

func TestMove(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	_ = d.Put(ctx, "src.txt", []byte("data"))

	if err := d.Move(ctx, "src.txt", "moved/dst.txt"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	if ok, _ := d.Exists(ctx, "src.txt"); ok {
		t.Errorf("Move did not remove source")
	}

	got, _ := d.Get(ctx, "moved/dst.txt")
	if string(got) != "data" {
		t.Errorf("moved contents = %q, want data", got)
	}
}

func TestList(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	_ = d.Put(ctx, "a.txt", []byte{})
	_ = d.Put(ctx, "b.txt", []byte{})
	_ = d.MakeDirectory(ctx, "sub")

	got, err := d.List(ctx, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	sort.Strings(got)

	want := []string{"a.txt", "b.txt", "sub"}

	if len(got) != len(want) {
		t.Fatalf("List = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("List[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestList_subDirPrefixesEntries(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	_ = d.Put(ctx, "sub/x.txt", []byte{})
	_ = d.Put(ctx, "sub/y.txt", []byte{})

	got, err := d.List(ctx, "sub")
	if err != nil {
		t.Fatalf("List sub: %v", err)
	}

	for _, entry := range got {
		if !strings.HasPrefix(entry, "sub/") {
			t.Errorf("List entry %q missing 'sub/' prefix", entry)
		}
	}
}

func TestMakeDirectory_idempotent(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	if err := d.MakeDirectory(ctx, "a/b/c"); err != nil {
		t.Fatalf("MakeDirectory: %v", err)
	}

	if err := d.MakeDirectory(ctx, "a/b/c"); err != nil {
		t.Errorf("MakeDirectory (twice) error = %v", err)
	}
}

func TestDeleteDirectory(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	_ = d.Put(ctx, "tree/a.txt", []byte{})
	_ = d.Put(ctx, "tree/sub/b.txt", []byte{})

	if err := d.DeleteDirectory(ctx, "tree"); err != nil {
		t.Fatalf("DeleteDirectory: %v", err)
	}

	if ok, _ := d.Exists(ctx, "tree"); ok {
		t.Errorf("DeleteDirectory did not remove tree")
	}
}

func TestPathTraversal_rejected(t *testing.T) {
	d, _ := newDisk(t)
	ctx := context.Background()

	for _, bad := range []string{"../escape", "foo/../../..", "/absolute"} {
		_, err := d.Get(ctx, bad)
		if !errors.Is(err, disk.ErrInvalidPath) {
			t.Errorf("Get(%q) error = %v, want ErrInvalidPath", bad, err)
		}
	}
}

func TestURL_noPrefixReturnsErrNotSupported(t *testing.T) {
	d, _ := newDisk(t)

	_, err := d.URL(context.Background(), "f.txt")
	if !errors.Is(err, disk.ErrNotSupported) {
		t.Errorf("URL() with no prefix = %v, want ErrNotSupported", err)
	}
}

func TestURL_withPrefix(t *testing.T) {
	root := t.TempDir()

	d, err := local.New(local.Options{
		Root:      root,
		URLPrefix: "https://cdn.example.com/uploads/",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := d.URL(context.Background(), "img/logo.png")
	if err != nil {
		t.Fatalf("URL: %v", err)
	}

	want := "https://cdn.example.com/uploads/img/logo.png"
	if got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
}

func TestVisibility_defaultPrivate(t *testing.T) {
	// FileMode 0o600 default via override, so world-read bit is off.
	root := t.TempDir()

	d, err := local.New(local.Options{Root: root, FileMode: 0o600})
	if err != nil {
		t.Fatal(err)
	}

	_ = d.Put(context.Background(), "f", []byte("x"))

	v, err := d.Visibility(context.Background(), "f")
	if err != nil {
		t.Fatalf("Visibility: %v", err)
	}

	if v != disk.VisibilityPrivate {
		t.Errorf("Visibility = %v, want private", v)
	}
}

func TestVisibility_setToggles(t *testing.T) {
	root := t.TempDir()

	d, err := local.New(local.Options{
		Root:           root,
		FileMode:       0o600,
		PublicFileMode: 0o644,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	_ = d.Put(ctx, "f", []byte("x"))

	if err := d.SetVisibility(ctx, "f", disk.VisibilityPublic); err != nil {
		t.Fatalf("SetVisibility public: %v", err)
	}

	v, _ := d.Visibility(ctx, "f")
	if v != disk.VisibilityPublic {
		t.Errorf("Visibility after public = %v, want public", v)
	}

	if err := d.SetVisibility(ctx, "f", disk.VisibilityPrivate); err != nil {
		t.Fatalf("SetVisibility private: %v", err)
	}

	v, _ = d.Visibility(ctx, "f")
	if v != disk.VisibilityPrivate {
		t.Errorf("Visibility after private = %v, want private", v)
	}
}
