# Interactive: prompts for module path, binary name, and components
hex init myapp

# Non-interactive with an embedded SQLite database and web server
hex init myapp --db sqlite --web --yes

# Scaffold into the current directory
hex init . --yes

# A batteries-on service: queue, cron, policy, i18n, flags, telemetry, AI
hex init myapp --web --queue memory --cron --policy --i18n \
  --featureflag --telemetry stdout --ai anthropic --yes

# Skip lint/goreleaser scaffolding; add a GitHub Actions release workflow
hex init myapp --lint=false --goreleaser=false --yes
hex init myapp --github --yes
