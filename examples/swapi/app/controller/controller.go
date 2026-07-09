// Package controller houses HTTP request handlers.
//
// Each controller is a plain struct with methods returning
// echo.HandlerFunc. Wire routes to handler methods in
// app/provider/routes.go, or scaffold both together with
// `hex make:controller <name>`.
package controller
