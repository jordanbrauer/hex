package errors_test

import (
	stdErrors "errors"
	"net/http"
	"testing"

	"github.com/jordanbrauer/hex/errors"
)

var (
	ErrNotFound = errors.New(errors.CodeNotFound, "not found")
	ErrConflict = errors.New(errors.CodeConflict, "conflict")
)

func TestNew_carriesCodeAndMessage(t *testing.T) {
	e := errors.New(errors.CodeInvalidArgument, "bad input")

	if e.Code() != errors.CodeInvalidArgument {
		t.Errorf("Code = %v, want CodeInvalidArgument", e.Code())
	}

	if e.Error() != "bad input" {
		t.Errorf("Error = %q, want %q", e.Error(), "bad input")
	}
}

func TestNewf_formatting(t *testing.T) {
	e := errors.Newf(errors.CodeNotFound, "user %q not found", "alice")

	if e.Error() != `user "alice" not found` {
		t.Errorf("Error = %q", e.Error())
	}
}

func TestIs_matchesByCode(t *testing.T) {
	err := errors.Newf(errors.CodeNotFound, "user %q", "alice")

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Is(err, ErrNotFound) = false; codes match, want true")
	}

	if errors.Is(err, ErrConflict) {
		t.Errorf("Is(err, ErrConflict) = true; codes differ")
	}
}

func TestWrap_preservesCauseAndCode(t *testing.T) {
	cause := stdErrors.New("row missing")
	err := errors.Wrap(cause, ErrNotFound, "user %q", "alice")

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Is(wrapped, ErrNotFound) = false")
	}

	if !stdErrors.Is(err, cause) {
		t.Errorf("cause not reachable via errors.Is")
	}

	want := `user "alice": row missing`
	if err.Error() != want {
		t.Errorf("Error = %q, want %q", err.Error(), want)
	}
}

func TestWrap_nilTargetInheritsCauseCode(t *testing.T) {
	inner := errors.New(errors.CodeConflict, "already exists")
	wrapped := errors.Wrap(inner, nil, "creating user")

	if wrapped.Code() != errors.CodeConflict {
		t.Errorf("Wrap(nil target) Code = %v, want inherit %v", wrapped.Code(), errors.CodeConflict)
	}
}

func TestCodeOf_defaultsToInternal(t *testing.T) {
	plain := stdErrors.New("boom")

	if got := errors.CodeOf(plain); got != errors.CodeInternal {
		t.Errorf("CodeOf(plain) = %v, want CodeInternal", got)
	}

	if got := errors.CodeOf(nil); got != "" {
		t.Errorf("CodeOf(nil) = %v, want empty", got)
	}
}

func TestHTTPStatus(t *testing.T) {
	tests := map[error]int{
		errors.New(errors.CodeInvalidArgument, ""): http.StatusBadRequest,
		errors.New(errors.CodeUnauthorized, ""):    http.StatusUnauthorized,
		errors.New(errors.CodeForbidden, ""):       http.StatusForbidden,
		errors.New(errors.CodeNotFound, ""):        http.StatusNotFound,
		errors.New(errors.CodeConflict, ""):        http.StatusConflict,
		errors.New(errors.CodeRateLimited, ""):     http.StatusTooManyRequests,
		errors.New(errors.CodeTimeout, ""):         http.StatusGatewayTimeout,
		errors.New(errors.CodeUnavailable, ""):     http.StatusServiceUnavailable,
		errors.New(errors.CodeInternal, ""):        http.StatusInternalServerError,
		stdErrors.New("plain"):                     http.StatusInternalServerError,
	}

	for err, want := range tests {
		if got := errors.HTTPStatus(err); got != want {
			t.Errorf("HTTPStatus(%v) = %d, want %d", err, got, want)
		}
	}
}

func TestUnwrap_returnsCause(t *testing.T) {
	cause := stdErrors.New("inner")
	err := errors.Wrap(cause, ErrConflict, "outer")

	if got := errors.Unwrap(err); got != cause {
		t.Errorf("Unwrap = %v, want cause", got)
	}
}

func TestJoin_combinesErrors(t *testing.T) {
	a := errors.New(errors.CodeNotFound, "first")
	b := errors.New(errors.CodeConflict, "second")

	joined := errors.Join(a, b)

	if !errors.Is(joined, a) {
		t.Errorf("Join lost first error")
	}

	if !errors.Is(joined, b) {
		t.Errorf("Join lost second error")
	}
}
