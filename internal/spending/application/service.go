package application

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"aurum/internal/common/logging"
	"aurum/internal/common/types"
	"aurum/internal/spending/domain"
)

// idempotencyConflictError is returned when a concurrent request won the race.
// The transaction should be rolled back and the existing response returned.
type idempotencyConflictError struct {
	existingEntry *domain.IdempotencyEntry
}

func (e *idempotencyConflictError) Error() string {
	return "idempotency conflict: concurrent request completed first"
}

// checkIdempotency checks if a response already exists for the given idempotency key.
// Returns the cached response if found, nil if not found.
func checkIdempotency[T any](ctx context.Context, store domain.IdempotencyStore, tenantID types.TenantID, key string) (*T, error) {
	existing, err := store.Get(ctx, tenantID, key)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}
	var resp T
	if err := json.Unmarshal(existing.ResponseBody, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// handleIdempotencyConflict handles the case where a concurrent request won the race.
// Returns (response, nil) if conflict was handled, (nil, original error) otherwise.
func handleIdempotencyConflict[T any](err error) (*T, error) {
	var conflictErr *idempotencyConflictError
	if !errors.As(err, &conflictErr) {
		return nil, err
	}
	var resp T
	if unmarshalErr := json.Unmarshal(conflictErr.existingEntry.ResponseBody, &resp); unmarshalErr != nil {
		return nil, unmarshalErr
	}
	return &resp, nil
}

// storeIdempotency atomically stores an idempotency entry, preventing TOCTOU races.
// Returns idempotencyConflictError if a concurrent request completed first.
func storeIdempotency[T any](
	ctx context.Context,
	store domain.IdempotencyStore,
	tenantID types.TenantID,
	idempotencyKey string,
	resourceID string,
	statusCode int,
	response *T,
	now time.Time,
) error {
	responseBody, _ := json.Marshal(response)
	created, existingEntry, err := store.SetIfAbsent(ctx, &domain.IdempotencyEntry{
		TenantID:       tenantID,
		IdempotencyKey: idempotencyKey,
		ResourceID:     resourceID,
		StatusCode:     statusCode,
		ResponseBody:   responseBody,
		CreatedAt:      now,
	})
	if err != nil {
		return err
	}
	if !created {
		return &idempotencyConflictError{existingEntry: existingEntry}
	}
	return nil
}

// SpendingService implements the application layer for the Spending context.
//
// Key design decisions:
//   - All state-changing operations use the Atomic callback pattern
//   - Domain events are written to the outbox within the same transaction
//   - Idempotency is enforced at the service layer
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
//   - Loads the tenant card account and enforces spending limits
//   - Creates the authorization in Authorized state and writes an outbox event
//   - Stores the idempotency entry atomically to avoid TOCTOU races
//   - Returns the stored response if a concurrent request wins the idempotency race
//   - All within a single atomic transaction
func (s *SpendingService) CreateAuthorization(ctx context.Context, req CreateAuthorizationRequest) (*CreateAuthorizationResponse, error) {
	// Check idempotency OUTSIDE transaction - fast path for replays
	// This read is already atomic and shortens transaction duration
	if cached, err := checkIdempotency[CreateAuthorizationResponse](ctx, s.repos.IdempotencyStore(), req.TenantID, req.IdempotencyKey); err != nil {
		return nil, err
	} else if cached != nil {
		return cached, nil
	}

	var result *CreateAuthorizationResponse

	err := s.dataStore.Atomic(ctx, func(repos domain.Repositories) error {
		now := time.Now()

		// Get card account for tenant
		cardAccount, err := repos.CardAccounts().FindByTenantID(ctx, req.TenantID)
		if err != nil {
			return err
		}

		// Validate spending limit
		if err := cardAccount.AuthorizeAmount(req.Amount, now); err != nil {
			return err
		}

		// Create authorization
		auth, err := domain.NewAuthorization(
			req.TenantID,
			cardAccount.ID(),
			req.Amount,
			req.MerchantRef,
			req.Reference,
			now,
		)
		if err != nil {
			return err
		}

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

		// Atomically store idempotency entry - prevents TOCTOU race
		if err := storeIdempotency(ctx, repos.IdempotencyStore(), req.TenantID, req.IdempotencyKey,
			auth.ID().String(), http.StatusCreated, result, now); err != nil {
			return err
		}

		logging.InfoContext(ctx, "Authorization created",
			"authorization_id", auth.ID().String(),
			"tenant_id", req.TenantID.String(),
			"amount", req.Amount.String(),
		)

		return nil
	})

	if conflict, conflictErr := handleIdempotencyConflict[CreateAuthorizationResponse](err); conflictErr != nil {
		return nil, conflictErr
	} else if conflict != nil {
		return conflict, nil
	}

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
	CapturedAmount  string `json:"captured_amount"`
}

// CaptureAuthorization captures an existing authorization.
// This operation:
//   - Checks idempotency key and returns existing response if found
//   - Validates authorization state and capture amount
//   - Updates authorization to Captured state and writes an outbox event
//   - Stores the idempotency entry atomically to avoid TOCTOU races
//   - Returns the stored response if a concurrent request wins the idempotency race
//   - All within a single atomic transaction
func (s *SpendingService) CaptureAuthorization(ctx context.Context, req CaptureAuthorizationRequest) (*CaptureAuthorizationResponse, error) {
	// Check idempotency OUTSIDE transaction - fast path for replays
	// This read is already atomic and shortens transaction duration
	if cached, err := checkIdempotency[CaptureAuthorizationResponse](ctx, s.repos.IdempotencyStore(), req.TenantID, req.IdempotencyKey); err != nil {
		return nil, err
	} else if cached != nil {
		return cached, nil
	}

	var result *CaptureAuthorizationResponse

	err := s.dataStore.Atomic(ctx, func(repos domain.Repositories) error {
		now := time.Now()

		// Get authorization
		auth, err := repos.Authorizations().FindByID(ctx, req.TenantID, req.AuthorizationID)
		if err != nil {
			return err
		}

		// Capture authorization
		if err := auth.Capture(req.Amount, now); err != nil {
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
			CapturedAmount:  auth.CapturedAmount().String(),
		}

		// Atomically store idempotency entry - prevents TOCTOU race
		if err := storeIdempotency(ctx, repos.IdempotencyStore(), req.TenantID, req.IdempotencyKey,
			auth.ID().String(), http.StatusOK, result, now); err != nil {
			return err
		}

		logging.InfoContext(ctx, "Authorization captured",
			"authorization_id", auth.ID().String(),
			"tenant_id", req.TenantID.String(),
			"captured_amount", req.Amount.String(),
		)

		return nil
	})

	if conflict, conflictErr := handleIdempotencyConflict[CaptureAuthorizationResponse](err); conflictErr != nil {
		return nil, conflictErr
	} else if conflict != nil {
		return conflict, nil
	}

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
// It loads the aggregate and maps domain fields into the response payload,
// formatting timestamps in RFC3339 for the API response.
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
// This operation:
//   - Creates a new account with the provided limit and current timestamp
//   - Persists the account within a single transaction
//   - Returns the generated account ID
// This is typically done during onboarding.
func (s *SpendingService) CreateCardAccount(ctx context.Context, req CreateCardAccountRequest) (*CreateCardAccountResponse, error) {
	var result *CreateCardAccountResponse

	err := s.dataStore.Atomic(ctx, func(repos domain.Repositories) error {
		now := time.Now()

		account, err := domain.NewCardAccount(req.TenantID, req.SpendingLimit, now)
		if err != nil {
			return err
		}

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

// ReverseAuthorizationRequest represents a request to reverse an authorization.
type ReverseAuthorizationRequest struct {
	TenantID        types.TenantID
	AuthorizationID domain.AuthorizationID
	IdempotencyKey  string
	CorrelationID   types.CorrelationID
}

// ReverseAuthorizationResponse represents the response from reversing an authorization.
type ReverseAuthorizationResponse struct {
	AuthorizationID string `json:"authorization_id"`
	Status          string `json:"status"`
}

// ReverseAuthorization reverses an existing authorization, releasing the held amount.
// This operation:
//   - Checks idempotency key and returns existing response if found
//   - Validates the authorization can be reversed (not already captured)
//   - Updates authorization to Reversed state and releases the held amount
//   - Writes a SpendReversed event to the outbox
//   - Stores the idempotency entry atomically to avoid TOCTOU races
//   - Returns the stored response if a concurrent request wins the idempotency race
//   - All within a single atomic transaction
func (s *SpendingService) ReverseAuthorization(ctx context.Context, req ReverseAuthorizationRequest) (*ReverseAuthorizationResponse, error) {
	// Check idempotency OUTSIDE transaction - fast path for replays
	// This read is already atomic and shortens transaction duration
	if cached, err := checkIdempotency[ReverseAuthorizationResponse](ctx, s.repos.IdempotencyStore(), req.TenantID, req.IdempotencyKey); err != nil {
		return nil, err
	} else if cached != nil {
		return cached, nil
	}

	var result *ReverseAuthorizationResponse

	err := s.dataStore.Atomic(ctx, func(repos domain.Repositories) error {
		now := time.Now()

		// Get authorization
		auth, err := repos.Authorizations().FindByID(ctx, req.TenantID, req.AuthorizationID)
		if err != nil {
			return err
		}

		// Get card account to release the amount
		cardAccount, err := repos.CardAccounts().FindByID(ctx, req.TenantID, auth.CardAccountID())
		if err != nil {
			return err
		}

		// Reverse authorization
		if err := auth.Reverse(now); err != nil {
			return err
		}

		// Release the authorized amount from the card account
		if err := cardAccount.ReleaseAmount(auth.AuthorizedAmount(), now); err != nil {
			return err
		}

		// Save card account
		if err := repos.CardAccounts().Save(ctx, cardAccount); err != nil {
			return err
		}

		// Save authorization
		if err := repos.Authorizations().Save(ctx, auth); err != nil {
			return err
		}

		// Write event to outbox
		outboxEntry, err := domain.NewSpendReversedOutboxEntry(auth, req.CorrelationID)
		if err != nil {
			return err
		}
		if err := repos.Outbox().Append(ctx, outboxEntry); err != nil {
			return err
		}

		// Prepare response
		result = &ReverseAuthorizationResponse{
			AuthorizationID: auth.ID().String(),
			Status:          string(auth.State()),
		}

		// Atomically store idempotency entry - prevents TOCTOU race
		if err := storeIdempotency(ctx, repos.IdempotencyStore(), req.TenantID, req.IdempotencyKey,
			auth.ID().String(), http.StatusOK, result, now); err != nil {
			return err
		}

		logging.InfoContext(ctx, "Authorization reversed",
			"authorization_id", auth.ID().String(),
			"tenant_id", req.TenantID.String(),
			"reversed_amount", auth.AuthorizedAmount().String(),
		)

		return nil
	})

	if conflict, conflictErr := handleIdempotencyConflict[ReverseAuthorizationResponse](err); conflictErr != nil {
		return nil, conflictErr
	} else if conflict != nil {
		return conflict, nil
	}

	return result, err
}

// GetCardAccountResponse represents the response from getting a card account.
type GetCardAccountResponse struct {
	CardAccountID  string      `json:"card_account_id"`
	SpendingLimit  types.Money `json:"spending_limit"`
	RollingSpend   types.Money `json:"rolling_spend"`
	AvailableLimit types.Money `json:"available_limit"`
}

// GetCardAccount retrieves a card account by tenant ID.
// It loads the account and returns its limits along with the calculated available amount.
func (s *SpendingService) GetCardAccount(ctx context.Context, tenantID types.TenantID) (*GetCardAccountResponse, error) {
	cardAccount, err := s.repos.CardAccounts().FindByTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	return &GetCardAccountResponse{
		CardAccountID:  cardAccount.ID().String(),
		SpendingLimit:  cardAccount.SpendingLimit(),
		RollingSpend:   cardAccount.RollingSpend(),
		AvailableLimit: cardAccount.AvailableLimit(),
	}, nil
}
