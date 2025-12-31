package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"aurum/internal/common/logging"
	vo "aurum/internal/common/value_objects"
	"aurum/internal/spending/application"
	"aurum/internal/spending/domain"
)

// Handler handles HTTP requests for the spending context.
type Handler struct {
	service *application.SpendingService
}

// NewHandler creates a new Handler.
func NewHandler(service *application.SpendingService) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the spending routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /authorizations", h.CreateAuthorization)
	mux.HandleFunc("GET /authorizations/{id}", h.GetAuthorization)
	mux.HandleFunc("POST /authorizations/{id}/capture", h.CaptureAuthorization)
}

// CreateAuthorizationRequest is the JSON request body for creating an authorization.
type CreateAuthorizationRequest struct {
	CardAccountID string `json:"card_account_id"`
	Amount        struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
}

// AuthorizationResponse is the JSON response for an authorization.
type AuthorizationResponse struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"authorized_amount"`
	CapturedAmount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"captured_amount"`
}

// ErrorResponse is the JSON response for errors.
type ErrorResponse struct {
	Error string `json:"error"`
}

// CreateAuthorization handles POST /authorizations.
func (h *Handler) CreateAuthorization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse and validate tenant ID from header
	tenantID, err := vo.ParseTenantID(r.Header.Get("X-Tenant-ID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "X-Tenant-ID header is required")
		return
	}

	// Get idempotency key from header
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}

	// Parse request body
	var req CreateAuthorizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Parse and validate card account ID
	cardAccountID, err := domain.ParseCardAccountID(req.CardAccountID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid card_account_id")
		return
	}

	// Parse amount (must be positive for authorizations)
	amount, err := vo.NewPositiveFromString(req.Amount.Value, req.Amount.Currency)
	if err != nil {
		if errors.Is(err, vo.ErrNonPositiveAmount) {
			writeError(w, http.StatusBadRequest, "amount must be positive")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid amount")
		return
	}

	// Create authorization
	resp, err := h.service.CreateAuthorization(ctx, application.CreateAuthorizationRequest{
		TenantID:       tenantID,
		IdempotencyKey: idempotencyKey,
		CardAccountID:  cardAccountID,
		Amount:         amount,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	logging.InfoContext(ctx, "Authorization created",
		"authorization_id", resp.ID.String(),
		"idempotent", resp.IsIdempotent,
	)

	status := http.StatusCreated
	if resp.IsIdempotent {
		status = http.StatusOK
	}

	writeJSON(w, status, toAuthorizationResponse(resp.ID.String(), resp.State, resp.AuthorizedAmount, resp.CapturedAmount))
}

// GetAuthorization handles GET /authorizations/{id}.
func (h *Handler) GetAuthorization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse and validate tenant ID from header
	tenantID, err := vo.ParseTenantID(r.Header.Get("X-Tenant-ID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "X-Tenant-ID header is required")
		return
	}

	// Parse and validate authorization ID
	authID, err := domain.ParseAuthorizationID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid authorization id")
		return
	}

	resp, err := h.service.GetAuthorization(ctx, tenantID, authID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toAuthorizationResponse(resp.ID.String(), resp.State, resp.AuthorizedAmount, resp.CapturedAmount))
}

// CaptureAuthorizationRequest is the JSON request body for capturing.
type CaptureAuthorizationRequest struct {
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
}

// CaptureAuthorization handles POST /authorizations/{id}/capture.
func (h *Handler) CaptureAuthorization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse and validate tenant ID from header
	tenantID, err := vo.ParseTenantID(r.Header.Get("X-Tenant-ID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "X-Tenant-ID header is required")
		return
	}

	// Parse and validate authorization ID
	authID, err := domain.ParseAuthorizationID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid authorization id")
		return
	}

	var req CaptureAuthorizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Parse amount (must be positive for captures)
	amount, err := vo.NewPositiveFromString(req.Amount.Value, req.Amount.Currency)
	if err != nil {
		if errors.Is(err, vo.ErrNonPositiveAmount) {
			writeError(w, http.StatusBadRequest, "amount must be positive")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid amount")
		return
	}

	resp, err := h.service.CaptureAuthorization(ctx, application.CaptureAuthorizationRequest{
		TenantID:        tenantID,
		AuthorizationID: authID,
		Amount:          amount,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	logging.InfoContext(ctx, "Authorization captured",
		"authorization_id", resp.ID.String(),
		"captured_amount", resp.CapturedAmount.String(),
	)

	writeJSON(w, http.StatusOK, toAuthorizationResponse(resp.ID.String(), resp.State, resp.AuthorizedAmount, resp.CapturedAmount))
}

func toAuthorizationResponse(id string, state domain.AuthorizationState, authorizedAmount, capturedAmount vo.Money) AuthorizationResponse {
	return AuthorizationResponse{
		ID:    id,
		State: string(state),
		Amount: struct {
			Value    string `json:"value"`
			Currency string `json:"currency"`
		}{
			Value:    authorizedAmount.Amount.StringFixed(2),
			Currency: authorizedAmount.Currency.String(),
		},
		CapturedAmount: struct {
			Value    string `json:"value"`
			Currency string `json:"currency"`
		}{
			Value:    capturedAmount.Amount.StringFixed(2),
			Currency: capturedAmount.Currency.String(),
		},
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logging.Error("Failed to encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}

func handleServiceError(w http.ResponseWriter, err error) {
	// Use errors.As for type-safe error matching
	var spendingLimitErr domain.ErrSpendingLimitExceeded
	var alreadyCapturedErr domain.ErrAlreadyCaptured
	var exceedsAmountErr domain.ErrExceedsAuthorizedAmount
	var invalidTransitionErr domain.ErrInvalidStateTransition
	var currencyMismatchErr domain.ErrCurrencyMismatch

	switch {
	case errors.Is(err, application.ErrAuthorizationNotFound):
		writeError(w, http.StatusNotFound, "authorization not found")
	case errors.Is(err, application.ErrCardAccountNotFound):
		writeError(w, http.StatusNotFound, "card account not found")
	case errors.As(err, &spendingLimitErr):
		writeError(w, http.StatusUnprocessableEntity, "spending limit exceeded")
	case errors.As(err, &alreadyCapturedErr):
		writeError(w, http.StatusConflict, "already captured")
	case errors.As(err, &exceedsAmountErr):
		writeError(w, http.StatusUnprocessableEntity, "exceeds authorized amount")
	case errors.As(err, &invalidTransitionErr):
		writeError(w, http.StatusConflict, "invalid state transition")
	case errors.As(err, &currencyMismatchErr):
		writeError(w, http.StatusUnprocessableEntity, "currency mismatch")
	default:
		logging.Error("Internal error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
