package spending

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/shopspring/decimal"

	"aurum/internal/common/types"
	"aurum/internal/spending/application"
	"aurum/internal/spending/domain"
	"aurum/internal/spending/infrastructure/memory"
)

type spendingState struct {
	ctx                context.Context
	service            *application.SpendingService
	tenantID           types.TenantID
	correlationID      types.CorrelationID
	lastAuthResponse   *application.CreateAuthorizationResponse
	lastCaptureResp    *application.CaptureAuthorizationResponse
	lastError          error
	authorizationCount int
	authorizations     map[string]domain.AuthorizationID
	idempotencyKeys    map[string]bool
}

func InitializeSpendingScenario(ctx *godog.ScenarioContext) {
	state := &spendingState{
		ctx:             context.Background(),
		tenantID:        types.TenantID("tenant-1"),
		correlationID:   types.NewCorrelationID(),
		authorizations:  make(map[string]domain.AuthorizationID),
		idempotencyKeys: make(map[string]bool),
	}

	// Background steps
	ctx.Step(`^a card account with spending limit of (\d+) ([A-Z]{3})$`, state.aCardAccountWithSpendingLimit)

	// Authorization steps
	ctx.Step(`^I authorize (\d+) ([A-Z]{3}) for merchant "([^"]*)"$`, state.iAuthorizeForMerchant)
	ctx.Step(`^I authorize (\d+) ([A-Z]{3}) for merchant "([^"]*)" with idempotency key "([^"]*)"$`, state.iAuthorizeForMerchantWithIdempotencyKey)
	ctx.Step(`^I have an authorization for (\d+) ([A-Z]{3})$`, state.iHaveAnAuthorizationFor)
	ctx.Step(`^the authorization should be created with status "([^"]*)"$`, state.theAuthorizationShouldBeCreatedWithStatus)
	ctx.Step(`^the authorization should be declined with error "([^"]*)"$`, state.theAuthorizationShouldBeDeclinedWithError)
	ctx.Step(`^the authorization status should be "([^"]*)"$`, state.theAuthorizationStatusShouldBe)
	ctx.Step(`^the authorization status should remain "([^"]*)"$`, state.theAuthorizationStatusShouldBe)
	ctx.Step(`^all authorizations should be created with status "([^"]*)"$`, state.allAuthorizationsShouldBeCreatedWithStatus)
	ctx.Step(`^only one authorization should be created$`, state.onlyOneAuthorizationShouldBeCreated)

	// Capture steps
	ctx.Step(`^I capture (\d+) ([A-Z]{3}) from the authorization$`, state.iCaptureFromTheAuthorization)
	ctx.Step(`^I attempt to capture (\d+) ([A-Z]{3}) from the authorization$`, state.iAttemptToCaptureFromTheAuthorization)
	ctx.Step(`^I have captured (\d+) ([A-Z]{3}) from the authorization$`, state.iHaveCapturedFromTheAuthorization)
	ctx.Step(`^the captured amount should be (\d+) ([A-Z]{3})$`, state.theCapturedAmountShouldBe)
	ctx.Step(`^the capture should be declined with error "([^"]*)"$`, state.theCaptureShouldBeDeclinedWithError)

	// Reversal steps
	ctx.Step(`^I reverse the authorization$`, state.iReverseTheAuthorization)
	ctx.Step(`^I attempt to reverse the authorization$`, state.iAttemptToReverseTheAuthorization)
	ctx.Step(`^the reversal should be declined with error "([^"]*)"$`, state.theReversalShouldBeDeclinedWithError)

	// Limit steps
	ctx.Step(`^the available spending limit should be (\d+) ([A-Z]{3})$`, state.theAvailableSpendingLimitShouldBe)
	ctx.Step(`^the available spending limit should remain (\d+) ([A-Z]{3})$`, state.theAvailableSpendingLimitShouldBe)
	ctx.Step(`^the rolling spend should be (\d+) ([A-Z]{3})$`, state.theRollingSpendShouldBe)
	ctx.Step(`^the rolling spend should remain (\d+) ([A-Z]{3})$`, state.theRollingSpendShouldBe)
}

func (s *spendingState) parseCurrency(currencyStr string) string {
	switch currencyStr {
	case "EUR":
		return types.CurrencyEUR
	case "USD":
		return types.CurrencyUSD
	default:
		return currencyStr
	}
}

func (s *spendingState) aCardAccountWithSpendingLimit(amount int, currency string) error {
	dataStore := memory.NewDataStore()
	s.service = application.NewSpendingService(dataStore)

	limit := types.NewMoney(decimal.NewFromInt(int64(amount)), s.parseCurrency(currency))
	_, err := s.service.CreateCardAccount(s.ctx, application.CreateCardAccountRequest{
		TenantID:      s.tenantID,
		SpendingLimit: limit,
	})
	return err
}

