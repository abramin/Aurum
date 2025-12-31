package application

import (
	"context"
	"errors"

	"aurum/internal/spending/domain"

	vo "aurum/internal/common/value_objects"
)

// SpendingService handles authorization and capture operations.
type SpendingService struct {
	authRepo         domain.AuthorizationRepository
	cardAccountRepo  domain.CardAccountRepository
	idempotencyStore domain.IdempotencyStore
	unitOfWork       domain.UnitOfWork
}

// NewSpendingService creates a new SpendingService.
func NewSpendingService(
	authRepo domain.AuthorizationRepository,
	cardAccountRepo domain.CardAccountRepository,
	idempotencyStore domain.IdempotencyStore,
) *SpendingService {
	return &SpendingService{
		authRepo:         authRepo,
		cardAccountRepo:  cardAccountRepo,
		idempotencyStore: idempotencyStore,
	}
}

// NewSpendingServiceWithUoW creates a new SpendingService with Unit of Work support.
func NewSpendingServiceWithUoW(
	authRepo domain.AuthorizationRepository,
	cardAccountRepo domain.CardAccountRepository,
	idempotencyStore domain.IdempotencyStore,
	unitOfWork domain.UnitOfWork,
) *SpendingService {
	return &SpendingService{
		authRepo:         authRepo,
		cardAccountRepo:  cardAccountRepo,
		idempotencyStore: idempotencyStore,
		unitOfWork:       unitOfWork,
	}
}

// CreateAuthorizationRequest represents a request to create an authorization.
type CreateAuthorizationRequest struct {
	TenantID       vo.TenantID
	IdempotencyKey string
	CardAccountID  domain.CardAccountID
	Amount         vo.Money
}

// CreateAuthorizationResponse represents the response from creating an authorization.
type CreateAuthorizationResponse struct {
	ID               domain.AuthorizationID
	State            domain.AuthorizationState
	AuthorizedAmount vo.Money
	CapturedAmount   vo.Money
	IsIdempotent     bool
}

// ErrCardAccountNotFound is returned when the card account is not found.
var ErrCardAccountNotFound = errors.New("card account not found")

// ErrAuthorizationNotFound is returned when the authorization is not found.
var ErrAuthorizationNotFound = errors.New("authorization not found")

// CreateAuthorization creates a new authorization with idempotency support.
func (s *SpendingService) CreateAuthorization(ctx context.Context, req CreateAuthorizationRequest) (*CreateAuthorizationResponse, error) {
	// Use transactional path if UnitOfWork is configured
	if s.unitOfWork != nil {
		return s.createAuthorizationWithTx(ctx, req)
	}
	return s.createAuthorizationLegacy(ctx, req)
}

// createAuthorizationWithTx creates an authorization using transactional semantics.
func (s *SpendingService) createAuthorizationWithTx(ctx context.Context, req CreateAuthorizationRequest) (*CreateAuthorizationResponse, error) {
	// Begin transaction
	tx, err := s.unitOfWork.Begin(ctx)
	if err != nil {
		return nil, err
	}

	// Ensure rollback on any error path
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	cardAccountRepo := tx.CardAccountRepo()
	authRepo := tx.AuthorizationRepo()

	// Get card account
	cardAccount, err := cardAccountRepo.FindByID(ctx, req.TenantID, req.CardAccountID)
	if err != nil {
		return nil, err
	}
	if cardAccount == nil {
		return nil, ErrCardAccountNotFound
	}

	// Create authorization (generates ID)
	auth := domain.NewAuthorization(req.TenantID, req.CardAccountID, req.Amount)

	// Prepare idempotency entry with the authorization ID
	idempotencyEntry := &domain.IdempotencyEntry{
		TenantID:       req.TenantID,
		IdempotencyKey: req.IdempotencyKey,
		ResourceID:     auth.ID().String(),
		StatusCode:     201,
	}

	// Atomically check and set idempotency key
	created, existing, err := s.idempotencyStore.SetIfAbsent(ctx, idempotencyEntry)
	if err != nil {
		return nil, err
	}
	if !created {
		// Return the existing authorization
		existingAuthID, err := domain.ParseAuthorizationID(existing.ResourceID)
		if err != nil {
			return nil, err
		}
		existingAuth, err := authRepo.FindByID(ctx, req.TenantID, existingAuthID)
		if err != nil {
			return nil, err
		}
		if existingAuth == nil {
			return nil, ErrAuthorizationNotFound
		}
		return &CreateAuthorizationResponse{
			ID:               existingAuth.ID(),
			State:            existingAuth.State(),
			AuthorizedAmount: existingAuth.AuthorizedAmount(),
			CapturedAmount:   existingAuth.CapturedAmount(),
			IsIdempotent:     true,
		}, nil
	}

	// Atomically check spending limit and record authorization
	if err := cardAccount.AuthorizeAmount(req.Amount); err != nil {
		return nil, err
	}

	// Stage card account and authorization saves
	if err := cardAccountRepo.Save(ctx, cardAccount); err != nil {
		return nil, err
	}
	if err := authRepo.Save(ctx, auth); err != nil {
		return nil, err
	}

	// Commit transaction atomically
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	return &CreateAuthorizationResponse{
		ID:               auth.ID(),
		State:            auth.State(),
		AuthorizedAmount: auth.AuthorizedAmount(),
		CapturedAmount:   auth.CapturedAmount(),
		IsIdempotent:     false,
	}, nil
}

