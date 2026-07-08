package renderer

import (
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// SharedFuncMap contains the default template functions available to all
// templates created via NewTemplate. Commands can extend this with local
// functions that take precedence.
var SharedFuncMap = template.FuncMap{
	// dash returns "—" for nil pointers and empty strings.
	// Accepts string, *string, or any nillable type.
	"dash": func(v any) string {
		if v == nil {
			return "—"
		}

		switch val := v.(type) {
		case string:
			if strings.TrimSpace(val) == "" {
				return "—"
			}

			return val
		case *string:
			if val == nil || strings.TrimSpace(*val) == "" {
				return "—"
			}

			return *val
		default:
			rv := reflect.ValueOf(v)
			switch rv.Kind() {
			case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
				if rv.IsNil() {
					return "—"
				}
			}

			s := fmt.Sprint(v)
			if strings.TrimSpace(s) == "" {
				return "—"
			}

			return s
		}
	},

	// ts formats a *time.Time as RFC3339, or returns "—" if nil/zero.
	"ts": func(v any) string {
		if v == nil {
			return "—"
		}

		switch t := v.(type) {
		case time.Time:
			if t.IsZero() {
				return "—"
			}

			return t.Format(time.RFC3339)
		case *time.Time:
			if t == nil || t.IsZero() {
				return "—"
			}

			return t.Format(time.RFC3339)
		default:
			return fmt.Sprint(v)
		}
	},

	// bool renders a boolean as "true" or "false".
	"bool": func(v bool) string {
		if v {
			return "true"
		}

		return "false"
	},

	// join wraps strings.Join for use in templates.
	//   {{ join .Scopes ", " }}
	"join": func(elems []string, sep string) string {
		return strings.Join(elems, sep)
	},

	// pad right-pads a string to the given width with spaces.
	"pad": func(width int, s string) string {
		if len(s) >= width {
			return s
		}

		return s + strings.Repeat(" ", width-len(s))
	},

	// hpad is an alias for pad ("header pad") used in table headers.
	"hpad": func(width int, s string) string {
		if len(s) >= width {
			return s
		}

		return s + strings.Repeat(" ", width-len(s))
	},

	// tsOrDash formats a *time.Time as RFC3339, or returns "—" if nil/zero.
	// Alias for ts, kept for template compatibility.
	"tsOrDash": func(v any) string {
		if v == nil {
			return "—"
		}

		switch t := v.(type) {
		case time.Time:
			if t.IsZero() {
				return "—"
			}

			return t.Format(time.RFC3339)
		case *time.Time:
			if t == nil || t.IsZero() {
				return "—"
			}

			return t.Format(time.RFC3339)
		default:
			return fmt.Sprint(v)
		}
	},

	// upper converts a string to uppercase.
	"upper": strings.ToUpper,

	// lower converts a string to lowercase.
	"lower": strings.ToLower,

	// title converts a string to title case.
	"title": cases.Title(language.English).String,

	// plural returns singular or plural form based on count.
	//   {{ plural .Count "item" "items" }}
	"plural": func(count int, singular, plural string) string {
		if count == 1 {
			return singular
		}

		return plural
	},
}

func init() {
	// Set FuncMap to SharedFuncMap so NewTemplate uses these by default.
	FuncMap = SharedFuncMap
}
