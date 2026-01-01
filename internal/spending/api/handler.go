package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"aurum/internal/common/logging"
	"aurum/internal/common/types"
	"aurum/internal/spending/application"
	"aurum/internal/spending/domain"
)

// Handler implements the HTTP handlers for the Spending API.
type Handler struct {
	service *application.SpendingService
}

// NewHandler creates a new Handler.
func NewHandler(service *application.SpendingService) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the Spending API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /authorizations", h.CreateAuthorization)
	mux.HandleFunc("GET /authorizations/{id}", h.GetAuthorization)
	mux.HandleFunc("POST /authorizations/{id}/capture", h.CaptureAuthorization)
	mux.HandleFunc("POST /card-accounts", h.CreateCardAccount)
}

// CreateAuthorizationRequest is the JSON request body for creating an authorization.
type CreateAuthorizationRequest struct {
	TenantID       string      `json:"tenant_id"`
	IdempotencyKey string      `json:"idempotency_key"`
	Amount         types.Money `json:"amount"`
	MerchantRef    string      `json:"merchant_ref"`
	Reference      string      `json:"reference"`
}

// CreateAuthorization handles POST /authorizations.
func (h *Handler) CreateAuthorization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateAuthorizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TenantID == "" {
		h.writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	if req.IdempotencyKey == "" {
		h.writeError(w, http.StatusBadRequest, "idempotency_key is required")
		return
	}
	if req.Amount.IsZero() || !req.Amount.IsPositive() {
		h.writeError(w, http.StatusBadRequest, "amount must be positive")
		return
	}

	correlationID := types.CorrelationID(r.Header.Get("X-Correlation-ID"))
	if correlationID.IsEmpty() {
		correlationID = types.NewCorrelationID()
	}

	resp, err := h.service.CreateAuthorization(ctx, application.CreateAuthorizationRequest{
		TenantID:       types.TenantID(req.TenantID),
		IdempotencyKey: req.IdempotencyKey,
		Amount:         req.Amount,
		MerchantRef:    req.MerchantRef,
		Reference:      req.Reference,
		CorrelationID:  correlationID,
	})
	if err != nil {
		h.handleDomainError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, resp)
}

// GetAuthorization handles GET /authorizations/{id}.
func (h *Handler) GetAuthorization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		h.writeError(w, http.StatusBadRequest, "tenant_id query parameter is required")
		return
	}

	authID, err := domain.ParseAuthorizationID(r.PathValue("id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid authorization_id")
		return
	}

	resp, err := h.service.GetAuthorization(ctx, application.GetAuthorizationRequest{
		TenantID:        types.TenantID(tenantID),
		AuthorizationID: authID,
	})
	if err != nil {
		h.handleDomainError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// CaptureAuthorizationRequest is the JSON request body for capturing an authorization.
type CaptureAuthorizationRequest struct {
	TenantID       string      `json:"tenant_id"`
	IdempotencyKey string      `json:"idempotency_key"`
	Amount         types.Money `json:"amount"`
}

// CaptureAuthorization handles POST /authorizations/{id}/capture.
func (h *Handler) CaptureAuthorization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CaptureAuthorizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TenantID == "" {
		h.writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	if req.IdempotencyKey == "" {
		h.writeError(w, http.StatusBadRequest, "idempotency_key is required")
		return
	}

	authID, err := domain.ParseAuthorizationID(r.PathValue("id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid authorization_id")
		return
	}

	correlationID := types.CorrelationID(r.Header.Get("X-Correlation-ID"))
	if correlationID.IsEmpty() {
		correlationID = types.NewCorrelationID()
	}

	resp, err := h.service.CaptureAuthorization(ctx, application.CaptureAuthorizationRequest{
		TenantID:        types.TenantID(req.TenantID),
		AuthorizationID: authID,
		IdempotencyKey:  req.IdempotencyKey,
		Amount:          req.Amount,
		CorrelationID:   correlationID,
	})
	if err != nil {
		h.handleDomainError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// CreateCardAccountRequest is the JSON request body for creating a card account.
type CreateCardAccountRequest struct {
	TenantID      string      `json:"tenant_id"`
	SpendingLimit types.Money `json:"spending_limit"`
}

// CreateCardAccount handles POST /card-accounts.
func (h *Handler) CreateCardAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateCardAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TenantID == "" {
		h.writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	if req.SpendingLimit.IsZero() || !req.SpendingLimit.IsPositive() {
		h.writeError(w, http.StatusBadRequest, "spending_limit must be positive")
		return
	}

	resp, err := h.service.CreateCardAccount(ctx, application.CreateCardAccountRequest{
		TenantID:      types.TenantID(req.TenantID),
		SpendingLimit: req.SpendingLimit,
	})
	if err != nil {
		h.handleDomainError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, resp)
}

// handleDomainError maps domain errors to HTTP responses.
// Internal error details are logged but never exposed to clients.
func (h *Handler) handleDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrAuthorizationNotFound):
		h.writeError(w, http.StatusNotFound, "authorization not found")
	case errors.Is(err, domain.ErrCardAccountNotFound):
		h.writeError(w, http.StatusNotFound, "card account not found")
	case errors.Is(err, domain.ErrAlreadyCaptured):
		h.writeError(w, http.StatusConflict, "authorization already captured")
	case errors.Is(err, domain.ErrInvalidStateTransition):
		h.writeError(w, http.StatusConflict, "invalid state transition")
	case errors.Is(err, domain.ErrExceedsAuthorizedAmount):
		h.writeError(w, http.StatusBadRequest, "capture amount exceeds authorized amount")
	case errors.Is(err, domain.ErrSpendingLimitExceeded):
		h.writeError(w, http.StatusUnprocessableEntity, "spending limit exceeded")
	case errors.Is(err, domain.ErrCurrencyMismatch):
		h.writeError(w, http.StatusBadRequest, "currency mismatch")
	case errors.Is(err, domain.ErrOptimisticLock):
		h.writeError(w, http.StatusConflict, "concurrent modification detected, please retry")
	case errors.Is(err, domain.ErrEmptyTenantID):
		h.writeError(w, http.StatusBadRequest, "tenant_id is required")
	case errors.Is(err, domain.ErrCorruptData):
		logging.Error("Corrupt data detected", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
	default:
		logging.Error("Unhandled error", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

// writeJSON writes a JSON response.
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// writeError writes an error response.
// Note: Internal error details are never exposed - they should be logged separately.
func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, ErrorResponse{Error: message})
}
