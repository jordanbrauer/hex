package main

import "testing"

func TestPascalCase(t *testing.T) {
	tests := map[string]string{
		"user":         "User",
		"user-account": "UserAccount",
		"user_account": "UserAccount",
		"user account": "UserAccount",
		"USER":         "User",
		"userAccount":  "UserAccount",
		"HTTPServer":   "HttpServer",
		"":             "",
	}

	for in, want := range tests {
		if got := pascalCase(in); got != want {
			t.Errorf("pascalCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCamelCase(t *testing.T) {
	tests := map[string]string{
		"user":         "user",
		"user-account": "userAccount",
		"userAccount":  "userAccount",
	}

	for in, want := range tests {
		if got := camelCase(in); got != want {
			t.Errorf("camelCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSnakeCase(t *testing.T) {
	tests := map[string]string{
		"user":            "user",
		"userAccount":     "user_account",
		"user-account":    "user_account",
		"UserAccountLine": "user_account_line",
	}

	for in, want := range tests {
		if got := snakeCase(in); got != want {
			t.Errorf("snakeCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPluralise(t *testing.T) {
	tests := map[string]string{
		"user":     "users",
		"box":      "boxes",
		"church":   "churches",
		"story":    "stories",
		"key":      "keys",
		"buzz":     "buzzes",
		"policy":   "policies",
		"birthday": "birthdays",
	}

	for in, want := range tests {
		if got := pluralise(in); got != want {
			t.Errorf("pluralise(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGoPackageName(t *testing.T) {
	tests := map[string]string{
		"user":       "user",
		"User":       "user",
		"user-thing": "userthing",
		"user_thing": "userthing",
		"user thing": "userthing",
		"user123":    "user123",
		"user!!name": "username",
	}

	for in, want := range tests {
		if got := goPackageName(in); got != want {
			t.Errorf("goPackageName(%q) = %q, want %q", in, got, want)
		}
	}
}
