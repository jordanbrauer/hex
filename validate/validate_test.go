package validate_test

import (
	"strings"
	"testing"

	hexerrors "github.com/jordanbrauer/hex/errors"
	"github.com/jordanbrauer/hex/validate"
)

func TestString_validates(t *testing.T) {
	schema := validate.String().Required().Min(3)

	var s string
	if issues := schema.Parse("hi!", &s); issues != nil {
		t.Errorf("valid input rejected: %v", issues)
	}

	if s != "hi!" {
		t.Errorf("parsed = %q", s)
	}

	if issues := schema.Parse("no", &s); issues == nil {
		t.Errorf("short input accepted")
	}
}

func TestInt_range(t *testing.T) {
	schema := validate.Int().GTE(18).LTE(120)

	var n int
	if issues := schema.Parse(25, &n); issues != nil {
		t.Errorf("valid age rejected: %v", issues)
	}

	if issues := schema.Parse(5, &n); issues == nil {
		t.Errorf("underage accepted")
	}

	if issues := schema.Parse(200, &n); issues == nil {
		t.Errorf("over max accepted")
	}
}

func TestBool_valid(t *testing.T) {
	schema := validate.Bool()

	var b bool
	if issues := schema.Parse(true, &b); issues != nil {
		t.Errorf("valid bool rejected: %v", issues)
	}

	if !b {
		t.Errorf("parsed = %v", b)
	}
}

func TestStruct_shape(t *testing.T) {
	type User struct {
		Email string
		Age   int
	}

	schema := validate.Struct(validate.Shape{
		"email": validate.String().Required().Email(),
		"age":   validate.Int().Required().GTE(18),
	})

	var u User

	issues := schema.Parse(map[string]any{
		"email": "alice@example.com",
		"age":   25,
	}, &u)

	if issues != nil {
		t.Errorf("valid struct rejected: %v", issues)
	}

	if u.Email != "alice@example.com" || u.Age != 25 {
		t.Errorf("parsed = %+v", u)
	}
}

func TestStruct_missingFieldsProduceIssues(t *testing.T) {
	type User struct {
		Email string
		Age   int
	}

	schema := validate.Struct(validate.Shape{
		"email": validate.String().Required(),
		"age":   validate.Int().Required().GTE(18),
	})

	var u User
	issues := schema.Parse(map[string]any{"email": "x"}, &u)

	if len(issues) == 0 {
		t.Errorf("missing required field accepted")
	}
}

func TestToError_carriesInvalidArgumentCode(t *testing.T) {
	schema := validate.String().Required().Min(5)

	var s string
	issues := schema.Parse("hi", &s)

	err := validate.ToError(issues)
	if err == nil {
		t.Fatal("ToError(issues) = nil")
	}

	if code := hexerrors.CodeOf(err); code != hexerrors.CodeInvalidArgument {
		t.Errorf("code = %v, want CodeInvalidArgument", code)
	}
}

func TestToError_nilOnEmptyIssues(t *testing.T) {
	if err := validate.ToError(nil); err != nil {
		t.Errorf("ToError(nil) = %v, want nil", err)
	}
}

func TestToError_joinsMultipleIssues(t *testing.T) {
	schema := validate.Struct(validate.Shape{
		"a": validate.String().Required().Min(5),
		"b": validate.Int().Required().GTE(10),
	})

	var out struct {
		A string
		B int
	}

	issues := schema.Parse(map[string]any{"a": "x", "b": 1}, &out)

	err := validate.ToError(issues)
	if err == nil {
		t.Fatal("expected error")
	}

	// Message should reference both fields somewhere.
	msg := err.Error()
	if !strings.Contains(msg, ";") {
		t.Errorf("error = %q, want multiple issues joined by ;", msg)
	}
}

func TestSlice_validatesElements(t *testing.T) {
	schema := validate.Slice(validate.Int().GTE(1))

	var xs []int

	issues := schema.Parse([]any{1, 2, 3}, &xs)
	if issues != nil {
		t.Errorf("valid slice rejected: %v", issues)
	}

	if len(xs) != 3 {
		t.Errorf("parsed len = %d, want 3", len(xs))
	}
}
