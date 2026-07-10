module github.com/jordanbrauer/hex/examples/bare-hex-app

go 1.26.5

require (
	github.com/jordanbrauer/hex v0.0.0
	github.com/spf13/cobra v1.10.2
)

replace github.com/jordanbrauer/hex => ../..
