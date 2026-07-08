# hex ships as a single Go module

The hex library packages and `cmd/hex` scaffolding CLI live in one Go module (`github.com/jordanbrauer/hex`). `go install github.com/jordanbrauer/hex/cmd/hex@latest` pulls library deps at build time but they don't bloat the binary — Go only links what's imported. Splitting into two modules (library + CLI) was considered but rejected because coordinating releases and cross-module version references adds real complexity for a marginal benefit that doesn't matter until hex has external users beyond the two Finch apps.
