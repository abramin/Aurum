package domain

type AuthorizationState string

const (
	AuthorizationStateAuthorized AuthorizationState = "authorized"
	AuthorizationStateCaptured   AuthorizationState = "captured"
	AuthorizationStateReversed   AuthorizationState = "reversed"
	AuthorizationStateExpired    AuthorizationState = "expired"
)
