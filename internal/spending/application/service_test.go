package application_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"

	"aurum/internal/common/types"
	"aurum/internal/spending/application"
	"aurum/internal/spending/domain"
	"aurum/internal/spending/infrastructure/memory"
)

// SpendingServiceSuite tests the SpendingService application layer.
//
// Justification: These tests validate orchestration concerns (idempotency key handling,
// repository coordination) that span multiple domain objects. This layer is the natural
// integration point before HTTP/feature tests.
type SpendingServiceSuite struct {
	suite.Suite
	ctx           context.Context
	tenantID      types.TenantID
	correlationID types.CorrelationID
}

func TestSpendingServiceSuite(t *testing.T) {
	suite.Run(t, new(SpendingServiceSuite))
}

func (s *SpendingServiceSuite) SetupTest() {
	s.ctx = context.Background()
	s.tenantID = types.TenantID("tenant-1")
	s.correlationID = types.NewCorrelationID()
}

func (s *SpendingServiceSuite) newService() *application.SpendingService {
	dataStore := memory.NewDataStore()
	return application.NewSpendingService(dataStore)
}

func (s *SpendingServiceSuite) createCardAccount(service *application.SpendingService, limit types.Money) {
	_, err := service.CreateCardAccount(s.ctx, application.CreateCardAccountRequest{
		TenantID:      s.tenantID,
		SpendingLimit: limit,
	})
	s.Require().NoError(err)
}

// TestAuthorizationWorkflow validates the end-to-end authorization creation flow.
func (s *SpendingServiceSuite) TestAuthorizationWorkflow() {
	s.Run("creates authorization within spending limit", func() {
		service := s.newService()
		limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)
		s.createCardAccount(service, limit)

		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		resp, err := service.CreateAuthorization(s.ctx, application.CreateAuthorizationRequest{
			TenantID:       s.tenantID,
			IdempotencyKey: "idem-1",
			Amount:         amount,
			MerchantRef:    "merchant-1",
			Reference:      "ref-1",
			CorrelationID:  s.correlationID,
		})

		s.Require().NoError(err)
		s.NotEmpty(resp.AuthorizationID)
		s.Equal("authorized", resp.Status)
	})

	s.Run("rejects authorization exceeding spending limit", func() {
		service := s.newService()
		limit := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		s.createCardAccount(service, limit)

		amount := types.NewMoney(decimal.NewFromInt(500), types.CurrencyEUR)
		_, err := service.CreateAuthorization(s.ctx, application.CreateAuthorizationRequest{
			TenantID:       s.tenantID,
			IdempotencyKey: "idem-1",
			Amount:         amount,
			MerchantRef:    "merchant-1",
			Reference:      "ref-1",
			CorrelationID:  s.correlationID,
		})

		s.ErrorIs(err, domain.ErrSpendingLimitExceeded)
	})
}

// TestCaptureWorkflow validates the capture lifecycle after authorization.
func (s *SpendingServiceSuite) TestCaptureWorkflow() {
	s.Run("captures authorized amount", func() {
		service := s.newService()
		limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)
		s.createCardAccount(service, limit)

		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		authResp, err := service.CreateAuthorization(s.ctx, application.CreateAuthorizationRequest{
			TenantID:       s.tenantID,
			IdempotencyKey: "idem-auth",
			Amount:         amount,
			MerchantRef:    "merchant-1",
			Reference:      "ref-1",
			CorrelationID:  s.correlationID,
		})
		s.Require().NoError(err)

		authID, err := domain.ParseAuthorizationID(authResp.AuthorizationID)
		s.Require().NoError(err)

		captureResp, err := service.CaptureAuthorization(s.ctx, application.CaptureAuthorizationRequest{
			TenantID:        s.tenantID,
			AuthorizationID: authID,
			IdempotencyKey:  "idem-capture",
			Amount:          amount,
			CorrelationID:   s.correlationID,
		})

		s.Require().NoError(err)
		s.Equal("captured", captureResp.Status)
	})

	s.Run("rejects capture exceeding authorized amount", func() {
		service := s.newService()
		limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)
		s.createCardAccount(service, limit)

		authAmount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		authResp, _ := service.CreateAuthorization(s.ctx, application.CreateAuthorizationRequest{
			TenantID:       s.tenantID,
			IdempotencyKey: "idem-auth",
			Amount:         authAmount,
			MerchantRef:    "merchant-1",
			Reference:      "ref-1",
			CorrelationID:  s.correlationID,
		})

		authID, _ := domain.ParseAuthorizationID(authResp.AuthorizationID)

		captureAmount := types.NewMoney(decimal.NewFromInt(150), types.CurrencyEUR)
		_, err := service.CaptureAuthorization(s.ctx, application.CaptureAuthorizationRequest{
			TenantID:        s.tenantID,
			AuthorizationID: authID,
			IdempotencyKey:  "idem-capture",
			Amount:          captureAmount,
			CorrelationID:   s.correlationID,
		})

		s.ErrorIs(err, domain.ErrExceedsAuthorizedAmount)
	})

	s.Run("rejects double capture", func() {
		service := s.newService()
		limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)
		s.createCardAccount(service, limit)

		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		authResp, _ := service.CreateAuthorization(s.ctx, application.CreateAuthorizationRequest{
			TenantID:       s.tenantID,
			IdempotencyKey: "idem-auth",
			Amount:         amount,
			MerchantRef:    "merchant-1",
			Reference:      "ref-1",
			CorrelationID:  s.correlationID,
		})

		authID, _ := domain.ParseAuthorizationID(authResp.AuthorizationID)

		// First capture
		_, _ = service.CaptureAuthorization(s.ctx, application.CaptureAuthorizationRequest{
			TenantID:        s.tenantID,
			AuthorizationID: authID,
			IdempotencyKey:  "idem-capture-1",
			Amount:          amount,
			CorrelationID:   s.correlationID,
		})

		// Second capture with different idempotency key
		_, err := service.CaptureAuthorization(s.ctx, application.CaptureAuthorizationRequest{
			TenantID:        s.tenantID,
			AuthorizationID: authID,
			IdempotencyKey:  "idem-capture-2",
			Amount:          amount,
			CorrelationID:   s.correlationID,
		})

		s.ErrorIs(err, domain.ErrAlreadyCaptured)
	})
}

// TestIdempotency validates that repeated requests with same idempotency key return same result.
func (s *SpendingServiceSuite) TestIdempotency() {
	s.Run("returns same authorization for duplicate request", func() {
		service := s.newService()
		limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)
		s.createCardAccount(service, limit)

		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		req := application.CreateAuthorizationRequest{
			TenantID:       s.tenantID,
			IdempotencyKey: "idem-same",
			Amount:         amount,
			MerchantRef:    "merchant-1",
			Reference:      "ref-1",
			CorrelationID:  s.correlationID,
		}

		resp1, err := service.CreateAuthorization(s.ctx, req)
		s.Require().NoError(err)

		resp2, err := service.CreateAuthorization(s.ctx, req)
		s.Require().NoError(err)

		s.Equal(resp1.AuthorizationID, resp2.AuthorizationID)
	})
}
