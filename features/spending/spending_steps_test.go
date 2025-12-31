package spending

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	vo "aurum/internal/common/value_objects"
	"aurum/internal/spending/api"
	"aurum/internal/spending/application"
	"aurum/internal/spending/domain"
	"aurum/internal/spending/infrastructure"

	"github.com/cucumber/godog"
)

type spendingState struct {
	tenantID         string
	idempotencyKey   string
	authID           string
	lastResponse     *http.Response
	lastBody         map[string]any
	lastError        string
	server           *httptest.Server
	service          *application.SpendingService
	authRepo         *infrastructure.MemoryAuthorizationRepository
	cardAccountRepo  *infrastructure.MemoryCardAccountRepository
	idempotencyStore *infrastructure.MemoryIdempotencyStore
	cardAccountID    domain.CardAccountID
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	state := &spendingState{}

	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		// Setup fresh repositories and server for each scenario
		state.authRepo = infrastructure.NewMemoryAuthorizationRepository()
		state.cardAccountRepo = infrastructure.NewMemoryCardAccountRepository()
		state.idempotencyStore = infrastructure.NewMemoryIdempotencyStore()
		state.service = application.NewSpendingService(state.authRepo, state.cardAccountRepo, state.idempotencyStore)

		handler := api.NewHandler(state.service)
		mux := http.NewServeMux()
		handler.RegisterRoutes(mux)
		state.server = httptest.NewServer(mux)

		return ctx, nil
	})

	ctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		if state.server != nil {
			state.server.Close()
		}
		return ctx, nil
	})

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
	// Create a default card account for this tenant with a high limit
	s.cardAccountID = domain.NewCardAccountID()
	limit := vo.NewFromInt(10000, "EUR")
	tenantIDParsed := vo.MustParseTenantID(tenantID)
	cardAccount := domain.NewCardAccount(s.cardAccountID, tenantIDParsed, limit)
	return s.cardAccountRepo.Save(context.Background(), cardAccount)
}

// Authorization creation steps

func (s *spendingState) anIdempotencyKey(key string) error {
	s.idempotencyKey = key
	return nil
}

func (s *spendingState) iCreateAnAuthorizationFor(amount float64, currency string) error {
	body := map[string]any{
		"card_account_id": s.cardAccountID.String(),
		"amount": map[string]any{
			"value":    fmt.Sprintf("%.2f", amount),
			"currency": currency,
		},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", s.server.URL+"/authorizations", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", s.tenantID)
	req.Header.Set("Idempotency-Key", s.idempotencyKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	s.lastResponse = resp

	var respBody map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return err
	}
	s.lastBody = respBody

	if id, ok := respBody["id"].(string); ok {
		s.authID = id
	}

	return nil
}

func (s *spendingState) theAuthorizationShouldBeInState(expectedState string) error {
	state, ok := s.lastBody["state"].(string)
	if !ok {
		return fmt.Errorf("state not found in response")
	}
	expectedLower := strings.ToLower(expectedState)
	if state != expectedLower {
		return fmt.Errorf("expected state %q, got %q", expectedLower, state)
	}
	return nil
}

func (s *spendingState) repeatingTheRequestReturnsTheSameAuthorization() error {
	originalID := s.authID

	// Repeat the request with the same idempotency key
	body := map[string]any{
		"card_account_id": s.cardAccountID.String(),
		"amount": map[string]any{
			"value":    "100.00",
			"currency": "EUR",
		},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", s.server.URL+"/authorizations", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", s.tenantID)
	req.Header.Set("Idempotency-Key", s.idempotencyKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	var respBody map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return err
	}

	newID, ok := respBody["id"].(string)
	if !ok {
		return fmt.Errorf("id not found in response")
	}

	if newID != originalID {
		return fmt.Errorf("expected same authorization ID %q, got %q", originalID, newID)
	}

	return nil
}

// Given authorization steps

