package controller

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
)

// wireControllerRoutes converts the controller/<pkg> blank import in
// routes.go into a non-blank one (if needed), then inserts the
// route-registration lines above the hex:routes marker.
func wireControllerRoutes(svc *generator.Service, routesFile, modulePath string, data controllerData, opts generator.Options) ([]generator.Action, error) {
	var actions []generator.Action

	// Promote blank controller import to a real one, once. Idempotent:
	// no-op after the first controller is scaffolded.
	act, err := svc.PromoteImport(routesFile, modulePath+"/app/controller", opts)
	if err != nil {
		return actions, err
	}

	if act != nil {
		actions = append(actions, *act)
	}

	var b bytes.Buffer

	fmt.Fprintf(&b, "%s := controller.%s()\n", data.Variable, data.Constructor)
	for _, a := range data.Actions {
		fmt.Fprintf(&b, "\te.%s(\"%s%s\", %s.%s)\n",
			methodCall(a.Verb), data.Path, a.Suffix, data.Variable, a.Method)
	}

	wireAct, err := svc.WireMarker(routesFile, "// hex:routes", b.String(), "added "+data.Struct, opts)
	if err != nil {
		return actions, fmt.Errorf("wire routes into %s: %w", routesFile, err)
	}

	if wireAct != nil {
		actions = append(actions, *wireAct)
	}

	return actions, nil
}

// methodCall maps an HTTP verb to echo.Echo's method name. GET → "GET",
// POST → "POST", etc. — they happen to match one-for-one, but keeping
// this helper isolates any future divergence.
func methodCall(verb string) string { return strings.ToUpper(verb) }
