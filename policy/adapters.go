package policy

import (
	"bufio"
	"bytes"
	"errors"
	"strings"

	casbinmodel "github.com/casbin/casbin/v2/model"
	casbinpersist "github.com/casbin/casbin/v2/persist"
)

// memoryAdapter is an in-process Adapter implementation. It seeds Casbin's
// in-memory model with any provided rows at LoadPolicy time and mutates
// itself on Add/RemovePolicy calls.
//
// Casbin ships an in-memory adapter, but it only exists as a Filter-supporting
// wrapper (memory-with-filter). This one is simpler and avoids the extra
// import surface.
type memoryAdapter struct {
	seed [][]string
	rows [][]string // authoritative store after first LoadPolicy
}

// LoadPolicy fills the Casbin model with the adapter's current rows.
func (a *memoryAdapter) LoadPolicy(model casbinmodel.Model) error {
	// On the very first load use seed; on subsequent loads use rows so
	// Enforcer.LoadPolicy() reflects the running state.
	if a.rows == nil {
		a.rows = make([][]string, len(a.seed))
		copy(a.rows, a.seed)
	}

	for _, row := range a.rows {
		if err := casbinpersist.LoadPolicyArray(row, model); err != nil {
			return err
		}
	}

	return nil
}

// SavePolicy replaces the adapter's rows with everything currently in the
// model. Called by Enforcer.SavePolicy.
func (a *memoryAdapter) SavePolicy(model casbinmodel.Model) error {
	a.rows = a.rows[:0]

	for ptype, ast := range model["p"] {
		for _, rule := range ast.Policy {
			a.rows = append(a.rows, append([]string{ptype}, rule...))
		}
	}

	for gtype, ast := range model["g"] {
		for _, rule := range ast.Policy {
			a.rows = append(a.rows, append([]string{gtype}, rule...))
		}
	}

	return nil
}

// AddPolicy appends one policy row.
func (a *memoryAdapter) AddPolicy(sec, ptype string, rule []string) error {
	row := append([]string{ptype}, rule...)
	a.rows = append(a.rows, row)

	return nil
}

// RemovePolicy removes a policy row exactly matching ptype+rule.
func (a *memoryAdapter) RemovePolicy(sec, ptype string, rule []string) error {
	for i, existing := range a.rows {
		if !rowMatches(existing, ptype, rule) {
			continue
		}

		a.rows = append(a.rows[:i], a.rows[i+1:]...)

		return nil
	}

	return nil // Casbin treats missing rows as "already gone"
}

// RemoveFilteredPolicy removes rows whose fields (starting at fieldIndex)
// all match fieldValues. Empty field values are wildcards.
func (a *memoryAdapter) RemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	filtered := a.rows[:0]

	for _, row := range a.rows {
		if !rowFilterMatches(row, ptype, fieldIndex, fieldValues) {
			filtered = append(filtered, row)
		}
	}

	a.rows = filtered

	return nil
}

// rowMatches reports whether row equals ptype+rule.
func rowMatches(row []string, ptype string, rule []string) bool {
	if len(row) != 1+len(rule) || row[0] != ptype {
		return false
	}

	for i, v := range rule {
		if row[i+1] != v {
			return false
		}
	}

	return true
}

// rowFilterMatches reports whether row starts with ptype and matches the
// filter starting at fieldIndex. Empty filter values are wildcards.
func rowFilterMatches(row []string, ptype string, fieldIndex int, fieldValues []string) bool {
	if len(row) < 1 || row[0] != ptype {
		return false
	}

	for i, v := range fieldValues {
		if v == "" {
			continue
		}

		idx := 1 + fieldIndex + i
		if idx >= len(row) || row[idx] != v {
			return false
		}
	}

	return true
}

// -- fsAdapter (read-only embed.FS) ---------------------------------------

// fsAdapter loads policy rows from a CSV blob (typically embedded via
// //go:embed). Writes are rejected — this adapter is for baked-in
// baseline policies.
type fsAdapter struct {
	data []byte
	err  error
}

func (a *fsAdapter) LoadPolicy(model casbinmodel.Model) error {
	if a.err != nil {
		return a.err
	}

	scanner := bufio.NewScanner(bytes.NewReader(a.data))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		row := splitCSV(line)
		if err := casbinpersist.LoadPolicyArray(row, model); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// SavePolicy is not supported by the fs adapter.
func (a *fsAdapter) SavePolicy(casbinmodel.Model) error {
	return errors.New("policy: fs adapter is read-only")
}

func (a *fsAdapter) AddPolicy(string, string, []string) error {
	return errors.New("policy: fs adapter is read-only")
}

func (a *fsAdapter) RemovePolicy(string, string, []string) error {
	return errors.New("policy: fs adapter is read-only")
}

func (a *fsAdapter) RemoveFilteredPolicy(string, string, int, ...string) error {
	return errors.New("policy: fs adapter is read-only")
}

// splitCSV performs Casbin's rule-line split: comma-separated, trimmed.
// Casbin's own file adapter uses encoding/csv, but that requires reading
// full lines; for embedded policies we do the same thing more simply.
func splitCSV(line string) []string {
	parts := strings.Split(line, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}

	return parts
}
