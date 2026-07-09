package provider

import (
	"github.com/jordanbrauer/hex/web"
	"github.com/jordanbrauer/hex/web/provider"
)

// Web wires an HTTP server (echo). Register routes inside Configure —
// it runs after Register and before Boot starts the listener.
func Web() *provider.Provider {
	return &provider.Provider{
		Namespace: "server",
		PublicDir: "public",
		Configure: func(s *web.Server) error {
			// TODO: register routes.
			//
			// s.Echo().GET("/hello", func(c echo.Context) error {
			//     return c.String(200, "hi")
			// })
			return nil
		},
	}
}
