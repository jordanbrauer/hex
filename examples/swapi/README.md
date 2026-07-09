# swapi — a Star Wars API demo built on hex

A tiny read-only web app rendering the classic
[SWAPI](https://swapi.dev) dataset (6 films, 82 people, 60 planets,
37 species, 36 starships, 39 vehicles) served straight out of a
316 KB SQLite file.

Every layer is stock hex — nothing custom, nothing hand-wired that a
`hex init` project wouldn't already have:

- `hex init swapi --db sqlite --web --frontend ts` scaffolded the shell
- `hex/db + hex/db/sqlite` — connects to the pre-populated `testdata/swapi.db`
- `hex/web` — Echo-backed HTTP server
- `hex/view` — Go templates with a shared head/foot layout
- `hex/webtest` — the smoke test in `main_test.go` boots the whole app
  into an `httptest.Server` and drives every route

## Run

```
cd examples/swapi
go run . serve
```

Then browse to <http://localhost:8080>.

## Available commands

Straight from the scaffold — no extra work:

```
./swapi serve       # start the HTTP server
./swapi repl        # container-aware Teal/Lua/Fennel REPL
./swapi run script  # execute a script with all providers loaded
./swapi version
```

## Routes

| Path | What it renders |
|---|---|
| `/` | Landing with counts per resource |
| `/films` | Chronological list of the six films |
| `/films/:id` | Title, director, release date, cast, opening crawl |
| `/people` | All 82 characters |
| `/people/:id` | Attributes + homeworld + films appeared in |
| `/planets`, `/species`, `/starships`, `/vehicles` | Simple listings |

## Tests

```
go test .
```

The `main_test.go` boots the same providers as production, wraps
Echo in an `httptest.Server`, and asserts on rendered HTML with
CSS selectors — no fake HTTP, no mocked DB, no test-specific wiring.

## Layout

```
app/           service providers, controllers, cobra commands
config/        TOML config for each provider
database/      empty migration (data comes pre-loaded in swapi.db)
domain/        Film, Person, Planet, Species, Starship, Vehicle
infrastructure/  SQL-backed read model (Repo)
testdata/      swapi.db
web/views/     Go html/templates (layouts/ + pages/)
```

## Data source

`testdata/swapi.db` is a SQLite dump of the classic
[SWAPI Django app](https://github.com/phalt/swapi) — same schema,
same `resources_*` table names, no transformations.
