# hex/lua is a runtime, not a plugin system

`hex/lua` embeds `yuin/gopher-lua` and exposes an `Environment` that compiles and executes Lua scripts. It ships no Go→Lua bindings and no plugin discovery/management convention.

Bindings and plugin conventions stay in consumers: finch-cli has `lib/lua/module/*` with dozens of app-specific modules; a future consumer will want a different set. Baking any specific set into hex either bloats every consumer or forces disagreement over which modules belong.

The plugin system (config manifests, `~/.config/<app>/plugins/`, versioning, hot reload) is finch-cli's product surface, not a framework concern. If a future consumer needs it, we extract that on demand into `hex/lua/plugin` or a separate module — not up front.

We rejected shipping "core modules" (json, http, disk, etc.) because the CLI's versions are already opinionated to fit its patterns and rewriting portable versions is a distraction from the framework core.