func (s *spendingState) anAuthorizationForInState(amount float64, currency, targetState string) error {
	// Create the authorization
	tenantID := vo.MustParseTenantID(s.tenantID)
	auth := domain.NewAuthorization(tenantID, s.cardAccountID, vo.NewFromInt(int64(amount), currency))
	s.authID = auth.ID().String()

	// Transition to target state if needed
	targetStateLower := strings.ToLower(targetState)
	switch targetStateLower {
	case "authorized":
		// Already in this state
	case "captured":
		if err := auth.Capture(vo.NewFromInt(int64(amount), currency)); err != nil {
			return err
		}
	case "reversed":
		if err := auth.Reverse(); err != nil {
			return err
		}
	case "expired":
		if err := auth.Expire(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown state: %s", targetState)
	}

	return s.authRepo.Save(context.Background(), auth)
}

// Capture steps

func (s *spendingState) iCapture(amount float64, currency string) error {
	body := map[string]any{
		"amount": map[string]any{
			"value":    fmt.Sprintf("%.2f", amount),
			"currency": currency,
		},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", s.server.URL+"/authorizations/"+s.authID+"/capture", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", s.tenantID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	s.lastResponse = resp

	var respBody map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return err
	}
	s.lastBody = respBody

	return nil
}

func (s *spendingState) theCapturedAmountShouldBe(amount float64, currency string) error {
	capturedAmount, ok := s.lastBody["captured_amount"].(map[string]any)
	if !ok {
		return fmt.Errorf("captured_amount not found in response")
	}

	value, ok := capturedAmount["value"].(string)
	if !ok {
		return fmt.Errorf("captured_amount.value not found")
	}

	expected := fmt.Sprintf("%.2f", amount)
	if value != expected {
		return fmt.Errorf("expected captured amount %s, got %s", expected, value)
	}

	curr, ok := capturedAmount["currency"].(string)
	if !ok {
		return fmt.Errorf("captured_amount.currency not found")
	}
	if curr != currency {
		return fmt.Errorf("expected currency %s, got %s", currency, curr)
	}

	return nil
}

// Rejection steps

func (s *spendingState) iAttemptToCapture(amount float64, currency string) error {
	// Same as iCapture, but we expect it might fail
	return s.iCapture(amount, currency)
}

func (s *spendingState) theCaptureShouldBeRejectedWith(reason string) error {
	if s.lastResponse.StatusCode >= 200 && s.lastResponse.StatusCode < 300 {
		return fmt.Errorf("expected rejection, but got status %d", s.lastResponse.StatusCode)
	}

	errorMsg, ok := s.lastBody["error"].(string)
	if !ok {
		return fmt.Errorf("error message not found in response")
	}

	if !strings.Contains(errorMsg, reason) {
		return fmt.Errorf("expected error containing %q, got %q", reason, errorMsg)
	}

	return nil
}

// Spending limit steps

func (s *spendingState) aCardAccountWithSpendingLimit(limit float64, currency string) error {
	s.cardAccountID = domain.NewCardAccountID()
	limitAmount := vo.NewFromInt(int64(limit), currency)
	tenantID := vo.MustParseTenantID(s.tenantID)
	cardAccount := domain.NewCardAccount(s.cardAccountID, tenantID, limitAmount)
	return s.cardAccountRepo.Save(context.Background(), cardAccount)
}

func (s *spendingState) existingAuthorizationsTotaling(total float64, currency string) error {
	// Get the card account and record the authorization
	tenantID := vo.MustParseTenantID(s.tenantID)
	cardAccount, err := s.cardAccountRepo.FindByID(context.Background(), tenantID, s.cardAccountID)
	if err != nil {
		return err
	}
	if cardAccount == nil {
		return fmt.Errorf("card account not found")
	}

	// Record the existing authorizations
	amount := vo.NewFromInt(int64(total), currency)
	if err := cardAccount.RecordAuthorization(amount); err != nil {
		return err
	}

	// Save the updated card account
	return s.cardAccountRepo.Save(context.Background(), cardAccount)
}

func (s *spendingState) iAttemptToCreateAnAuthorizationFor(amount float64, currency string) error {
	s.idempotencyKey = "attempt-key"
	return s.iCreateAnAuthorizationFor(amount, currency)
}

func (s *spendingState) theAuthorizationShouldBeRejectedWith(reason string) error {
	if s.lastResponse.StatusCode >= 200 && s.lastResponse.StatusCode < 300 {
		return fmt.Errorf("expected rejection, but got status %d", s.lastResponse.StatusCode)
	}

	errorMsg, ok := s.lastBody["error"].(string)
	if !ok {
		return fmt.Errorf("error message not found in response")
	}

	if !strings.Contains(errorMsg, reason) {
		return fmt.Errorf("expected error containing %q, got %q", reason, errorMsg)
	}

	return nil
}