// createAuthorizationLegacy creates an authorization without transactions (legacy path).
func (s *SpendingService) createAuthorizationLegacy(ctx context.Context, req CreateAuthorizationRequest) (*CreateAuthorizationResponse, error) {
	// Get card account and check spending limit first
	cardAccount, err := s.cardAccountRepo.FindByID(ctx, req.TenantID, req.CardAccountID)
	if err != nil {
		return nil, err
	}
	if cardAccount == nil {
		return nil, ErrCardAccountNotFound
	}

	// Create authorization (generates ID)
	auth := domain.NewAuthorization(req.TenantID, req.CardAccountID, req.Amount)

	// Prepare idempotency entry with the authorization ID
	idempotencyEntry := &domain.IdempotencyEntry{
		TenantID:       req.TenantID,
		IdempotencyKey: req.IdempotencyKey,
		ResourceID:     auth.ID().String(),
		StatusCode:     201,
	}

	// Atomically check and set idempotency key
	created, existing, err := s.idempotencyStore.SetIfAbsent(ctx, idempotencyEntry)
	if err != nil {
		return nil, err
	}
	if !created {
		// Return the existing authorization
		existingAuthID, err := domain.ParseAuthorizationID(existing.ResourceID)
		if err != nil {
			return nil, err
		}
		existingAuth, err := s.authRepo.FindByID(ctx, req.TenantID, existingAuthID)
		if err != nil {
			return nil, err
		}
		if existingAuth == nil {
			return nil, ErrAuthorizationNotFound
		}
		return &CreateAuthorizationResponse{
			ID:               existingAuth.ID(),
			State:            existingAuth.State(),
			AuthorizedAmount: existingAuth.AuthorizedAmount(),
			CapturedAmount:   existingAuth.CapturedAmount(),
			IsIdempotent:     true,
		}, nil
	}

	// Atomically check spending limit and record authorization
	// This prevents TOCTOU races between checking and recording
	if err := cardAccount.AuthorizeAmount(req.Amount); err != nil {
		return nil, err
	}

	// Save card account and authorization
	if err := s.cardAccountRepo.Save(ctx, cardAccount); err != nil {
		return nil, err
	}
	if err := s.authRepo.Save(ctx, auth); err != nil {
		return nil, err
	}

	return &CreateAuthorizationResponse{
		ID:               auth.ID(),
		State:            auth.State(),
		AuthorizedAmount: auth.AuthorizedAmount(),
		CapturedAmount:   auth.CapturedAmount(),
		IsIdempotent:     false,
	}, nil
}

// CaptureAuthorizationRequest represents a request to capture an authorization.
type CaptureAuthorizationRequest struct {
	TenantID        vo.TenantID
	AuthorizationID domain.AuthorizationID
	Amount          vo.Money
}

// CaptureAuthorizationResponse represents the response from capturing an authorization.
type CaptureAuthorizationResponse struct {
	ID               domain.AuthorizationID
	State            domain.AuthorizationState
	AuthorizedAmount vo.Money
	CapturedAmount   vo.Money
}

// CaptureAuthorization captures an existing authorization.
func (s *SpendingService) CaptureAuthorization(ctx context.Context, req CaptureAuthorizationRequest) (*CaptureAuthorizationResponse, error) {
	auth, err := s.authRepo.FindByID(ctx, req.TenantID, req.AuthorizationID)
	if err != nil {
		return nil, err
	}
	if auth == nil {
		return nil, ErrAuthorizationNotFound
	}

	if err := auth.Capture(req.Amount); err != nil {
		return nil, err
	}

	if err := s.authRepo.Save(ctx, auth); err != nil {
		return nil, err
	}

	return &CaptureAuthorizationResponse{
		ID:               auth.ID(),
		State:            auth.State(),
		AuthorizedAmount: auth.AuthorizedAmount(),
		CapturedAmount:   auth.CapturedAmount(),
	}, nil
}

// GetAuthorization retrieves an authorization by ID.
func (s *SpendingService) GetAuthorization(ctx context.Context, tenantID vo.TenantID, id domain.AuthorizationID) (*CreateAuthorizationResponse, error) {
	auth, err := s.authRepo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if auth == nil {
		return nil, ErrAuthorizationNotFound
	}

	return &CreateAuthorizationResponse{
		ID:               auth.ID(),
		State:            auth.State(),
		AuthorizedAmount: auth.AuthorizedAmount(),
		CapturedAmount:   auth.CapturedAmount(),
	}, nil
}
