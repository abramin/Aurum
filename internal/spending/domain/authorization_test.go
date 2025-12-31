package domain

import (
	"testing"

	vo "aurum/internal/common/value_objects"

	"github.com/stretchr/testify/suite"
)

type AuthorizationSuite struct {
	suite.Suite
}

func TestAuthorizationSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationSuite))
}

func (s *AuthorizationSuite) TestStateTransitions() {
	amount, err := vo.NewFromString("1000.0", "EUR")
	s.Require().NoError(err)
	tenantID := vo.MustParseTenantID("tenant-1")
	cardAccountID := NewCardAccountID()

	s.Run("new authorization starts in Authorized state", func() {
		auth := NewAuthorization(tenantID, cardAccountID, amount)
		s.Equal(AuthorizationStateAuthorized, auth.State())
		s.True(auth.AuthorizedAmount().Equal(amount))
		s.True(auth.CapturedAmount().IsZero())
	})

	s.Run("Authorized can transition to Captured", func() {
		auth := NewAuthorization(tenantID, cardAccountID, amount)
		err := auth.Capture(amount)
		s.Require().NoError(err)
		s.Equal(AuthorizationStateCaptured, auth.State())
	})

	s.Run("Authorized can transition to Reversed", func() {
		auth := NewAuthorization(tenantID, cardAccountID, amount)
		err := auth.Reverse()
		s.Require().NoError(err)
		s.Equal(AuthorizationStateReversed, auth.State())
	})

	s.Run("Authorized can transition to Expired", func() {
		auth := NewAuthorization(tenantID, cardAccountID, amount)
		err := auth.Expire()
		s.Require().NoError(err)
		s.Equal(AuthorizationStateExpired, auth.State())
	})
}

func (s *AuthorizationSuite) TestCaptureInvariants() {
	amount, err := vo.NewFromString("100", "EUR")
	s.Require().NoError(err)
	tenantID := vo.MustParseTenantID("tenant-1")
	cardAccountID := NewCardAccountID()

	s.Run("cannot capture more than authorized amount", func() {
		auth := NewAuthorization(tenantID, cardAccountID, amount)
		err := auth.Capture(vo.NewFromInt(150, "EUR"))
		s.ErrorIs(err, ErrExceedsAuthorizedAmount{})
	})

	s.Run("cannot capture from non-Authorized state", func() {
		tests := []struct {
			name       string
			setupState func() *Authorization
		}{
			{
				name: "Reversed",
				setupState: func() *Authorization {
					auth := NewAuthorization(tenantID, cardAccountID, amount)
					_ = auth.Reverse()
					return auth
				},
			},
			{
				name: "Expired",
				setupState: func() *Authorization {
					auth := NewAuthorization(tenantID, cardAccountID, amount)
					_ = auth.Expire()
					return auth
				},
			},
		}

		for _, tt := range tests {
			s.Run(tt.name, func() {
				auth := tt.setupState()
				err := auth.Capture(amount)
				s.ErrorIs(err, ErrInvalidStateTransition{})
			})
		}
	})

	s.Run("cannot capture twice", func() {
		auth := NewAuthorization(tenantID, cardAccountID, amount)
		_ = auth.Capture(amount)
		err := auth.Capture(amount)
		s.ErrorIs(err, ErrAlreadyCaptured{})
	})

	s.Run("partial capture records captured amount", func() {
		auth := NewAuthorization(tenantID, cardAccountID, amount)
		err := auth.Capture(vo.NewFromInt(60, "EUR"))
		s.Require().NoError(err)
		s.True(auth.CapturedAmount().Equal(vo.NewFromInt(60, "EUR")))
	})
}

// Security: ID type safety tests
func (s *AuthorizationSuite) TestIDTypeSafety() {
	s.Run("ParseAuthorizationID rejects empty string", func() {
		_, err := ParseAuthorizationID("")
		s.Error(err)
		s.ErrorIs(err, ErrEmptyAuthorizationID)
	})

	s.Run("ParseAuthorizationID rejects invalid UUID", func() {
		_, err := ParseAuthorizationID("not-a-uuid")
		s.Error(err)
		s.ErrorIs(err, ErrInvalidAuthorizationID)
	})

	s.Run("ParseAuthorizationID accepts valid UUID", func() {
		id := NewAuthorizationID()
		parsed, err := ParseAuthorizationID(id.String())
		s.NoError(err)
		s.Equal(id.String(), parsed.String())
	})

	s.Run("ParseCardAccountID rejects empty string", func() {
		_, err := ParseCardAccountID("")
		s.Error(err)
		s.ErrorIs(err, ErrEmptyCardAccountID)
	})

	s.Run("ParseCardAccountID rejects invalid UUID", func() {
		_, err := ParseCardAccountID("not-a-uuid")
		s.Error(err)
		s.ErrorIs(err, ErrInvalidCardAccountID)
	})

	s.Run("ParseCardAccountID accepts valid UUID", func() {
		id := NewCardAccountID()
		parsed, err := ParseCardAccountID(id.String())
		s.NoError(err)
		s.Equal(id.String(), parsed.String())
	})
}
