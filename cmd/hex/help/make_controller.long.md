Generate an HTTP controller at `app/controller/<name>.go` and wire its routes
into `app/provider/routes.go` above the `// hex:routes` marker.

By default a single `Index` handler with a `GET /<name>` route is scaffolded.
Use `--all` for full RESTful CRUD (index/show/store/update/destroy) or
`--actions` to pick a comma-separated subset.

Requires `--web` to have been enabled at `hex init` so the Routes provider and
the `app/controller/` package already exist.
