package domain_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
)

func TestAuthorization_Capture(t *testing.T) {
	tenantID := types.TenantID("tenant-1")
	cardAccountID := domain.NewCardAccountID()
	amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)

	t.Run("successful capture", func(t *testing.T) {
		auth := domain.NewAuthorization(tenantID, cardAccountID, amount, "merchant-1", "ref-1")

		captureAmount := types.NewMoney(decimal.NewFromInt(50), types.CurrencyEUR)
		err := auth.Capture(captureAmount)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if auth.State() != domain.AuthorizationStateCaptured {
			t.Errorf("expected state %s, got %s", domain.AuthorizationStateCaptured, auth.State())
		}
		if !auth.CapturedAmount().Equal(captureAmount) {
			t.Errorf("expected captured amount %s, got %s", captureAmount.String(), auth.CapturedAmount().String())
		}
	})

	t.Run("capture full amount", func(t *testing.T) {
		auth := domain.NewAuthorization(tenantID, cardAccountID, amount, "merchant-1", "ref-1")

		err := auth.Capture(amount)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !auth.CapturedAmount().Equal(amount) {
			t.Errorf("expected captured amount %s, got %s", amount.String(), auth.CapturedAmount().String())
		}
	})

	t.Run("cannot capture more than authorized", func(t *testing.T) {
		auth := domain.NewAuthorization(tenantID, cardAccountID, amount, "merchant-1", "ref-1")

		captureAmount := types.NewMoney(decimal.NewFromInt(150), types.CurrencyEUR)
		err := auth.Capture(captureAmount)

		if err != domain.ErrExceedsAuthorizedAmount {
			t.Errorf("expected ErrExceedsAuthorizedAmount, got %v", err)
		}
		if auth.State() != domain.AuthorizationStateAuthorized {
			t.Errorf("expected state %s, got %s", domain.AuthorizationStateAuthorized, auth.State())
		}
	})

	t.Run("cannot capture twice", func(t *testing.T) {
		auth := domain.NewAuthorization(tenantID, cardAccountID, amount, "merchant-1", "ref-1")

		captureAmount := types.NewMoney(decimal.NewFromInt(50), types.CurrencyEUR)
		_ = auth.Capture(captureAmount)

		err := auth.Capture(captureAmount)

		if err != domain.ErrAlreadyCaptured {
			t.Errorf("expected ErrAlreadyCaptured, got %v", err)
		}
	})

	t.Run("cannot capture with different currency", func(t *testing.T) {
		auth := domain.NewAuthorization(tenantID, cardAccountID, amount, "merchant-1", "ref-1")

		captureAmount := types.NewMoney(decimal.NewFromInt(50), types.CurrencyUSD)
		err := auth.Capture(captureAmount)

		if err != domain.ErrCurrencyMismatch {
			t.Errorf("expected ErrCurrencyMismatch, got %v", err)
		}
	})

	t.Run("cannot capture reversed authorization", func(t *testing.T) {
		auth := domain.NewAuthorization(tenantID, cardAccountID, amount, "merchant-1", "ref-1")
		_ = auth.Reverse()

		captureAmount := types.NewMoney(decimal.NewFromInt(50), types.CurrencyEUR)
		err := auth.Capture(captureAmount)

		if err != domain.ErrInvalidStateTransition {
			t.Errorf("expected ErrInvalidStateTransition, got %v", err)
		}
	})
}

func TestAuthorization_Reverse(t *testing.T) {
	tenantID := types.TenantID("tenant-1")
	cardAccountID := domain.NewCardAccountID()
	amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)

	t.Run("successful reverse", func(t *testing.T) {
		auth := domain.NewAuthorization(tenantID, cardAccountID, amount, "merchant-1", "ref-1")

		err := auth.Reverse()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if auth.State() != domain.AuthorizationStateReversed {
			t.Errorf("expected state %s, got %s", domain.AuthorizationStateReversed, auth.State())
		}
	})

	t.Run("cannot reverse captured authorization", func(t *testing.T) {
		auth := domain.NewAuthorization(tenantID, cardAccountID, amount, "merchant-1", "ref-1")
		_ = auth.Capture(amount)

		err := auth.Reverse()

		if err != domain.ErrInvalidStateTransition {
			t.Errorf("expected ErrInvalidStateTransition, got %v", err)
		}
	})
}
