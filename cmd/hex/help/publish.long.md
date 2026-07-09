Copy the config files that a hex framework provider ships (via its embedded
`Configs()` fs.FS) into your project's `config/` directory so you can inspect and
edit them. Files are copied as-is; the framework's original defaults still apply
as a fallback when your local copy is missing a field.

Pass `--all` to publish every framework component at once. Pass `--force` to
overwrite files you have already published.
