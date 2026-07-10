Generate a domain package at `domain/<name>/` containing the entity
(`<name>.go`), the `Repository` port interface (`repository.go`), the `Service`
use-cases (`service.go`), and sentinel errors (`errors.go`).

The name is normalised to a lower-case package name and a PascalCase type. The
domain package depends on nothing outside itself — infrastructure implements the
`Repository` port via `hex make:adapter`.
