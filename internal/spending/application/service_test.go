package application_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"

	"aurum/internal/common/types"
	"aurum/internal/spending/application"
	"aurum/internal/spending/domain"
	"aurum/internal/spending/infrastructure/memory"
)

func TestSpendingService_CreateAuthorization(t *testing.T) {
	ctx := context.Background()
	tenantID := types.TenantID("tenant-1")
	correlationID := types.NewCorrelationID()

	t.Run("successful authorization within limit", func(t *testing.T) {
		dataStore := memory.NewDataStore()
		service := application.NewSpendingService(dataStore)

		// First create a card account
		limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)
		_, err := service.CreateCardAccount(ctx, application.CreateCardAccountRequest{
			TenantID:      tenantID,
			SpendingLimit: limit,
		})
		if err != nil {
			t.Fatalf("failed to create card account: %v", err)
		}

		// Create authorization
		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		resp, err := service.CreateAuthorization(ctx, application.CreateAuthorizationRequest{
			TenantID:       tenantID,
			IdempotencyKey: "idem-1",
			Amount:         amount,
			MerchantRef:    "merchant-1",
			Reference:      "ref-1",
			CorrelationID:  correlationID,
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.AuthorizationID == "" {
			t.Error("expected authorization ID to be set")
		}
		if resp.Status != "authorized" {
			t.Errorf("expected status 'authorized', got %s", resp.Status)
		}
	})

	t.Run("authorization exceeds spending limit", func(t *testing.T) {
		dataStore := memory.NewDataStore()
		service := application.NewSpendingService(dataStore)

		// Create card account with small limit
		limit := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		_, err := service.CreateCardAccount(ctx, application.CreateCardAccountRequest{
			TenantID:      tenantID,
			SpendingLimit: limit,
		})
		if err != nil {
			t.Fatalf("failed to create card account: %v", err)
		}

		// Try to authorize more than limit
		amount := types.NewMoney(decimal.NewFromInt(500), types.CurrencyEUR)
		_, err = service.CreateAuthorization(ctx, application.CreateAuthorizationRequest{
			TenantID:       tenantID,
			IdempotencyKey: "idem-1",
			Amount:         amount,
			MerchantRef:    "merchant-1",
			Reference:      "ref-1",
			CorrelationID:  correlationID,
		})

		if err != domain.ErrSpendingLimitExceeded {
			t.Errorf("expected ErrSpendingLimitExceeded, got %v", err)
		}
	})

	t.Run("idempotency returns same response", func(t *testing.T) {
		dataStore := memory.NewDataStore()
		service := application.NewSpendingService(dataStore)

		// Create card account
		limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)
		_, err := service.CreateCardAccount(ctx, application.CreateCardAccountRequest{
			TenantID:      tenantID,
			SpendingLimit: limit,
		})
		if err != nil {
			t.Fatalf("failed to create card account: %v", err)
		}

		// Create authorization
		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		req := application.CreateAuthorizationRequest{
			TenantID:       tenantID,
			IdempotencyKey: "idem-same",
			Amount:         amount,
			MerchantRef:    "merchant-1",
			Reference:      "ref-1",
			CorrelationID:  correlationID,
		}

		resp1, err := service.CreateAuthorization(ctx, req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Call again with same idempotency key
		resp2, err := service.CreateAuthorization(ctx, req)
		if err != nil {
			t.Fatalf("expected no error on idempotent retry, got %v", err)
		}

		if resp1.AuthorizationID != resp2.AuthorizationID {
			t.Errorf("expected same authorization ID, got %s and %s", resp1.AuthorizationID, resp2.AuthorizationID)
		}
	})
}

func TestSpendingService_CaptureAuthorization(t *testing.T) {
	ctx := context.Background()
	tenantID := types.TenantID("tenant-1")
	correlationID := types.NewCorrelationID()

	t.Run("successful capture", func(t *testing.T) {
		dataStore := memory.NewDataStore()
		service := application.NewSpendingService(dataStore)

		// Setup: create card account and authorization
		limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)
		_, err := service.CreateCardAccount(ctx, application.CreateCardAccountRequest{
			TenantID:      tenantID,
			SpendingLimit: limit,
		})
		if err != nil {
			t.Fatalf("failed to create card account: %v", err)
		}

		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		authResp, err := service.CreateAuthorization(ctx, application.CreateAuthorizationRequest{
			TenantID:       tenantID,
			IdempotencyKey: "idem-auth",
			Amount:         amount,
			MerchantRef:    "merchant-1",
			Reference:      "ref-1",
			CorrelationID:  correlationID,
		})
		if err != nil {
			t.Fatalf("failed to create authorization: %v", err)
		}

		// Parse authorization ID
		authID, err := domain.ParseAuthorizationID(authResp.AuthorizationID)
		if err != nil {
			t.Fatalf("failed to parse authorization ID: %v", err)
		}

		// Capture
		captureResp, err := service.CaptureAuthorization(ctx, application.CaptureAuthorizationRequest{
			TenantID:        tenantID,
			AuthorizationID: authID,
			IdempotencyKey:  "idem-capture",
			Amount:          amount,
			CorrelationID:   correlationID,
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if captureResp.Status != "captured" {
			t.Errorf("expected status 'captured', got %s", captureResp.Status)
		}
	})

	t.Run("cannot capture more than authorized", func(t *testing.T) {
		dataStore := memory.NewDataStore()
		service := application.NewSpendingService(dataStore)

		// Setup
		limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)
		_, _ = service.CreateCardAccount(ctx, application.CreateCardAccountRequest{
			TenantID:      tenantID,
			SpendingLimit: limit,
		})

		authAmount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		authResp, _ := service.CreateAuthorization(ctx, application.CreateAuthorizationRequest{
			TenantID:       tenantID,
			IdempotencyKey: "idem-auth",
			Amount:         authAmount,
			MerchantRef:    "merchant-1",
			Reference:      "ref-1",
			CorrelationID:  correlationID,
		})

		authID, _ := domain.ParseAuthorizationID(authResp.AuthorizationID)

		// Try to capture more than authorized
		captureAmount := types.NewMoney(decimal.NewFromInt(150), types.CurrencyEUR)
		_, err := service.CaptureAuthorization(ctx, application.CaptureAuthorizationRequest{
			TenantID:        tenantID,
			AuthorizationID: authID,
			IdempotencyKey:  "idem-capture",
			Amount:          captureAmount,
			CorrelationID:   correlationID,
		})

		if err != domain.ErrExceedsAuthorizedAmount {
			t.Errorf("expected ErrExceedsAuthorizedAmount, got %v", err)
		}
	})

	t.Run("cannot capture twice", func(t *testing.T) {
		dataStore := memory.NewDataStore()
		service := application.NewSpendingService(dataStore)

		// Setup
		limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)
		_, _ = service.CreateCardAccount(ctx, application.CreateCardAccountRequest{
			TenantID:      tenantID,
			SpendingLimit: limit,
		})

		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		authResp, _ := service.CreateAuthorization(ctx, application.CreateAuthorizationRequest{
			TenantID:       tenantID,
			IdempotencyKey: "idem-auth",
			Amount:         amount,
			MerchantRef:    "merchant-1",
			Reference:      "ref-1",
			CorrelationID:  correlationID,
		})

		authID, _ := domain.ParseAuthorizationID(authResp.AuthorizationID)

		// First capture
		_, _ = service.CaptureAuthorization(ctx, application.CaptureAuthorizationRequest{
			TenantID:        tenantID,
			AuthorizationID: authID,
			IdempotencyKey:  "idem-capture-1",
			Amount:          amount,
			CorrelationID:   correlationID,
		})

		// Second capture with different idempotency key
		_, err := service.CaptureAuthorization(ctx, application.CaptureAuthorizationRequest{
			TenantID:        tenantID,
			AuthorizationID: authID,
			IdempotencyKey:  "idem-capture-2",
			Amount:          amount,
			CorrelationID:   correlationID,
		})

		if err != domain.ErrAlreadyCaptured {
			t.Errorf("expected ErrAlreadyCaptured, got %v", err)
		}
	})
}
