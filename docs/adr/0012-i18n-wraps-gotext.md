# hex/i18n wraps gotext with a multi-locale Translator

`hex/i18n` wraps github.com/leonelquinteros/gotext. Gotext speaks GNU gettext's PO/MO format — the same format `xgettext`, Weblate, Crowdin, POEditor, and every established translation toolchain already produce. Rolling our own format would trade compatibility for zero gain.

The wrapper follows the pattern established for cron/log/web/lua/pool/policy/casbin: type alias for `gotext.Locale` plus hex-owned constructors that accept a path or an `fs.FS` (for `//go:embed`), all rooted at the standard gettext directory layout (`<lang>/LC_MESSAGES/<domain>.po`).

On top of that, hex/i18n adds a small `Translator` type that holds multiple locales and selects one per call — the shape most applications need but gotext leaves the caller to assemble. A package-level default (`SetDefault` / `T` / `TN` / `TC`) mirrors `hex/config`.

## Test fixtures are files, not string literals

Same rule as hex/policy: PO files live under `testdata/` and are embedded via `//go:embed`. PO structure (msgctxt / msgid / msgid_plural / msgstr[n] / header) does not survive inline string literals well, and the whole point of using a standard format is that translators can edit real files.

## No auto-detection

hex/i18n does not sniff `Accept-Language` headers, `LANG` env vars, or Cookie preferences. Locale selection is the consumer's job — a middleware, a CLI flag, or a user profile setting — because different apps disambiguate differently. The `Translator` type accepts a language code from wherever the consumer decides.
