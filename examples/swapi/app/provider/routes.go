package provider

import (
	"context"
	"database/sql"

	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/provider"
	"github.com/jordanbrauer/hex/web"

	"github.com/jordanbrauer/hex/examples/swapi/app/controller"
	"github.com/jordanbrauer/hex/examples/swapi/infrastructure"
)

// Routes wires application HTTP routes onto the *web.Server registered
// by the web provider. hex make controller inserts route
// registrations above the `// hex:routes` marker below. Do not
// remove the marker.
//
// Route registration lives in Boot (not Register) because it needs the
// *sql.DB which the db provider only opens during Boot. Echo's mux is
// dynamic, so routes registered after web/provider.Boot has started
// the listener are still served correctly.
type Routes struct {
	provider.Base
}

// Boot attaches routes to the *web.Server.
func (p *Routes) Boot(_ context.Context, app provider.Application) error {
	server, err := container.Make[*web.Server](app, "http")
	if err != nil {
		return err
	}

	db, err := container.Make[*sql.DB](app, "db")
	if err != nil {
		return err
	}

	res := controller.NewResources(infrastructure.NewRepo(db))
	e := server.Echo()

	e.GET("/", res.Home)
	e.GET("/films", res.Films)
	e.GET("/films/:id", res.Film)
	e.GET("/people", res.People)
	e.GET("/people/:id", res.Person)
	e.GET("/planets", res.Planets)
	e.GET("/species", res.Species)
	e.GET("/starships", res.Starships)
	e.GET("/vehicles", res.Vehicles)

	// hex:routes

	return nil
}
