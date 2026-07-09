// Package bdd wires the hex/webtest client into a hex/bdd suite as
// a set of standard Gherkin step definitions, giving apps a
// react-testing-library-flavoured browser-test vocabulary in
// .feature files:
//
//	Feature: dashboard
//
//	  Scenario: authenticated user sees welcome
//	    When I visit "/dashboard"
//	    Then the response status should be 200
//	    And I should see "Welcome, Alice"
//	    And "h1" should have text "Welcome, Alice"
//	    And there should be 3 elements matching ".user-card"
//	    And "button[data-testid=add-user]" should exist
//
//	  Scenario: bad credentials
//	    When I post form to "/login":
//	      | email    | user@example.com |
//	      | password | wrong            |
//	    Then the response status should be 401
//	    And I should see "bad credentials"
//
// Usage from a test:
//
//	func TestFeatures(t *testing.T) {
//	    app := hextest.NewApp(t, myproviders...)
//	    suite := bdd.NewSuite(t)
//	    webtestbdd.Register(suite, func() *webtest.Client {
//	        return webtest.New(t, app)
//	    })
//	    suite.Run()
//	}
//
// The Client factory is called ONCE PER SCENARIO so each scenario
// gets a fresh cookie jar and a clean request state. State that
// should persist across scenarios (seeded rows, cached files, etc.)
// belongs on the app itself, not the client.
package bdd

import (
	"strings"

	"github.com/jordanbrauer/hex/bdd"
	"github.com/jordanbrauer/hex/webtest"
)

// Register attaches the standard webtest step vocabulary to suite.
// factory is invoked once per scenario to build a fresh Client bound
// to the app under test.
//
// Register does not run the suite — callers call suite.Run() when
// they've added any additional app-specific steps.
func Register(suite *bdd.Suite, factory func() *webtest.Client) {
	// Client lifecycle. Every scenario gets a fresh one so cookies,
	// last-response state, and default headers reset cleanly. The
	// clientHolder lazily builds on the first step per scenario.
	current := &clientHolder{factory: factory}

	// Request steps.
	suite.AddStep(`^I visit "([^"]+)"$`, func(t bdd.StepTest, ctx bdd.Context, path string) {
		current.get().Get(path)
	})

	suite.AddStep(`^I get "([^"]+)"$`, func(t bdd.StepTest, ctx bdd.Context, path string) {
		current.get().Get(path)
	})

	suite.AddStep(`^I delete "([^"]+)"$`, func(t bdd.StepTest, ctx bdd.Context, path string) {
		current.get().Delete(path)
	})

	suite.AddStep(`^I send header "([^"]+)" = "([^"]*)"$`, func(t bdd.StepTest, ctx bdd.Context, name, value string) {
		current.get().Header(name, value)
	})

	// Response assertions.
	suite.AddStep(`^the response status should be (\d+)$`, func(t bdd.StepTest, ctx bdd.Context, code int) {
		current.get().StatusIs(code)
	})

	suite.AddStep(`^I should see "([^"]+)"$`, func(t bdd.StepTest, ctx bdd.Context, text string) {
		current.get().See(text)
	})

	suite.AddStep(`^I should not see "([^"]+)"$`, func(t bdd.StepTest, ctx bdd.Context, text string) {
		client := current.get()

		if strings.Contains(client.Body(), text) {
			t.Errorf("body unexpectedly contains %q", text)
		}
	})

	suite.AddStep(`^the response header "([^"]+)" should be "([^"]*)"$`, func(t bdd.StepTest, ctx bdd.Context, name, value string) {
		current.get().HeaderIs(name, value)
	})

	suite.AddStep(`^I should be redirected to "([^"]+)"$`, func(t bdd.StepTest, ctx bdd.Context, path string) {
		current.get().LocationIs(path)
	})

	// DOM assertions.
	suite.AddStep(`^"([^"]+)" should exist$`, func(t bdd.StepTest, ctx bdd.Context, selector string) {
		current.get().Find(selector).Exists()
	})

	suite.AddStep(`^"([^"]+)" should not exist$`, func(t bdd.StepTest, ctx bdd.Context, selector string) {
		current.get().Find(selector).DoesNotExist()
	})

	suite.AddStep(`^"([^"]+)" should have text "([^"]+)"$`, func(t bdd.StepTest, ctx bdd.Context, selector, text string) {
		current.get().Find(selector).HasText(text)
	})

	suite.AddStep(`^"([^"]+)" should have exact text "([^"]+)"$`, func(t bdd.StepTest, ctx bdd.Context, selector, text string) {
		current.get().Find(selector).HasExactText(text)
	})

	suite.AddStep(`^"([^"]+)" should have class "([^"]+)"$`, func(t bdd.StepTest, ctx bdd.Context, selector, class string) {
		current.get().Find(selector).HasClass(class)
	})

	suite.AddStep(`^"([^"]+)" should have attribute "([^"]+)"$`, func(t bdd.StepTest, ctx bdd.Context, selector, attr string) {
		current.get().Find(selector).HasAttribute(attr)
	})

	suite.AddStep(`^"([^"]+)" should have attribute "([^"]+)" = "([^"]*)"$`, func(t bdd.StepTest, ctx bdd.Context, selector, attr, value string) {
		current.get().Find(selector).HasAttributeValue(attr, value)
	})

	suite.AddStep(`^there should be (\d+) elements? matching "([^"]+)"$`, func(t bdd.StepTest, ctx bdd.Context, n int, selector string) {
		current.get().Find(selector).Count(n)
	})

	suite.AddStep(`^there should be no elements matching "([^"]+)"$`, func(t bdd.StepTest, ctx bdd.Context, selector string) {
		current.get().Find(selector).DoesNotExist()
	})
}

// clientHolder is the closure state that gives every step access
// to the current scenario's Client, rebuilt lazily via factory.
type clientHolder struct {
	factory func() *webtest.Client
	client  *webtest.Client
}

func (h *clientHolder) get() *webtest.Client {
	if h.client == nil {
		h.client = h.factory()
	}

	return h.client
}
