// Whole-tree schema. Top-level fields describe each namespace.
// A per-namespace <ns>.cue file unifies with the corresponding field here.

server: {
	address: string & !=""
	cors?:   bool
}
