// Package validate is a hex-namespaced entrypoint for github.com/Oudwins/zog,
// a Zod-style schema parser + validator.
//
// Zog is already idiomatic Go and the wrapper is intentionally thin — hex
// re-exports the common builder functions and issue types so consumers
// get a hex-consistent import path (and one place to swap the library
// later) but do not lose zog's expressive API. For advanced features
// import the underlying package directly.
//
// The one hex-specific helper is ToError, which converts a zog issue list
// into a hex/errors typed error with CodeInvalidArgument, so API handlers
// can map validation failures to 400 responses uniformly.
//
// Example:
//
//	var userSchema = validate.Struct(validate.Shape{
//	    "email": validate.String().Required().Email(),
//	    "age":   validate.Int().Required().GTE(18),
//	})
//
//	type User struct {
//	    Email string
//	    Age   int
//	}
//
//	var u User
//	if issues := userSchema.Parse(input, &u); issues != nil {
//	    return validate.ToError(issues)  // *errors.Error with CodeInvalidArgument
//	}
package validate

import (
	"strings"

	z "github.com/Oudwins/zog"

	hexerrors "github.com/jordanbrauer/hex/errors"
)

// -- Re-exports: types ---------------------------------------------------

// Issue is a single validation failure with path and message.
type Issue = z.ZogIssue

// Issues is a slice of validation failures returned by Parse/Validate.
type Issues = z.ZogIssueList

// Shape describes a struct schema — a map of field names to per-field
// schemas.
type Shape = z.Shape

// Schema is the interface every validator implements.
type Schema = z.ZogSchema

// -- Re-exports: builders ------------------------------------------------

// String creates a schema for string values.
func String(opts ...z.SchemaOption) *z.StringSchema[string] {
	return z.String(opts...)
}

// Int creates a schema for int values.
func Int(opts ...z.SchemaOption) *z.NumberSchema[int] {
	return z.Int(opts...)
}

// Int64 creates a schema for int64 values.
func Int64(opts ...z.SchemaOption) *z.NumberSchema[int64] {
	return z.Int64(opts...)
}

// Float creates a schema for float64 values.
func Float(opts ...z.SchemaOption) *z.NumberSchema[float64] {
	return z.Float(opts...)
}

// Bool creates a schema for bool values.
func Bool(opts ...z.SchemaOption) *z.BoolSchema[bool] {
	return z.Bool(opts...)
}

// Slice creates a schema for a slice whose elements validate against schema.
func Slice(schema z.ZogSchema, opts ...z.SchemaOption) *z.SliceSchema {
	return z.Slice(schema, opts...)
}

// Struct creates a schema for a struct described by shape.
func Struct(shape Shape) *z.StructSchema {
	return z.Struct(shape)
}

// -- hex/errors integration ----------------------------------------------

// ToError converts a zog issue list into a hex/errors typed error carrying
// CodeInvalidArgument. Multiple issues are joined with a semicolon so the
// resulting Error message reads naturally in logs; consumers who want
// per-field detail should walk the raw Issues.
//
// Returns nil if issues is empty — callers can:
//
//	if err := validate.ToError(schema.Parse(in, &out)); err != nil {
//	    return err
//	}
func ToError(issues Issues) error {
	if len(issues) == 0 {
		return nil
	}

	parts := make([]string, 0, len(issues))
	for _, iss := range issues {
		parts = append(parts, formatIssue(iss))
	}

	return hexerrors.New(hexerrors.CodeInvalidArgument, strings.Join(parts, "; "))
}

// formatIssue renders one issue as "path.to.field: message". Zog's Path
// is a []string of segments; we join with dots for a familiar
// dotted-path shape.
func formatIssue(iss *z.ZogIssue) string {
	path := strings.Join(iss.Path, ".")
	msg := iss.Message

	switch {
	case path == "" && msg == "":
		return "invalid"
	case path == "":
		return msg
	case msg == "":
		return path + ": invalid"
	default:
		return path + ": " + msg
	}
}
