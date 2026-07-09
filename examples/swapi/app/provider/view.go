package provider

import (
	viewprovider "github.com/jordanbrauer/hex/view/provider"

	"github.com/jordanbrauer/hex/examples/swapi/web/views"
)

// View wires the hex/view Engine into the *web.Server registered by
// the web provider.
func View() *viewprovider.Provider {
	return &viewprovider.Provider{
		FS: views.Files,
	}
}
