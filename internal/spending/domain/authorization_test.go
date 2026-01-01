package domain_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"

	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
)

// AuthorizationSuite tests Authorization aggregate behavior.
//
// Justification for unit tests: These tests provide fast feedback on state machine
// invariants. The same behaviors are covered at integration level via feature files,
// but unit tests catch regressions before slower integration tests run.
type AuthorizationSuite struct {
	suite.Suite
	tenantID      types.TenantID
	cardAccountID domain.CardAccountID
	amount        types.Money
}

func TestAuthorizationSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationSuite))
}

func (s *AuthorizationSuite) SetupTest() {
	s.tenantID = types.TenantID("tenant-1")
	s.cardAccountID = domain.NewCardAccountID()
	s.amount = types.NewMoney(decimal.NewFromInt(100), types.CurrencyEUR)
}

func (s *AuthorizationSuite) newAuthorization() *domain.Authorization {
	auth, err := domain.NewAuthorization(s.tenantID, s.cardAccountID, s.amount, "merchant-1", "ref-1")
	s.Require().NoError(err)
	return auth
}

// TestSettlement validates the capture lifecycle - transitioning from authorized to captured state.
func (s *AuthorizationSuite) TestSettlement() {
	s.Run("settles for partial amount", func() {
		auth := s.newAuthorization()
		captureAmount := types.NewMoney(decimal.NewFromInt(50), types.CurrencyEUR)

		err := auth.Capture(captureAmount)

		s.Require().NoError(err)
		s.Equal(domain.AuthorizationStateCaptured, auth.State())
		s.True(auth.CapturedAmount().Equal(captureAmount))
	})

	s.Run("settles for full authorized amount", func() {
		auth := s.newAuthorization()

		err := auth.Capture(s.amount)

		s.Require().NoError(err)
		s.True(auth.CapturedAmount().Equal(s.amount))
	})

	s.Run("rejects amount exceeding authorization", func() {
		auth := s.newAuthorization()
		captureAmount := types.NewMoney(decimal.NewFromInt(150), types.CurrencyEUR)

		err := auth.Capture(captureAmount)

		s.ErrorIs(err, domain.ErrExceedsAuthorizedAmount)
		s.Equal(domain.AuthorizationStateAuthorized, auth.State(), "state should remain authorized on rejection")
	})

	s.Run("prevents double settlement", func() {
		auth := s.newAuthorization()
		captureAmount := types.NewMoney(decimal.NewFromInt(50), types.CurrencyEUR)
		_ = auth.Capture(captureAmount)

		err := auth.Capture(captureAmount)

		s.ErrorIs(err, domain.ErrAlreadyCaptured)
	})

	s.Run("rejects currency mismatch", func() {
		auth := s.newAuthorization()
		captureAmount := types.NewMoney(decimal.NewFromInt(50), types.CurrencyUSD)

		err := auth.Capture(captureAmount)

		s.ErrorIs(err, domain.ErrCurrencyMismatch)
	})

	s.Run("rejects settlement of reversed authorization", func() {
		auth := s.newAuthorization()
		_ = auth.Reverse()
		captureAmount := types.NewMoney(decimal.NewFromInt(50), types.CurrencyEUR)

		err := auth.Capture(captureAmount)

		s.ErrorIs(err, domain.ErrInvalidStateTransition)
	})
}

// TestReversal validates the reversal lifecycle - releasing holds on authorized amounts.
func (s *AuthorizationSuite) TestReversal() {
	s.Run("reverses authorized amount", func() {
		auth := s.newAuthorization()

		err := auth.Reverse()

		s.Require().NoError(err)
		s.Equal(domain.AuthorizationStateReversed, auth.State())
	})

	s.Run("rejects reversal of captured authorization", func() {
		auth := s.newAuthorization()
		_ = auth.Capture(s.amount)

		err := auth.Reverse()

		s.ErrorIs(err, domain.ErrInvalidStateTransition)
	})
}
