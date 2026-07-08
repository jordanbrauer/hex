package main

import (
	"fmt"
	"os"
	"strings"
)

// addImport inserts importPath into path's import block if not already
// present. Idempotent — no-op when the import exists.
//
// Simple heuristic: locate the first `import (` line and the next `)`;
// insert a tab-indented line above the closing paren. Files that use
// the bare `import "x"` form are extended with the same block anyway
// (Go's grouped-imports formatter fixes the shape on next go fmt).
func addImport(path, importPath string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	src := string(source)

	if strings.Contains(src, `"`+importPath+`"`) {
		return nil // already imported
	}

	openIdx := strings.Index(src, "import (")
	if openIdx < 0 {
		return fmt.Errorf("import block not found in %s", path)
	}

	closeIdx := strings.Index(src[openIdx:], ")")
	if closeIdx < 0 {
		return fmt.Errorf("import block not closed in %s", path)
	}

	closeIdx += openIdx

	// Walk back to the start of the closing paren's line.
	lineStart := strings.LastIndex(src[:closeIdx], "\n")
	if lineStart < 0 {
		lineStart = openIdx
	} else {
		lineStart++
	}

	line := "\t\"" + importPath + "\"\n"
	out := src[:lineStart] + line + src[lineStart:]

	return os.WriteFile(path, []byte(out), 0o644)
}
