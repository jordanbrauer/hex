# Security policy

## Supported versions

hex is pre-1.0 (see `go.mod`). There is no long-term-support branch —
security fixes land on `main` and ship in the next tagged release.
Always run the latest tag; older tags do not receive backports.

## Reporting a vulnerability

Please **do not open a public issue** for security reports.

Use [GitHub's private vulnerability reporting](https://github.com/jordanbrauer/hex/security/advisories/new)
(Security tab → "Report a vulnerability"). This opens a private
advisory visible only to the maintainer until a fix is ready.

Include, where relevant:

- The package(s) affected (e.g. `hex/policy`, `hex/lua`, `cmd/hex`)
- A minimal reproduction (code, config, or scaffolded project)
- The potential impact (what an attacker gains)
- Whether the issue originates in hex's own code or in a wrapped
  upstream dependency (see the dependency table in `AGENTS.md`) — if
  it's upstream, please also report it there

## Scope

In scope:

- hex library packages (`hex/*`)
- the `hex` scaffolding CLI (`cmd/hex`), including generated project
  templates
- the vendored Fennel compiler (`lua/fennel/fennel.lua`) — though
  fixes here are typically a version bump; see `lua/fennel/NOTICE.md`

Out of scope:

- Vulnerabilities in wrapped upstream libraries with no hex-specific
  exploitation path — report those to the upstream project directly
- Vulnerabilities in applications *built with* hex that stem from
  application code, not the framework

## Response

There's no formal SLA (small maintainer team), but reports are
triaged as they arrive. A fix ships as a patch release once
confirmed; credit is given in the release notes unless you ask
otherwise.
