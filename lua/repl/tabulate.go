package repl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	lua "github.com/yuin/gopher-lua"
)

// tabulate renders a Lua table for REPL display, picking the layout
// that best matches the table's shape:
//
//   - empty                    → "{}"
//   - array of records (rows)  → bordered table, columns = union of
//     all keys across rows in first-seen order
//   - array of scalars         → single-column "value" table
//   - map / mixed / nested     → two-column "key | value" table
//
// Nested Lua tables inside a cell are rendered as "{…}" so the
// output stays a fixed grid; drill into them by indexing the source
// value at the prompt.
func tabulate(t *lua.LTable) string {
	if t.Len() == 0 && countMapKeys(t) == 0 {
		return "{}"
	}

	if rows, cols, ok := arrayOfRecords(t); ok {
		return renderRecordTable(cols, rows)
	}

	if values, ok := arrayOfScalars(t); ok {
		rows := make([][]string, len(values))
		for i, v := range values {
			rows[i] = []string{v}
		}

		return renderRecordTable([]string{"value"}, rows)
	}

	return renderMap(t)
}

// prependIndexColumn adds a `#` column at column 0 with 0-indexed
// row numbers, so tabulate output matches nushell-style listings
// (see docs/repl.md for the reference shape).
func prependIndexColumn(cols []string, rows [][]string) ([]string, [][]string) {
	newCols := make([]string, 0, len(cols)+1)
	newCols = append(newCols, "#")
	newCols = append(newCols, cols...)

	newRows := make([][]string, len(rows))
	for i, r := range rows {
		newRow := make([]string, 0, len(r)+1)
		newRow = append(newRow, fmt.Sprintf("%d", i))
		newRow = append(newRow, r...)
		newRows[i] = newRow
	}

	return newCols, newRows
}

// arrayOfRecords reports whether t is a 1..n indexed array whose
// every element is itself an LTable — the shape SQL query results
// come back as. When true, returns the string cells and the
// column-key list (union across all rows, sorted alphabetically
// for deterministic output).
//
// Sort choice: gopher-lua's LTable stores string-keyed entries in a
// `map[string]LValue`, whose iteration order is randomised by the
// Go runtime. There is no way to recover source / SQL column order
// from the LTable alone. Alphabetical is the least-surprising
// stable order until db.query grows an ordered-row shape (follow-up).
func arrayOfRecords(t *lua.LTable) ([][]string, []string, bool) {
	n := t.Len()
	if n == 0 {
		return nil, nil, false
	}

	// Every 1..n element must be a table.
	rowTables := make([]*lua.LTable, 0, n)

	for i := 1; i <= n; i++ {
		v := t.RawGetInt(i)

		rt, ok := v.(*lua.LTable)
		if !ok {
			return nil, nil, false
		}

		rowTables = append(rowTables, rt)
	}

	// Union of string keys across all rows.
	seen := map[string]bool{}

	for _, rt := range rowTables {
		rt.ForEach(func(k, _ lua.LValue) {
			if ks, ok := k.(lua.LString); ok {
				seen[string(ks)] = true
			}
		})
	}

	if len(seen) == 0 {
		return nil, nil, false
	}

	cols := make([]string, 0, len(seen))
	for k := range seen {
		cols = append(cols, k)
	}

	sort.Strings(cols)

	rows := make([][]string, len(rowTables))

	for i, rt := range rowTables {
		row := make([]string, len(cols))
		for j, c := range cols {
			row[j] = cellString(rt.RawGetString(c))
		}

		rows[i] = row
	}

	return rows, cols, true
}

// arrayOfScalars reports whether t is a 1..n indexed array whose
// every element is a non-table value.
func arrayOfScalars(t *lua.LTable) ([]string, bool) {
	n := t.Len()
	if n == 0 {
		return nil, false
	}

	// Must have no non-array (string-keyed) entries.
	if countMapKeys(t) > 0 {
		return nil, false
	}

	out := make([]string, 0, n)

	for i := 1; i <= n; i++ {
		v := t.RawGetInt(i)
		if _, isTable := v.(*lua.LTable); isTable {
			return nil, false
		}

		out = append(out, cellString(v))
	}

	return out, true
}

// renderMap draws a two-column "key | value" grid for tables that
// aren't pure arrays. Preserves iteration order from LTable.ForEach
// (insertion order for gopher-lua string-keyed entries).
func renderMap(t *lua.LTable) string {
	var (
		rows [][]string
		n    = t.Len()
	)

	// Array portion first (numeric 1..n).
	for i := 1; i <= n; i++ {
		rows = append(rows, []string{fmt.Sprintf("%d", i), cellString(t.RawGetInt(i))})
	}

	// String / hash portion.
	t.ForEach(func(k, v lua.LValue) {
		if _, isInt := k.(lua.LNumber); isInt {
			// Already in the array portion.
			if in := int(k.(lua.LNumber)); in >= 1 && in <= n && float64(in) == float64(k.(lua.LNumber)) {
				return
			}
		}

		rows = append(rows, []string{k.String(), cellString(v)})
	})

	return renderRecordTable([]string{"key", "value"}, rows)
}

// renderRecordTable draws an open-sided table via lipgloss/table,
// nushell-style: no left/right vertical borders, top/bottom rules
// only extended between column separators, header row underlined,
// leading `#` column with 0-indexed row numbers right-aligned.
//
// Style is deliberately quiet — dim border, bold header — so the
// grid doesn't fight the surrounding REPL scrollback.
func renderRecordTable(cols []string, rows [][]string) string {
	cols, rows = prependIndexColumn(cols, rows)

	border := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245")).Padding(0, 1)
	cell := lipgloss.NewStyle().Padding(0, 1)
	index := cell.Foreground(lipgloss.Color("245")).Align(lipgloss.Right)

	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(border).
		BorderLeft(false).
		BorderRight(false).
		Headers(cols...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow && col == 0:
				return header.Align(lipgloss.Right)
			case row == table.HeaderRow:
				return header
			case col == 0:
				return index
			default:
				return cell
			}
		})

	return strings.TrimRight(tbl.String(), "\n")
}

// countMapKeys counts non-array (string- or non-1..n-indexed)
// entries in t. Cheap way to distinguish pure arrays from maps /
// mixed tables without allocating an intermediate slice.
func countMapKeys(t *lua.LTable) int {
	n := t.Len()
	count := 0

	t.ForEach(func(k, _ lua.LValue) {
		if kn, ok := k.(lua.LNumber); ok {
			i := int(kn)
			if float64(i) == float64(kn) && i >= 1 && i <= n {
				return
			}
		}

		count++
	})

	return count
}

// cellString renders one value for a table cell. Nested tables
// collapse to a placeholder so the grid stays fixed-width.
func cellString(v lua.LValue) string {
	if v == nil || v == lua.LNil {
		return ""
	}

	if _, ok := v.(*lua.LTable); ok {
		return "{…}"
	}

	if s, ok := v.(lua.LString); ok {
		return string(s)
	}

	return v.String()
}
