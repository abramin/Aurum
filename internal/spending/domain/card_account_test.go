package domain

import (
	"testing"

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
		s.T().Skip("TDD: implement CardAccount.CanAuthorize()")
		// account := NewCardAccount(SpendingLimit{500, EUR})
		// err := account.CanAuthorize(Money{100, EUR})
		// s.NoError(err)
	})

	s.Run("authorization exceeding limit is rejected", func() {
		s.T().Skip("TDD: CanAuthorize returns error when over limit")
		// account := NewCardAccount(SpendingLimit{500, EUR})
		// err := account.CanAuthorize(Money{600, EUR})
		// s.ErrorIs(err, ErrSpendingLimitExceeded)
	})

	s.Run("limit considers existing authorizations", func() {
		s.T().Skip("TDD: rolling spend affects available limit")
		// account := NewCardAccount(SpendingLimit{500, EUR})
		// account.RecordAuthorization(Money{450, EUR})
		// err := account.CanAuthorize(Money{100, EUR})
		// s.ErrorIs(err, ErrSpendingLimitExceeded)
	})
}

// Rolling Spend Counters
func (s *CardAccountSuite) TestRollingSpendCounters() {
	s.Run("authorization increases rolling spend", func() {
		s.T().Skip("TDD: implement CardAccount.RecordAuthorization()")
		// account := NewCardAccount(SpendingLimit{500, EUR})
		// account.RecordAuthorization(Money{100, EUR})
		// s.Equal(Money{100, EUR}, account.RollingSpend())
	})

	s.Run("capture does not double-count spend", func() {
		s.T().Skip("TDD: capture of existing auth doesn't increase counter")
		// account := NewCardAccount(SpendingLimit{500, EUR})
		// account.RecordAuthorization(Money{100, EUR})
		// account.RecordCapture(Money{100, EUR}) // already counted
		// s.Equal(Money{100, EUR}, account.RollingSpend())
	})

	s.Run("reversal decreases rolling spend", func() {
		s.T().Skip("TDD: implement CardAccount.RecordReversal()")
		// account := NewCardAccount(SpendingLimit{500, EUR})
		// account.RecordAuthorization(Money{100, EUR})
		// account.RecordReversal(Money{100, EUR})
		// s.Equal(Money{0, EUR}, account.RollingSpend())
	})
}
