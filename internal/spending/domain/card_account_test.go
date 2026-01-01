package domain_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
)

// CardAccountSuite tests CardAccount aggregate behavior.
//
// Justification for unit tests: These tests provide fast feedback on spending limit
// invariants. The same behaviors are covered at integration level via feature files,
// but unit tests catch regressions before slower integration tests run.
type CardAccountSuite struct {
	suite.Suite
	tenantID types.TenantID
	limit    types.Money
}

func TestCardAccountSuite(t *testing.T) {
	suite.Run(t, new(CardAccountSuite))
}

func (s *CardAccountSuite) SetupTest() {
	s.tenantID = types.TenantID("tenant-1")
	s.limit = types.NewMoney(decimal.NewFromInt(1000), types.CurrencyEUR)
}

func (s *CardAccountSuite) newCardAccount() *domain.CardAccount {
	account, err := domain.NewCardAccount(s.tenantID, s.limit)
	s.Require().NoError(err)
	return account
}

// TestSpendLimitEnforcement validates that authorizations respect the spending limit.
func (s *CardAccountSuite) TestSpendLimitEnforcement() {
	s.Run("authorizes single amount within limit", func() {
		account := s.newCardAccount()
		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)

		err := account.AuthorizeAmount(amount)

		s.Require().NoError(err)
		s.True(account.RollingSpend().Equal(amount))
	})

	s.Run("authorizes multiple amounts within cumulative limit", func() {
		account := s.newCardAccount()
		amount := types.NewMoney(decimal.NewFromInt(300), types.CurrencyEUR)

		_ = account.AuthorizeAmount(amount)
		_ = account.AuthorizeAmount(amount)
		err := account.AuthorizeAmount(amount)

		s.Require().NoError(err)
		expectedSpend := types.NewMoney(decimal.NewFromInt(900), types.CurrencyEUR)
		s.True(account.RollingSpend().Equal(expectedSpend))
	})

	s.Run("rejects single authorization exceeding limit", func() {
		account := s.newCardAccount()
		amount := types.NewMoney(decimal.NewFromInt(1500), types.CurrencyEUR)

		err := account.AuthorizeAmount(amount)

		s.ErrorIs(err, domain.ErrSpendingLimitExceeded)
		s.True(account.RollingSpend().IsZero(), "rolling spend should remain zero on rejection")
	})

	s.Run("rejects cumulative authorization exceeding limit", func() {
		account := s.newCardAccount()
		amount := types.NewMoney(decimal.NewFromInt(600), types.CurrencyEUR)
		_ = account.AuthorizeAmount(amount)

		err := account.AuthorizeAmount(amount) // Total would be 1200 > 1000

		s.ErrorIs(err, domain.ErrSpendingLimitExceeded)
		s.True(account.RollingSpend().Equal(amount), "rolling spend should remain at first authorization")
	})

	s.Run("rejects authorization with currency mismatch", func() {
		account := s.newCardAccount()
		amount := types.NewMoney(decimal.NewFromInt(100), types.CurrencyUSD)

		err := account.AuthorizeAmount(amount)

		s.ErrorIs(err, domain.ErrCurrencyMismatch)
	})

	s.Run("decreases available limit with authorizations", func() {
		account := s.newCardAccount()
		amount := types.NewMoney(decimal.NewFromInt(400), types.CurrencyEUR)

		_ = account.AuthorizeAmount(amount)

		available := account.AvailableLimit()
		expected := types.NewMoney(decimal.NewFromInt(600), types.CurrencyEUR)
		s.True(available.Equal(expected))
	})
}

// TestAmountRelease validates that reversals restore available spending capacity.
func (s *CardAccountSuite) TestAmountRelease() {
	s.Run("releases amount back to available limit", func() {
		account := s.newCardAccount()
		authAmount := types.NewMoney(decimal.NewFromInt(500), types.CurrencyEUR)
		_ = account.AuthorizeAmount(authAmount)

		releaseAmount := types.NewMoney(decimal.NewFromInt(200), types.CurrencyEUR)
		err := account.ReleaseAmount(releaseAmount)

		s.Require().NoError(err)
		expected := types.NewMoney(decimal.NewFromInt(300), types.CurrencyEUR)
		s.True(account.RollingSpend().Equal(expected))
	})

	s.Run("rejects release with currency mismatch", func() {
		account := s.newCardAccount()
		authAmount := types.NewMoney(decimal.NewFromInt(500), types.CurrencyEUR)
		_ = account.AuthorizeAmount(authAmount)

		releaseAmount := types.NewMoney(decimal.NewFromInt(200), types.CurrencyUSD)
		err := account.ReleaseAmount(releaseAmount)

		s.ErrorIs(err, domain.ErrCurrencyMismatch)
	})
}
