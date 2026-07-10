Generate an infrastructure adapter at
`infrastructure/<dialect>/<domain>_repository.go` — a stub implementation of
`domain/<domain>.Repository` backed by the given SQL dialect.

The generator produces `panic("not implemented")` stubs for the standard
`Repository` methods (Store, Get, List, Delete) that `hex make domain` scaffolds,
plus a compile-time `var _ <domain>.Repository = (*…)(nil)` assertion. If you
have extended the interface, add the extra methods by hand.
