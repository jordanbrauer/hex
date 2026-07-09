# TOML for app config, YAML for env var mapping

Application configuration is always TOML — embedded defaults in `config/defaults/app.toml`, user overrides in a TOML file. Environment variable bindings are declared in a separate YAML file (`config/env.yaml`) that maps config keys to env var names (e.g. `database.dsn: MYAPP_DATABASE_DSN`). The YAML file is not application config — it's a binding declaration consumed by Viper's `BindEnv`. One format per concern: TOML for values, YAML for env mappings.
