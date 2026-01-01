package domain_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
)

func TestCardAccount_AuthorizeAmount(t *testing.T) {
	tenantID := types.TenantID("tenant-1")
	limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)

	t.Run("successful authorization within limit", func(t *testing.T) {
		account := domain.NewCardAccount(tenantID, limit)

		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
		err := account.AuthorizeAmount(amount)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !account.RollingSpend().Equal(amount) {
			t.Errorf("expected rolling spend %s, got %s", amount.String(), account.RollingSpend().String())
		}
	})

	t.Run("multiple authorizations within limit", func(t *testing.T) {
		account := domain.NewCardAccount(tenantID, limit)

		amount := types.NewMoney(decimal.NewFromInt(300), types.CurrencyEUR)
		_ = account.AuthorizeAmount(amount)
		_ = account.AuthorizeAmount(amount)
		err := account.AuthorizeAmount(amount)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expectedSpend := types.NewMoney(decimal.NewFromInt(900), types.CurrencyEUR)
		if !account.RollingSpend().Equal(expectedSpend) {
			t.Errorf("expected rolling spend %s, got %s", expectedSpend.String(), account.RollingSpend().String())
		}
	})

	t.Run("authorization exceeds limit", func(t *testing.T) {
		account := domain.NewCardAccount(tenantID, limit)

		amount := types.NewMoney(decimal.NewFromInt(1500), types.CurrencyEUR)
		err := account.AuthorizeAmount(amount)

		if err != domain.ErrSpendingLimitExceeded {
			t.Errorf("expected ErrSpendingLimitExceeded, got %v", err)
		}
		if !account.RollingSpend().IsZero() {
			t.Errorf("expected rolling spend to be zero, got %s", account.RollingSpend().String())
		}
	})

	t.Run("cumulative authorization exceeds limit", func(t *testing.T) {
		account := domain.NewCardAccount(tenantID, limit)

		amount := types.NewMoney(decimal.NewFromInt(600), types.CurrencyEUR)
		_ = account.AuthorizeAmount(amount)

		err := account.AuthorizeAmount(amount) // Total would be 1200 > 1000

		if err != domain.ErrSpendingLimitExceeded {
			t.Errorf("expected ErrSpendingLimitExceeded, got %v", err)
		}
		// Rolling spend should remain at first authorization
		if !account.RollingSpend().Equal(amount) {
			t.Errorf("expected rolling spend %s, got %s", amount.String(), account.RollingSpend().String())
		}
	})

	t.Run("authorization with different currency fails", func(t *testing.T) {
		account := domain.NewCardAccount(tenantID, limit)

		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyUSD)
		err := account.AuthorizeAmount(amount)

		if err != domain.ErrCurrencyMismatch {
			t.Errorf("expected ErrCurrencyMismatch, got %v", err)
		}
	})

	t.Run("available limit decreases with authorizations", func(t *testing.T) {
		account := domain.NewCardAccount(tenantID, limit)

		amount := types.NewMoney(decimal.NewFromInt(400), types.CurrencyEUR)
		_ = account.AuthorizeAmount(amount)

		available := account.AvailableLimit()
		expected := types.NewMoney(decimal.NewFromInt(600), types.CurrencyEUR)
		if !available.Equal(expected) {
			t.Errorf("expected available limit %s, got %s", expected.String(), available.String())
		}
	})
}

func TestCardAccount_ReleaseAmount(t *testing.T) {
	tenantID := types.TenantID("tenant-1")
	limit := types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)

	t.Run("successful release", func(t *testing.T) {
		account := domain.NewCardAccount(tenantID, limit)

		authAmount := types.NewMoney(decimal.NewFromInt(500), types.CurrencyEUR)
		_ = account.AuthorizeAmount(authAmount)

		releaseAmount := types.NewMoney(decimal.NewFromInt(200), types.CurrencyEUR)
		err := account.ReleaseAmount(releaseAmount)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := types.NewMoney(decimal.NewFromInt(300), types.CurrencyEUR)
		if !account.RollingSpend().Equal(expected) {
			t.Errorf("expected rolling spend %s, got %s", expected.String(), account.RollingSpend().String())
		}
	})

	t.Run("release with different currency fails", func(t *testing.T) {
		account := domain.NewCardAccount(tenantID, limit)

		authAmount := types.NewMoney(decimal.NewFromInt(500), types.CurrencyEUR)
		_ = account.AuthorizeAmount(authAmount)

		releaseAmount := types.NewMoney(decimal.NewFromInt(200), types.CurrencyUSD)
		err := account.ReleaseAmount(releaseAmount)

		if err != domain.ErrCurrencyMismatch {
			t.Errorf("expected ErrCurrencyMismatch, got %v", err)
		}
	})
}