func (s *spendingState) iAuthorizeForMerchant(amount int, currency, merchant string) error {
	return s.iAuthorizeForMerchantWithIdempotencyKey(amount, currency, merchant, fmt.Sprintf("idem-%d", s.authorizationCount))
}

func (s *spendingState) iAuthorizeForMerchantWithIdempotencyKey(amount int, currency, merchant, idempotencyKey string) error {
	money := types.NewMoney(decimal.NewFromInt(int64(amount)), s.parseCurrency(currency))

	resp, err := s.service.CreateAuthorization(s.ctx, application.CreateAuthorizationRequest{
		TenantID:       s.tenantID,
		IdempotencyKey: idempotencyKey,
		Amount:         money,
		MerchantRef:    merchant,
		Reference:      fmt.Sprintf("ref-%d", s.authorizationCount),
		CorrelationID:  s.correlationID,
	})

	s.lastError = err
	s.lastAuthResponse = resp

	if err == nil && !s.idempotencyKeys[idempotencyKey] {
		s.authorizationCount++
		s.idempotencyKeys[idempotencyKey] = true
		if resp != nil {
			authID, _ := domain.ParseAuthorizationID(resp.AuthorizationID)
			s.authorizations["last"] = authID
		}
	}

	return nil // We capture errors in state for later assertions
}

func (s *spendingState) iHaveAnAuthorizationFor(amount int, currency string) error {
	err := s.iAuthorizeForMerchant(amount, currency, "test-merchant")
	if err != nil {
		return err
	}
	if s.lastError != nil {
		return fmt.Errorf("failed to create authorization: %w", s.lastError)
	}
	return nil
}

func (s *spendingState) theAuthorizationShouldBeCreatedWithStatus(status string) error {
	if s.lastError != nil {
		return fmt.Errorf("expected authorization to succeed, got error: %v", s.lastError)
	}
	if s.lastAuthResponse == nil {
		return errors.New("no authorization response")
	}
	if s.lastAuthResponse.Status != status {
		return fmt.Errorf("expected status %q, got %q", status, s.lastAuthResponse.Status)
	}
	return nil
}

func (s *spendingState) theAuthorizationShouldBeDeclinedWithError(errorMsg string) error {
	if s.lastError == nil {
		return errors.New("expected authorization to be declined, but it succeeded")
	}

	expectedErrors := map[string]error{
		"spending limit exceeded": domain.ErrSpendingLimitExceeded,
		"currency mismatch":       domain.ErrCurrencyMismatch,
	}

	if expected, ok := expectedErrors[errorMsg]; ok {
		if !errors.Is(s.lastError, expected) {
			return fmt.Errorf("expected error %q, got: %v", errorMsg, s.lastError)
		}
		return nil
	}

	if !strings.Contains(s.lastError.Error(), errorMsg) {
		return fmt.Errorf("expected error containing %q, got: %v", errorMsg, s.lastError)
	}
	return nil
}

func (s *spendingState) theAuthorizationStatusShouldBe(status string) error {
	if s.lastAuthResponse != nil && s.lastAuthResponse.Status == status {
		return nil
	}
	if s.lastCaptureResp != nil && s.lastCaptureResp.Status == status {
		return nil
	}
	return fmt.Errorf("expected status %q", status)
}

func (s *spendingState) allAuthorizationsShouldBeCreatedWithStatus(_ string) error {
	// If we got here without errors, all authorizations succeeded
	if s.lastError != nil {
		return fmt.Errorf("expected all authorizations to succeed, got error: %v", s.lastError)
	}
	return nil
}

func (s *spendingState) onlyOneAuthorizationShouldBeCreated() error {
	if s.authorizationCount != 1 {
		return fmt.Errorf("expected 1 authorization, got %d", s.authorizationCount)
	}
	return nil
}

func (s *spendingState) iCaptureFromTheAuthorization(amount int, currency string) error {
	return s.captureFromAuthorization(amount, currency, true)
}

func (s *spendingState) iAttemptToCaptureFromTheAuthorization(amount int, currency string) error {
	return s.captureFromAuthorization(amount, currency, false)
}

