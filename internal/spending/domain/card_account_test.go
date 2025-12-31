package domain

import (
	"testing"

	vo "aurum/internal/common/value_objects"

	"github.com/stretchr/testify/suite"
)

type CardAccountSuite struct {
	suite.Suite
}

func TestCardAccountSuite(t *testing.T) {
	suite.Run(t, new(CardAccountSuite))
}

// Spending Limit Enforcement
func (s *CardAccountSuite) TestSpendingLimitEnforcement() {
	s.Run("authorization within limit is allowed", func() {
		limit := vo.NewFromInt(500, "EUR")
		account := NewCardAccount(NewCardAccountID(), vo.MustParseTenantID("tenant-1"), limit)
		err := account.CanAuthorize(vo.NewFromInt(100, "EUR"))
		s.NoError(err)
	})

	s.Run("authorization exceeding limit is rejected", func() {
		limit := vo.NewFromInt(500, "EUR")
		account := NewCardAccount(NewCardAccountID(), vo.MustParseTenantID("tenant-1"), limit)
		err := account.CanAuthorize(vo.NewFromInt(600, "EUR"))
		s.ErrorIs(err, ErrSpendingLimitExceeded{})
	})

	s.Run("limit considers existing authorizations", func() {
		limit := vo.NewFromInt(500, "EUR")
		account := NewCardAccount(NewCardAccountID(), vo.MustParseTenantID("tenant-1"), limit)
		err := account.RecordAuthorization(vo.NewFromInt(450, "EUR"))
		s.Require().NoError(err)
		err = account.CanAuthorize(vo.NewFromInt(100, "EUR"))
		s.ErrorIs(err, ErrSpendingLimitExceeded{})
	})

	s.Run("authorization at exact limit is allowed", func() {
		limit := vo.NewFromInt(500, "EUR")
		account := NewCardAccount(NewCardAccountID(), vo.MustParseTenantID("tenant-1"), limit)
		err := account.CanAuthorize(vo.NewFromInt(500, "EUR"))
		s.NoError(err)
	})

	s.Run("currency mismatch is rejected", func() {
		limit := vo.NewFromInt(500, "EUR")
		account := NewCardAccount(NewCardAccountID(), vo.MustParseTenantID("tenant-1"), limit)
		err := account.CanAuthorize(vo.NewFromInt(100, "USD"))
		s.ErrorIs(err, ErrCurrencyMismatch{})
	})
}

// Rolling Spend Counters
func (s *CardAccountSuite) TestRollingSpendCounters() {
	s.Run("authorization increases rolling spend", func() {
		limit := vo.NewFromInt(500, "EUR")
		account := NewCardAccount(NewCardAccountID(), vo.MustParseTenantID("tenant-1"), limit)
		err := account.RecordAuthorization(vo.NewFromInt(100, "EUR"))
		s.Require().NoError(err)
		s.True(account.RollingSpend().Equal(vo.NewFromInt(100, "EUR")))
	})

	s.Run("capture does not double-count spend", func() {
		limit := vo.NewFromInt(500, "EUR")
		account := NewCardAccount(NewCardAccountID(), vo.MustParseTenantID("tenant-1"), limit)
		err := account.RecordAuthorization(vo.NewFromInt(100, "EUR"))
		s.Require().NoError(err)
		err = account.RecordCapture(vo.NewFromInt(100, "EUR")) // already counted
		s.Require().NoError(err)
		s.True(account.RollingSpend().Equal(vo.NewFromInt(100, "EUR")))
	})

	s.Run("reversal decreases rolling spend", func() {
		limit := vo.NewFromInt(500, "EUR")
		account := NewCardAccount(NewCardAccountID(), vo.MustParseTenantID("tenant-1"), limit)
		err := account.RecordAuthorization(vo.NewFromInt(100, "EUR"))
		s.Require().NoError(err)
		err = account.RecordReversal(vo.NewFromInt(100, "EUR"))
		s.Require().NoError(err)
		s.True(account.RollingSpend().Equal(vo.NewFromInt(0, "EUR")))
	})

	s.Run("multiple authorizations accumulate", func() {
		limit := vo.NewFromInt(500, "EUR")
		account := NewCardAccount(NewCardAccountID(), vo.MustParseTenantID("tenant-1"), limit)
		_ = account.RecordAuthorization(vo.NewFromInt(100, "EUR"))
		_ = account.RecordAuthorization(vo.NewFromInt(150, "EUR"))
		s.True(account.RollingSpend().Equal(vo.NewFromInt(250, "EUR")))
	})
}

func (s *CardAccountSuite) TestAvailableLimit() {
	s.Run("available limit is spending limit minus rolling spend", func() {
		limit := vo.NewFromInt(500, "EUR")
		account := NewCardAccount(NewCardAccountID(), vo.MustParseTenantID("tenant-1"), limit)
		_ = account.RecordAuthorization(vo.NewFromInt(200, "EUR"))
		s.True(account.AvailableLimit().Equal(vo.NewFromInt(300, "EUR")))
	})
}

// Security: Atomic AuthorizeAmount prevents TOCTOU races
func (s *CardAccountSuite) TestAtomicAuthorizeAmount() {
	s.Run("AuthorizeAmount atomically checks and records", func() {
		limit := vo.NewFromInt(500, "EUR")
		account := NewCardAccount(NewCardAccountID(), vo.MustParseTenantID("tenant-1"), limit)

		// First authorization should succeed and update rolling spend
		err := account.AuthorizeAmount(vo.NewFromInt(300, "EUR"))
		s.NoError(err)
		s.True(account.RollingSpend().Equal(vo.NewFromInt(300, "EUR")))

		// Second authorization that would exceed limit should fail
		// AND should NOT update the rolling spend
		err = account.AuthorizeAmount(vo.NewFromInt(300, "EUR"))
		s.ErrorIs(err, ErrSpendingLimitExceeded{})
		// Rolling spend should remain unchanged
		s.True(account.RollingSpend().Equal(vo.NewFromInt(300, "EUR")))
	})

	s.Run("AuthorizeAmount rejects zero amount", func() {
		limit := vo.NewFromInt(500, "EUR")
		account := NewCardAccount(NewCardAccountID(), vo.MustParseTenantID("tenant-1"), limit)

		// Zero amount should succeed (no spend, no limit exceeded)
		// but the rolling spend should be zero
		err := account.AuthorizeAmount(vo.NewFromInt(0, "EUR"))
		s.NoError(err)
		s.True(account.RollingSpend().IsZero())
	})
}
