package command

import (
	"strings"
	"testing"
)

func TestHelp_everyCommandHasLongAndExample(t *testing.T) {
	keys := []string{
		"init", "publish",
		"make_provider", "make_domain", "make_migration",
		"make_command", "make_adapter", "make_controller",
	}

	for _, k := range keys {
		if strings.TrimSpace(helpLong(k)) == "" {
			t.Errorf("%s.long.md is empty", k)
		}

		if strings.TrimSpace(helpExample(k)) == "" {
			t.Errorf("%s.example.sh is empty", k)
		}
	}
}
