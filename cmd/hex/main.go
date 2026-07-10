// Command hex is the scaffolding CLI for hex applications. It is itself
// a hex app: it boots through hex.New(), registers its own providers in
// app/boot.go, and builds its cobra tree in app/command/root.go — the
// same shape `hex init` scaffolds for any consumer app.
//
// Usage:
//
//	hex init [name]              # scaffold a new project
//	hex make provider <name>     # generate a service provider
//	hex make domain <name>       # generate a domain package
//	hex make migration <name>    # generate up/down migration files
//
// Run without arguments to see the full command list.
package main

import (
	"context"
	"os"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/log"

	"github.com/jordanbrauer/hex/cmd/hex/app"
	"github.com/jordanbrauer/hex/cmd/hex/app/command"
)

func main() {
	ctx := context.Background()
	kernel := hex.New()

	if err := app.Bootstrap(ctx, kernel); err != nil {
		log.Fatal("boot", "error", err)
	}

	defer kernel.Shutdown(ctx)

	os.Exit(command.Execute(kernel))
}
