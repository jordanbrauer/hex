package disk_test

import (
	"errors"
	"testing"

	"github.com/jordanbrauer/hex/disk"
)

func TestCleanPath_valid(t *testing.T) {
	tests := map[string]string{
		"foo":                     "foo",
		"foo/bar":                 "foo/bar",
		"foo/bar.txt":             "foo/bar.txt",
		"foo//bar":                "foo/bar",
		"foo/./bar":               "foo/bar",
		`foo\bar`:                 "foo/bar",
		"deeply/nested/thing.txt": "deeply/nested/thing.txt",
	}

	for input, want := range tests {
		got, err := disk.CleanPath(input)
		if err != nil {
			t.Errorf("CleanPath(%q) error = %v", input, err)

			continue
		}

		if got != want {
			t.Errorf("CleanPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCleanPath_rejected(t *testing.T) {
	tests := []string{
		"",
		"/absolute",
		"/etc/passwd",
		"../escape",
		"foo/../bar",
		"foo/..",
		"..",
	}

	for _, input := range tests {
		if _, err := disk.CleanPath(input); !errors.Is(err, disk.ErrInvalidPath) {
			t.Errorf("CleanPath(%q) error = %v, want ErrInvalidPath", input, err)
		}
	}
}
