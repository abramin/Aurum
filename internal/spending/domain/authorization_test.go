package domain

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type AuthorizationSuite struct {
	suite.Suite
}

func TestAuthorizationSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationSuite))
}

// State Transitions - valid paths through the state machine
func (s *AuthorizationSuite) TestStateTransitions() {
	s.Run("new authorization starts in Authorized state", func() {
		s.T().Skip("TDD: implement Authorization.New()")
		// auth := NewAuthorization(...)
		// s.Equal(StateAuthorized, auth.State())
	})

	s.Run("Authorized can transition to Captured", func() {
		s.T().Skip("TDD: implement Authorization.Capture()")
		// auth := NewAuthorization(...)
		// err := auth.Capture(amount)
		// s.Require().NoError(err)
		// s.Equal(StateCaptured, auth.State())
	})

	s.Run("Authorized can transition to Reversed", func() {
		s.T().Skip("TDD: implement Authorization.Reverse()")
		// auth := NewAuthorization(...)
		// err := auth.Reverse()
		// s.Require().NoError(err)
		// s.Equal(StateReversed, auth.State())
	})

	s.Run("Authorized can transition to Expired", func() {
		s.T().Skip("TDD: implement Authorization.Expire()")
		// auth := NewAuthorization(...)
		// err := auth.Expire()
		// s.Require().NoError(err)
		// s.Equal(StateExpired, auth.State())
	})
}

// Capture Invariants - rules that must never be violated
func (s *AuthorizationSuite) TestCaptureInvariants() {
	s.Run("cannot capture more than authorized amount", func() {
		s.T().Skip("TDD: capture > authorized returns error")
		// auth := NewAuthorization(Money{100, EUR})
		// err := auth.Capture(Money{150, EUR})
		// s.ErrorIs(err, ErrExceedsAuthorizedAmount)
	})

	s.Run("cannot capture from non-Authorized state", func() {
		s.T().Skip("TDD: capture on Captured/Reversed/Expired returns error")
		// auth := newCapturedAuthorization()
		// err := auth.Capture(amount)
		// s.ErrorIs(err, ErrInvalidStateTransition)
	})

	s.Run("cannot capture twice", func() {
		s.T().Skip("TDD: second capture returns error")
		// auth := NewAuthorization(Money{100, EUR})
		// _ = auth.Capture(Money{100, EUR})
		// err := auth.Capture(Money{50, EUR})
		// s.ErrorIs(err, ErrAlreadyCaptured)
	})

	s.Run("partial capture records captured amount", func() {
		s.T().Skip("TDD: capture 60 of 100 records 60")
		// auth := NewAuthorization(Money{100, EUR})
		// err := auth.Capture(Money{60, EUR})
		// s.Require().NoError(err)
		// s.Equal(Money{60, EUR}, auth.CapturedAmount())
	})
}
