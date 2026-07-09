Generate a service provider at `app/provider/<name>.go` and wire it into
`app/boot.go` above the `// hex:providers` marker.

The name is normalised to PascalCase for the type and lower-case snake_case for
the filename. The generated provider embeds `provider.Base`; add your bindings in
`Register` and open resources or start goroutines in `Boot`.