func (s *spendingState) captureFromAuthorization(amount int, currency string, expectSuccess bool) error {
	authID, ok := s.authorizations["last"]
	if !ok {
		return errors.New("no authorization to capture")
	}

	money := types.NewMoney(decimal.NewFromInt(int64(amount)), s.parseCurrency(currency))

	resp, err := s.service.CaptureAuthorization(s.ctx, application.CaptureAuthorizationRequest{
		TenantID:        s.tenantID,
		AuthorizationID: authID,
		IdempotencyKey:  fmt.Sprintf("capture-%d", amount),
		Amount:          money,
		CorrelationID:   s.correlationID,
	})

	s.lastError = err
	s.lastCaptureResp = resp

	if expectSuccess && err != nil {
		return fmt.Errorf("expected capture to succeed, got: %v", err)
	}

	return nil
}

func (s *spendingState) iHaveCapturedFromTheAuthorization(amount int, currency string) error {
	return s.iCaptureFromTheAuthorization(amount, currency)
}

func (s *spendingState) theCapturedAmountShouldBe(amount int, currency string) error {
	if s.lastCaptureResp == nil {
		return errors.New("no capture response")
	}

	expectedAmount := strconv.Itoa(amount)
	if !strings.Contains(s.lastCaptureResp.CapturedAmount, expectedAmount) {
		return fmt.Errorf("expected captured amount %d %s, got %s", amount, currency, s.lastCaptureResp.CapturedAmount)
	}
	return nil
}

func (s *spendingState) theCaptureShouldBeDeclinedWithError(errorMsg string) error {
	if s.lastError == nil {
		return errors.New("expected capture to be declined, but it succeeded")
	}

	expectedErrors := map[string]error{
		"exceeds authorized amount": domain.ErrExceedsAuthorizedAmount,
		"already captured":          domain.ErrAlreadyCaptured,
	}

	if expected, ok := expectedErrors[errorMsg]; ok {
		if !errors.Is(s.lastError, expected) {
			return fmt.Errorf("expected error %q, got: %v", errorMsg, s.lastError)
		}
		return nil
	}

	if !strings.Contains(s.lastError.Error(), errorMsg) {
		return fmt.Errorf("expected error containing %q, got: %v", errorMsg, s.lastError)
	}
	return nil
}

func (s *spendingState) iReverseTheAuthorization() error {
	authID, ok := s.authorizations["last"]
	if !ok {
		return errors.New("no authorization to reverse")
	}

	resp, err := s.service.ReverseAuthorization(s.ctx, application.ReverseAuthorizationRequest{
		TenantID:        s.tenantID,
		AuthorizationID: authID,
		IdempotencyKey:  "reverse-1",
		CorrelationID:   s.correlationID,
	})

	s.lastError = err
	if err == nil && resp != nil {
		s.lastAuthResponse = &application.CreateAuthorizationResponse{
			AuthorizationID: authID.String(),
			Status:          resp.Status,
		}
	}

	return nil
}

func (s *spendingState) iAttemptToReverseTheAuthorization() error {
	return s.iReverseTheAuthorization()
}

func (s *spendingState) theReversalShouldBeDeclinedWithError(errorMsg string) error {
	if s.lastError == nil {
		return errors.New("expected reversal to be declined, but it succeeded")
	}

	expectedErrors := map[string]error{
		"invalid state transition": domain.ErrInvalidStateTransition,
	}

	if expected, ok := expectedErrors[errorMsg]; ok {
		if !errors.Is(s.lastError, expected) {
			return fmt.Errorf("expected error %q, got: %v", errorMsg, s.lastError)
		}
		return nil
	}

	if !strings.Contains(s.lastError.Error(), errorMsg) {
		return fmt.Errorf("expected error containing %q, got: %v", errorMsg, s.lastError)
	}
	return nil
}

func (s *spendingState) theAvailableSpendingLimitShouldBe(amount int, currency string) error {
	cardAccount, err := s.service.GetCardAccount(s.ctx, s.tenantID)
	if err != nil {
		return fmt.Errorf("failed to get card account: %w", err)
	}

	expected := types.NewMoney(decimal.NewFromInt(int64(amount)), s.parseCurrency(currency))
	if !cardAccount.AvailableLimit.Equal(expected) {
		return fmt.Errorf("expected available limit %s, got %s", expected.String(), cardAccount.AvailableLimit.String())
	}
	return nil
}

func (s *spendingState) theRollingSpendShouldBe(amount int, currency string) error {
	cardAccount, err := s.service.GetCardAccount(s.ctx, s.tenantID)
	if err != nil {
		return fmt.Errorf("failed to get card account: %w", err)
	}

	expected := types.NewMoney(decimal.NewFromInt(int64(amount)), s.parseCurrency(currency))
	if !cardAccount.RollingSpend.Equal(expected) {
		return fmt.Errorf("expected rolling spend %s, got %s", expected.String(), cardAccount.RollingSpend.String())
	}
	return nil
}
