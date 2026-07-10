package generator

import (
	"strings"
	"unicode"
)

// PascalCase converts foo-bar_baz -> FooBarBaz.
func PascalCase(s string) string {
	if s == "" {
		return s
	}

	fields := splitIdentifier(s)
	for i, f := range fields {
		fields[i] = strings.ToUpper(f[:1]) + strings.ToLower(f[1:])
	}

	return strings.Join(fields, "")
}

// CamelCase converts foo-bar_baz -> fooBarBaz.
func CamelCase(s string) string {
	if s == "" {
		return s
	}

	p := PascalCase(s)

	return strings.ToLower(p[:1]) + p[1:]
}

// SnakeCase converts fooBar / foo-bar -> foo_bar.
func SnakeCase(s string) string {
	var b strings.Builder

	for i, r := range s {
		switch {
		case r == '-' || r == ' ':
			b.WriteByte('_')
		case unicode.IsUpper(r):
			if i > 0 {
				b.WriteByte('_')
			}

			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

// TitleCase upper-cases the first letter and leaves the rest alone.
func TitleCase(s string) string {
	if s == "" {
		return s
	}

	return strings.ToUpper(s[:1]) + s[1:]
}

// GoPackageName produces a valid Go package identifier from an arbitrary
// name — lower-cased, non-alphanumeric characters stripped.
func GoPackageName(s string) string {
	var b strings.Builder

	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}

	return b.String()
}

// Pluralise applies a very small English pluralisation table. Good enough
// for the domain generator's default naming; consumers with irregular
// nouns pass an explicit table name to the migration generator.
func Pluralise(s string) string {
	if s == "" {
		return s
	}

	lower := strings.ToLower(s)

	switch {
	case strings.HasSuffix(lower, "s"),
		strings.HasSuffix(lower, "sh"),
		strings.HasSuffix(lower, "ch"),
		strings.HasSuffix(lower, "x"),
		strings.HasSuffix(lower, "z"):
		return s + "es"
	case strings.HasSuffix(lower, "y") && len(s) > 1 && !isVowel(lower[len(lower)-2]):
		return s[:len(s)-1] + "ies"
	default:
		return s + "s"
	}
}

func isVowel(b byte) bool {
	switch b {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	default:
		return false
	}
}

// splitIdentifier breaks a mixed-style identifier into lowercase word
// segments — used by PascalCase / CamelCase.
func splitIdentifier(s string) []string {
	var (
		out   []string
		start = 0
	)

	// First split on separators.
	for i, r := range s {
		if r == '-' || r == '_' || r == ' ' {
			if i > start {
				out = append(out, s[start:i])
			}

			start = i + 1
		}
	}

	if start < len(s) {
		out = append(out, s[start:])
	}

	// Then further split any camelCase segments.
	var final []string

	for _, seg := range out {
		final = append(final, splitCamel(seg)...)
	}

	// Drop empties.
	filtered := final[:0]
	for _, s := range final {
		if s != "" {
			filtered = append(filtered, s)
		}
	}

	return filtered
}

// splitCamel breaks `fooBarBaz` into `foo`, `Bar`, `Baz` (each caller
// then normalises case). Consecutive upper-case letters stay grouped
// with the following segment: `HTTPServer` -> `HTTP`, `Server`.
func splitCamel(s string) []string {
	if s == "" {
		return nil
	}

	var (
		out   []string
		start int
	)

	runes := []rune(s)
	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		curr := runes[i]

		// lower -> upper boundary
		if unicode.IsLower(prev) && unicode.IsUpper(curr) {
			out = append(out, string(runes[start:i]))
			start = i
		}

		// upper -> lower boundary (end of an acronym), only if we have
		// at least 2 uppers preceding
		if i > 1 && unicode.IsUpper(prev) && unicode.IsLower(curr) && unicode.IsUpper(runes[i-2]) {
			out = append(out, string(runes[start:i-1]))
			start = i - 1
		}
	}

	if start < len(runes) {
		out = append(out, string(runes[start:]))
	}

	return out
}
