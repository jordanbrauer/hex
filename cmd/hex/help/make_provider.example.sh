# Generate app/provider/payments.go and wire it into app/boot.go
hex make:provider payments

# Preview what would be written and wired, without touching disk
hex make:provider payments --dry-run

# Machine-readable output for tooling / agents
hex make:provider payments --dry-run --format json
