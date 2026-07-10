package main

import (
	"context"
	"os"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/log"

	"github.com/jordanbrauer/hex/examples/swapi/app"
	"github.com/jordanbrauer/hex/examples/swapi/app/command"
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
