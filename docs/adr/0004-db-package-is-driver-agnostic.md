# hex/db is driver-agnostic

`hex/db` works against `database/sql` interfaces only. Consumer apps blank-import their own driver (`modernc.org/sqlite`, `github.com/lib/pq`, etc.). This keeps hex's dependency tree small — the SQLite driver alone is ~30MB in the module cache — and makes it trivial to support future drivers without changing hex. Consumer apps embed their own migrations via `embed.FS` and pass them to `hex/db.Open()`.
