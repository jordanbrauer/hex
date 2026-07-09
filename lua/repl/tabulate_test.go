package repl

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// newTable builds an LTable from a Lua source snippet — easier than
// hand-wiring RawSet calls for every fixture.
func newTable(t *testing.T, source string) *lua.LTable {
	t.Helper()

	L := lua.NewState()
	t.Cleanup(L.Close)

	if err := L.DoString("return " + source); err != nil {
		t.Fatalf("build fixture: %v", err)
	}

	v := L.Get(-1)

	tbl, ok := v.(*lua.LTable)
	if !ok {
		t.Fatalf("fixture did not produce table, got %T", v)
	}

	return tbl
}

func TestTabulate_empty(t *testing.T) {
	if got := tabulate(newTable(t, "{}")); got != "{}" {
		t.Errorf("empty table = %q, want %q", got, "{}")
	}
}

func TestTabulate_arrayOfRecords_unionsAllKeys(t *testing.T) {
	// Two rows with overlapping keys — c only appears in row 2. All
	// three columns must be present in the header, and the missing
	// cell must render as empty.
	fixture := `{
        { a = 1, b = "hi" },
        { a = 2, b = "yo", c = true },
    }`

	got := tabulate(newTable(t, fixture))

	for _, want := range []string{"a", "b", "c", "hi", "yo", "true"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n---output---\n%s", want, got)
		}
	}
}

func TestTabulate_arrayOfRecords_columnsSortedAlphabetically(t *testing.T) {
	// gopher-lua doesn't preserve insertion order for string-keyed
	// entries, so tabulate sorts columns alphabetically for a
	// deterministic display. Rows introduce keys in order name, id,
	// email — header must render in alphabetical order regardless.
	fixture := `{
        { name = "Luke",  id = 1 },
        { name = "Vader", id = 2, email = "vader@empire.gov" },
    }`

	got := tabulate(newTable(t, fixture))

	iEmail := strings.Index(got, "email")
	iID := strings.Index(got, "id")
	iName := strings.Index(got, "name")

	if iEmail < 0 || iID < 0 || iName < 0 {
		t.Fatalf("header missing one of email/id/name\n%s", got)
	}

	if !(iEmail < iID && iID < iName) {
		t.Errorf("column order not alphabetical: email@%d id@%d name@%d\n%s", iEmail, iID, iName, got)
	}
}

func TestTabulate_arrayOfScalars_singleColumn(t *testing.T) {
	got := tabulate(newTable(t, `{ "widgets", "gadgets", "gizmos" }`))

	if !strings.Contains(got, "value") {
		t.Errorf("scalar array missing 'value' header\n%s", got)
	}

	for _, want := range []string{"widgets", "gadgets", "gizmos"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

func TestTabulate_map_twoColumnKeyValue(t *testing.T) {
	got := tabulate(newTable(t, `{ name = "Luke", height = 172, gender = "male" }`))

	for _, want := range []string{"key", "value", "name", "Luke", "172"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

func TestTabulate_nestedTablesCollapse(t *testing.T) {
	// The inner table in the `films` column shouldn't be printed as
	// a giant sub-grid — it collapses to {…} so the outer grid
	// stays a fixed shape.
	fixture := `{
        { name = "Luke", films = { "A New Hope", "Empire" } },
    }`

	got := tabulate(newTable(t, fixture))

	if !strings.Contains(got, "{…}") {
		t.Errorf("nested table not collapsed to placeholder\n%s", got)
	}

	if strings.Contains(got, "A New Hope") {
		t.Errorf("nested table contents leaked into cell\n%s", got)
	}
}
