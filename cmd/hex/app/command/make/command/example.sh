# A top-level command: app/command/migrate.go, wired into root
hex make command migrate

# A grouped subcommand: app/command/user/create.go under a "user" group
hex make command create --group user

# Preview the files and wiring
hex make command create --group user --dry-run
