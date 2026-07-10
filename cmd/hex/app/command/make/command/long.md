Generate a Cobra command wired into the application.

Without `--group`, the command lands at `app/command/<name>.go` and is registered
against the root's `// hex:commands` marker.

With `--group`, the command lands at `app/command/<group>/<name>.go` and is
registered against the group's `// hex:commands:<group>` marker. The group's
`root.go` is generated automatically the first time and wired into the top-level
command; existing group roots are never overwritten, preserving prior
registrations.
