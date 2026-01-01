package application

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"aurum/internal/common/logging"
	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
)

// SpendingService implements the application layer for the Spending context.
// It uses the Atomic pattern from Qonto for transaction management.
//
// Key design decisions:
//   - All state-changing operations use the Atomic callback pattern
//   - Domain events are written to the outbox within the same transaction
//   - Idempotency is enforced at the service layer
//
// See: https://medium.com/qonto-way/transactions-in-go-hexagonal-architecture-f12c7a817a61
type SpendingService struct {
	dataStore domain.AtomicExecutor
	repos     domain.Repositories
}

// NewSpendingService creates a new SpendingService.
// The dataStore must implement both AtomicExecutor and Repositories interfaces.
func NewSpendingService(dataStore interface {
	domain.AtomicExecutor
	domain.Repositories
}) *SpendingService {
	return &SpendingService{
		dataStore: dataStore,
		repos:     dataStore,
	}
}

// CreateAuthorizationRequest represents a request to create an authorization.
type CreateAuthorizationRequest struct {
	TenantID       types.TenantID
	IdempotencyKey string
	Amount         types.Money
	MerchantRef    string
	Reference      string
	CorrelationID  types.CorrelationID
}

// CreateAuthorizationResponse represents the response from creating an authorization.
type CreateAuthorizationResponse struct {
	AuthorizationID string `json:"authorization_id"`
	Status          string `json:"status"`
}

// CreateAuthorization creates a new spend authorization.
// This operation:
//   - Checks idempotency key and returns existing response if found
//   - Validates spending limit on the card account
//   - Creates the authorization in Authorized state
//   - Writes SpendAuthorized event to outbox
//   - All within a single atomic transaction
func (s *SpendingService) CreateAuthorization(ctx context.Context, req CreateAuthorizationRequest) (*CreateAuthorizationResponse, error) {
	var result *CreateAuthorizationResponse

	err := s.dataStore.Atomic(ctx, func(repos domain.Repositories) error {
		// Check idempotency
		existing, err := repos.IdempotencyStore().Get(ctx, req.TenantID, req.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			// Replay existing response
			var resp CreateAuthorizationResponse
			if err := json.Unmarshal(existing.ResponseBody, &resp); err != nil {
				return err
			}
			result = &resp
			return nil
		}

		// Get card account for tenant
		cardAccount, err := repos.CardAccounts().FindByTenantID(ctx, req.TenantID)
		if err != nil {
			return err
		}

		// Validate spending limit
		if err := cardAccount.AuthorizeAmount(req.Amount); err != nil {
			return err
		}

		// Create authorization
		auth := domain.NewAuthorization(
			req.TenantID,
			cardAccount.ID(),
			req.Amount,
			req.MerchantRef,
			req.Reference,
		)

		// Save card account (with updated rolling spend)
		if err := repos.CardAccounts().Save(ctx, cardAccount); err != nil {
			return err
		}

		// Save authorization
		if err := repos.Authorizations().Save(ctx, auth); err != nil {
			return err
		}

		// Write event to outbox
		outboxEntry, err := domain.NewSpendAuthorizedOutboxEntry(auth, req.CorrelationID)
		if err != nil {
			return err
		}
		if err := repos.Outbox().Append(ctx, outboxEntry); err != nil {
			return err
		}

		// Prepare response
		result = &CreateAuthorizationResponse{
			AuthorizationID: auth.ID().String(),
			Status:          string(auth.State()),
		}

		// Store idempotency entry
		responseBody, _ := json.Marshal(result)
		if err := repos.IdempotencyStore().Set(ctx, &domain.IdempotencyEntry{
			TenantID:       req.TenantID,
			IdempotencyKey: req.IdempotencyKey,
			ResourceID:     auth.ID().String(),
			StatusCode:     http.StatusCreated,
			ResponseBody:   responseBody,
			CreatedAt:      time.Now(),
		}); err != nil {
			return err
		}

		logging.InfoContext(ctx, "Authorization created",
			"authorization_id", auth.ID().String(),
			"tenant_id", req.TenantID.String(),
			"amount", req.Amount.String(),
		)

		return nil
	})

	return result, err
}

// CaptureAuthorizationRequest represents a request to capture an authorization.
type CaptureAuthorizationRequest struct {
	TenantID        types.TenantID
	AuthorizationID domain.AuthorizationID
	IdempotencyKey  string
	Amount          types.Money
	CorrelationID   types.CorrelationID
}

// CaptureAuthorizationResponse represents the response from capturing an authorization.
type CaptureAuthorizationResponse struct {
	AuthorizationID string `json:"authorization_id"`
	Status          string `json:"status"`
}

