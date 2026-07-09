package webtest

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// Selection is a chainable assertion wrapper around a goquery
// selection. Modeled after react-testing-library's queries and
// Laravel's TestResponse assertions.
//
// Every assertion method returns the same Selection so calls chain:
//
//	client.Find(".user-card").
//	    HasClass("active").
//	    HasText("Alice").
//	    Count(3)
type Selection struct {
	t        testing.TB
	client   *Client
	selector string
	sel      *goquery.Selection
}

// Underlying returns the wrapped *goquery.Selection. Escape hatch
// when callers need the full goquery API.
func (s *Selection) Underlying() *goquery.Selection { return s.sel }

// Count asserts the selection contains exactly n elements. Zero
// asserts absence; use Exists / DoesNotExist for readability.
func (s *Selection) Count(n int) *Selection {
	s.t.Helper()

	if got := s.sel.Length(); got != n {
		s.t.Errorf("%s: %q count = %d, want %d",
			s.client.lastReq, s.selector, got, n)
	}

	return s
}

// Exists asserts at least one element matched the selector.
func (s *Selection) Exists() *Selection {
	s.t.Helper()

	if s.sel.Length() == 0 {
		s.t.Errorf("%s: %q not found in response",
			s.client.lastReq, s.selector)
	}

	return s
}

// DoesNotExist asserts no elements matched the selector.
func (s *Selection) DoesNotExist() *Selection {
	s.t.Helper()

	if s.sel.Length() != 0 {
		s.t.Errorf("%s: %q found in response (expected absent)",
			s.client.lastReq, s.selector)
	}

	return s
}

// HasText asserts the selected elements' combined text content
// contains substring. Whitespace is not normalised — pass exactly
// what you expect (with mindful spacing).
func (s *Selection) HasText(substring string) *Selection {
	s.t.Helper()

	text := s.sel.Text()
	if !strings.Contains(text, substring) {
		s.t.Errorf("%s: %q text missing %q\nactual text:\n%s",
			s.client.lastReq, s.selector, substring, text)
	}

	return s
}

// HasExactText asserts the selected element's text (trimmed) is
// exactly equal to want.
func (s *Selection) HasExactText(want string) *Selection {
	s.t.Helper()

	got := strings.TrimSpace(s.sel.Text())
	if got != want {
		s.t.Errorf("%s: %q text = %q, want %q",
			s.client.lastReq, s.selector, got, want)
	}

	return s
}

// HasClass asserts the selection has the given CSS class.
func (s *Selection) HasClass(class string) *Selection {
	s.t.Helper()

	if !s.sel.HasClass(class) {
		s.t.Errorf("%s: %q missing class %q",
			s.client.lastReq, s.selector, class)
	}

	return s
}

// HasAttribute asserts the selected element has attribute name (any value).
func (s *Selection) HasAttribute(name string) *Selection {
	s.t.Helper()

	if _, exists := s.sel.Attr(name); !exists {
		s.t.Errorf("%s: %q missing attribute %q",
			s.client.lastReq, s.selector, name)
	}

	return s
}

// HasAttributeValue asserts the attribute name equals want.
func (s *Selection) HasAttributeValue(name, want string) *Selection {
	s.t.Helper()

	got, exists := s.sel.Attr(name)
	if !exists {
		s.t.Errorf("%s: %q missing attribute %q",
			s.client.lastReq, s.selector, name)

		return s
	}

	if got != want {
		s.t.Errorf("%s: %q attribute %q = %q, want %q",
			s.client.lastReq, s.selector, name, got, want)
	}

	return s
}

// Attr returns the value of attribute name on the first matched element.
// If missing, returns "" and reports the miss (does not fail).
func (s *Selection) Attr(name string) string {
	v, _ := s.sel.Attr(name)

	return v
}

// Text returns the combined text content of all selected elements.
func (s *Selection) Text() string { return s.sel.Text() }

// First returns a Selection for the first matched element.
func (s *Selection) First() *Selection {
	return &Selection{
		t:        s.t,
		client:   s.client,
		selector: s.selector + " (first)",
		sel:      s.sel.First(),
	}
}

// Nth returns a Selection for the nth (0-indexed) matched element.
func (s *Selection) Nth(n int) *Selection {
	return &Selection{
		t:        s.t,
		client:   s.client,
		selector: s.selector,
		sel:      s.sel.Eq(n),
	}
}

// Find scopes a further selection inside the currently selected
// elements — same semantics as CSS descendant combinator.
func (s *Selection) Find(sel string) *Selection {
	return &Selection{
		t:        s.t,
		client:   s.client,
		selector: s.selector + " " + sel,
		sel:      s.sel.Find(sel),
	}
}
