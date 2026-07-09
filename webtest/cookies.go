package webtest

import (
	"net/http/cookiejar"

	"golang.org/x/net/publicsuffix"
)

// cookiejar returns a fresh in-memory cookie jar backed by the
// public-suffix list, matching what browsers do for cookie scoping.
// Errors are unreachable in practice (cookiejar.New only fails on
// nil Options with a nil-check that never trips); returned as-is
// for API cleanliness.
func newCookieJar() (*cookiejar.Jar, error) {
	return cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
}
