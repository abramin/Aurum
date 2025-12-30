package spending

import "github.com/cucumber/godog"

type spendingState struct {
	tenantID       string
	idempotencyKey string
	authID         string
	lastResponse   int
	lastError      string
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	state := &spendingState{}

	// Background
	ctx.Step(`^a tenant "([^"]*)"$`, state.aTenant)

	// Authorization creation
	ctx.Step(`^an idempotency key "([^"]*)"$`, state.anIdempotencyKey)
	ctx.Step(`^I create an authorization for (\d+\.\d+) ([A-Z]{3})$`, state.iCreateAnAuthorizationFor)
	ctx.Step(`^the authorization should be in "([^"]*)" state$`, state.theAuthorizationShouldBeInState)
	ctx.Step(`^repeating the request returns the same authorization$`, state.repeatingTheRequestReturnsTheSameAuthorization)

	// Given authorization in state
	ctx.Step(`^an authorization for (\d+\.\d+) ([A-Z]{3}) in "([^"]*)" state$`, state.anAuthorizationForInState)

	// Capture
	ctx.Step(`^I capture (\d+\.\d+) ([A-Z]{3})$`, state.iCapture)
	ctx.Step(`^the captured amount should be (\d+\.\d+) ([A-Z]{3})$`, state.theCapturedAmountShouldBe)

	// Rejection scenarios
	ctx.Step(`^I attempt to capture (\d+\.\d+) ([A-Z]{3})$`, state.iAttemptToCapture)
	ctx.Step(`^the capture should be rejected with "([^"]*)"$`, state.theCaptureShouldBeRejectedWith)

	// Spending limits
	ctx.Step(`^a card account with spending limit (\d+\.\d+) ([A-Z]{3})$`, state.aCardAccountWithSpendingLimit)
	ctx.Step(`^existing authorizations totaling (\d+\.\d+) ([A-Z]{3})$`, state.existingAuthorizationsTotaling)
	ctx.Step(`^I attempt to create an authorization for (\d+\.\d+) ([A-Z]{3})$`, state.iAttemptToCreateAnAuthorizationFor)
	ctx.Step(`^the authorization should be rejected with "([^"]*)"$`, state.theAuthorizationShouldBeRejectedWith)
}

// Background steps

func (s *spendingState) aTenant(tenantID string) error {
	s.tenantID = tenantID
	return godog.ErrPending
}

// Authorization creation steps

func (s *spendingState) anIdempotencyKey(key string) error {
	s.idempotencyKey = key
	return godog.ErrPending
}

func (s *spendingState) iCreateAnAuthorizationFor(amount float64, currency string) error {
	// TDD: POST /authorizations with amount and currency
	return godog.ErrPending
}

func (s *spendingState) theAuthorizationShouldBeInState(expectedState string) error {
	// TDD: Assert authorization state from response
	return godog.ErrPending
}

func (s *spendingState) repeatingTheRequestReturnsTheSameAuthorization() error {
	// TDD: Repeat POST /authorizations with same idempotency key
	return godog.ErrPending
}

// Given authorization steps

func (s *spendingState) anAuthorizationForInState(amount float64, currency, state string) error {
	// TDD: Create authorization and optionally capture to reach desired state
	return godog.ErrPending
}

// Capture steps

func (s *spendingState) iCapture(amount float64, currency string) error {
	// TDD: POST /authorizations/{id}/capture
	return godog.ErrPending
}

func (s *spendingState) theCapturedAmountShouldBe(amount float64, currency string) error {
	// TDD: Assert captured amount from response
	return godog.ErrPending
}

// Rejection steps

func (s *spendingState) iAttemptToCapture(amount float64, currency string) error {
	// TDD: POST /authorizations/{id}/capture expecting failure
	return godog.ErrPending
}

func (s *spendingState) theCaptureShouldBeRejectedWith(reason string) error {
	// TDD: Assert error response contains reason
	return godog.ErrPending
}

// Spending limit steps

func (s *spendingState) aCardAccountWithSpendingLimit(limit float64, currency string) error {
	// TDD: Set up card account with limit
	return godog.ErrPending
}

func (s *spendingState) existingAuthorizationsTotaling(total float64, currency string) error {
	// TDD: Create authorizations summing to total
	return godog.ErrPending
}

func (s *spendingState) iAttemptToCreateAnAuthorizationFor(amount float64, currency string) error {
	// TDD: POST /authorizations expecting limit rejection
	return godog.ErrPending
}

func (s *spendingState) theAuthorizationShouldBeRejectedWith(reason string) error {
	// TDD: Assert error response contains reason
	return godog.ErrPending
}
