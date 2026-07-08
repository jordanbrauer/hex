// Package errors adds typed error semantics on top of the stdlib errors
// package: a stable machine-readable Code, an optional HTTP status hint,
// and Wrap/Is/As support. Consumers who just want stdlib behaviour can
// keep using errors — this package is opt-in for structured errors.
//
// Typical usage in a service:
//
//	ErrNotFound  = errors.New(errors.CodeNotFound, "not found")
//	ErrConflict  = errors.New(errors.CodeConflict, "conflict")
//
//	func GetUser(id string) (*User, error) {
//	    u, err := repo.Find(id)
//	    if err != nil {
//	        return nil, errors.Wrap(err, ErrNotFound, "user %q", id)
//	    }
//	    return u, nil
//	}
//
//	// In an HTTP handler:
//	if err := svc.Do(); err != nil {
//	    status := errors.HTTPStatus(err)          // 404 for ErrNotFound
//	    code := errors.CodeOf(err)                // "not_found"
//	    // ...
//	}
package errors

import (
	stdErrors "errors"
	"fmt"
	"net/http"
)

// Code is a stable machine-readable identifier. Consumers define their own
// codes as constants; hex ships the common ones as a starting point.
type Code string

// Common code constants. Consumers may extend with their own; these cover
// the most frequent HTTP-mappable failures.
const (
	CodeInternal        Code = "internal"
	CodeInvalidArgument Code = "invalid_argument"
	CodeNotFound        Code = "not_found"
	CodeConflict        Code = "conflict"
	CodeUnauthorized    Code = "unauthorized"
	CodeForbidden       Code = "forbidden"
	CodeRateLimited     Code = "rate_limited"
	CodeUnavailable     Code = "unavailable"
	CodeTimeout         Code = "timeout"
)

// Error is the typed error type. It carries a Code, a message, and an
// optional wrapped cause.
type Error struct {
	code    Code
	message string
	cause   error
}

// New returns a fresh Error with the given code and message.
func New(code Code, msg string) *Error {
	return &Error{code: code, message: msg}
}

// Newf is New with fmt.Sprintf-style formatting.
func Newf(code Code, format string, args ...any) *Error {
	return &Error{code: code, message: fmt.Sprintf(format, args...)}
}

// Wrap attaches cause to a new Error carrying msg. The result satisfies
// errors.Is / errors.As against both the target sentinel and the cause.
// If target is nil, cause's own code is preserved when possible.
func Wrap(cause error, target *Error, format string, args ...any) *Error {
	e := &Error{
		message: fmt.Sprintf(format, args...),
		cause:   cause,
	}

	switch {
	case target != nil:
		e.code = target.code
	default:
		e.code = CodeOf(cause)
	}

	return e
}

// Error returns the fully composed message. If a cause is attached, its
// message follows a colon.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}

	if e.cause == nil {
		return e.message
	}

	if e.message == "" {
		return e.cause.Error()
	}

	return e.message + ": " + e.cause.Error()
}

// Code returns the Error's code, or CodeInternal if unset.
func (e *Error) Code() Code {
	if e == nil || e.code == "" {
		return CodeInternal
	}

	return e.code
}

// Unwrap returns the wrapped cause so errors.Is / errors.As walk the chain.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.cause
}

// Is matches on Code equality. This makes sentinel comparisons work:
//
//	errors.Is(err, ErrNotFound)  // true when err has code "not_found"
func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}

	t, ok := target.(*Error)
	if !ok {
		return false
	}

	return e.Code() == t.Code()
}

// -- helpers --------------------------------------------------------------

// CodeOf returns err's code if it (or any wrapped error) is an *Error;
// otherwise CodeInternal.
func CodeOf(err error) Code {
	if err == nil {
		return ""
	}

	var e *Error
	if stdErrors.As(err, &e) {
		return e.Code()
	}

	return CodeInternal
}

// HTTPStatus maps err's code to a suggested HTTP status. Unknown codes
// fall through to 500. Consumers who want app-specific mappings should
// bring their own function; this is a reasonable default only.
func HTTPStatus(err error) int {
	switch CodeOf(err) {
	case CodeInvalidArgument:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeTimeout:
		return http.StatusGatewayTimeout
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// Is is a passthrough to errors.Is for consumers who use this package by
// default and want to avoid two errors imports.
func Is(err, target error) bool { return stdErrors.Is(err, target) }

// As is a passthrough to errors.As.
func As(err error, target any) bool { return stdErrors.As(err, target) }

// Unwrap is a passthrough to errors.Unwrap.
func Unwrap(err error) error { return stdErrors.Unwrap(err) }

// Join is a passthrough to errors.Join.
func Join(errs ...error) error { return stdErrors.Join(errs...) }
