package container

import (
	"runtime"
	"strconv"
	"strings"
)

// goid returns the current goroutine ID. Used only for cycle detection —
// resolving is a per-goroutine concept, so we need a stable per-goroutine
// key. Parsing runtime.Stack is intentional: Go does not expose the ID and
// the alternative (a goroutine-local via context) would require threading
// context through the whole Container API.
func goid() uint64 {
	var buf [64]byte

	n := runtime.Stack(buf[:], false)
	// Format: "goroutine 42 [running]:\n..."
	s := string(buf[:n])
	s = strings.TrimPrefix(s, "goroutine ")

	if idx := strings.IndexByte(s, ' '); idx > 0 {
		s = s[:idx]
	}

	id, _ := strconv.ParseUint(s, 10, 64)

	return id
}