// CaptureAuthorization captures an existing authorization.
// This operation:
//   - Checks idempotency key and returns existing response if found
//   - Validates authorization state and capture amount
//   - Updates authorization to Captured state
//   - Writes SpendCaptured event to outbox
//   - All within a single atomic transaction
func (s *SpendingService) CaptureAuthorization(ctx context.Context, req CaptureAuthorizationRequest) (*CaptureAuthorizationResponse, error) {
	var result *CaptureAuthorizationResponse

	err := s.dataStore.Atomic(ctx, func(repos domain.Repositories) error {
		// Check idempotency
		existing, err := repos.IdempotencyStore().Get(ctx, req.TenantID, req.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			// Replay existing response
			var resp CaptureAuthorizationResponse
			if err := json.Unmarshal(existing.ResponseBody, &resp); err != nil {
				return err
			}
			result = &resp
			return nil
		}

		// Get authorization
		auth, err := repos.Authorizations().FindByID(ctx, req.TenantID, req.AuthorizationID)
		if err != nil {
			return err
		}

		// Capture authorization
		if err := auth.Capture(req.Amount); err != nil {
			return err
		}

		// Save authorization
		if err := repos.Authorizations().Save(ctx, auth); err != nil {
			return err
		}

		// Write event to outbox
		outboxEntry, err := domain.NewSpendCapturedOutboxEntry(auth, req.CorrelationID)
		if err != nil {
			return err
		}
		if err := repos.Outbox().Append(ctx, outboxEntry); err != nil {
			return err
		}

		// Prepare response
		result = &CaptureAuthorizationResponse{
			AuthorizationID: auth.ID().String(),
			Status:          string(auth.State()),
		}

		// Store idempotency entry
		responseBody, _ := json.Marshal(result)
		if err := repos.IdempotencyStore().Set(ctx, &domain.IdempotencyEntry{
			TenantID:       req.TenantID,
			IdempotencyKey: req.IdempotencyKey,
			ResourceID:     auth.ID().String(),
			StatusCode:     http.StatusOK,
			ResponseBody:   responseBody,
			CreatedAt:      time.Now(),
		}); err != nil {
			return err
		}

		logging.InfoContext(ctx, "Authorization captured",
			"authorization_id", auth.ID().String(),
			"tenant_id", req.TenantID.String(),
			"captured_amount", req.Amount.String(),
		)

		return nil
	})

	return result, err
}

// GetAuthorizationRequest represents a request to get an authorization.
type GetAuthorizationRequest struct {
	TenantID        types.TenantID
	AuthorizationID domain.AuthorizationID
}

// GetAuthorizationResponse represents the response from getting an authorization.
type GetAuthorizationResponse struct {
	AuthorizationID  string      `json:"authorization_id"`
	CardAccountID    string      `json:"card_account_id"`
	AuthorizedAmount types.Money `json:"authorized_amount"`
	CapturedAmount   types.Money `json:"captured_amount"`
	MerchantRef      string      `json:"merchant_ref"`
	Reference        string      `json:"reference"`
	Status           string      `json:"status"`
	CreatedAt        string      `json:"created_at"`
	UpdatedAt        string      `json:"updated_at"`
}

// GetAuthorization retrieves an authorization by ID.
// This is a read-only operation and doesn't use the Atomic pattern.
func (s *SpendingService) GetAuthorization(ctx context.Context, req GetAuthorizationRequest) (*GetAuthorizationResponse, error) {
	auth, err := s.repos.Authorizations().FindByID(ctx, req.TenantID, req.AuthorizationID)
	if err != nil {
		return nil, err
	}

	return &GetAuthorizationResponse{
		AuthorizationID:  auth.ID().String(),
		CardAccountID:    auth.CardAccountID().String(),
		AuthorizedAmount: auth.AuthorizedAmount(),
		CapturedAmount:   auth.CapturedAmount(),
		MerchantRef:      auth.MerchantRef(),
		Reference:        auth.Reference(),
		Status:           string(auth.State()),
		CreatedAt:        auth.CreatedAt().Format(time.RFC3339),
		UpdatedAt:        auth.UpdatedAt().Format(time.RFC3339),
	}, nil
}

// CreateCardAccountRequest represents a request to create a card account.
type CreateCardAccountRequest struct {
	TenantID      types.TenantID
	SpendingLimit types.Money
}

// CreateCardAccountResponse represents the response from creating a card account.
type CreateCardAccountResponse struct {
	CardAccountID string `json:"card_account_id"`
}

// CreateCardAccount creates a new card account for a tenant.
// This is typically done during onboarding.
func (s *SpendingService) CreateCardAccount(ctx context.Context, req CreateCardAccountRequest) (*CreateCardAccountResponse, error) {
	var result *CreateCardAccountResponse

	err := s.dataStore.Atomic(ctx, func(repos domain.Repositories) error {
		account := domain.NewCardAccount(req.TenantID, req.SpendingLimit)

		if err := repos.CardAccounts().Save(ctx, account); err != nil {
			return err
		}

		result = &CreateCardAccountResponse{
			CardAccountID: account.ID().String(),
		}

		logging.InfoContext(ctx, "Card account created",
			"card_account_id", account.ID().String(),
			"tenant_id", req.TenantID.String(),
			"spending_limit", req.SpendingLimit.String(),
		)

		return nil
	})

	return result, err
}
