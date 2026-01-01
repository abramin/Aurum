package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"

	"aurum/internal/common/types"
	"aurum/internal/spending/api"
	"aurum/internal/spending/application"
	"aurum/internal/spending/infrastructure/memory"
)

// HandlerSuite tests HTTP handler behavior including error mapping.
//
// Justification: Error-to-status-code mapping is a boundary concern that requires
// HTTP-level testing. Domain errors must translate to appropriate HTTP responses.
type HandlerSuite struct {
	suite.Suite
	mux     *http.ServeMux
	service *application.SpendingService
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(HandlerSuite))
}

func (s *HandlerSuite) SetupTest() {
	dataStore := memory.NewDataStore()
	s.service = application.NewSpendingService(dataStore)
	handler := api.NewHandler(s.service)

	s.mux = http.NewServeMux()
	handler.RegisterRoutes(s.mux)
}

func (s *HandlerSuite) createCardAccount(tenantID string, limitAmount int64) {
	limit := types.NewMoney(decimal.NewFromInt(limitAmount), types.CurrencyEUR)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := s.service.CreateCardAccount(req.Context(), application.CreateCardAccountRequest{
		TenantID:      types.TenantID(tenantID),
		SpendingLimit: limit,
	})
	s.Require().NoError(err)
}

func (s *HandlerSuite) doRequest(method, path string, body any) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	return rec
}

func (s *HandlerSuite) TestErrorMapping() {
	s.Run("ErrCardAccountNotFound returns 404", func() {
		body := map[string]any{
			"tenant_id":       "nonexistent-tenant",
			"idempotency_key": "idem-1",
			"amount":          map[string]any{"value": "100.00", "currency": "EUR"},
			"merchant_ref":    "test",
			"reference":       "ref-1",
		}

		rec := s.doRequest(http.MethodPost, "/authorizations", body)

		s.Equal(http.StatusNotFound, rec.Code)
		s.Contains(rec.Body.String(), "card account not found")
	})

	s.Run("ErrSpendingLimitExceeded returns 422", func() {
		s.createCardAccount("tenant-limit", 100) // 100 EUR limit

		body := map[string]any{
			"tenant_id":       "tenant-limit",
			"idempotency_key": "idem-exceed",
			"amount":          map[string]any{"value": "500.00", "currency": "EUR"},
			"merchant_ref":    "test",
			"reference":       "ref-1",
		}

		rec := s.doRequest(http.MethodPost, "/authorizations", body)

		s.Equal(http.StatusUnprocessableEntity, rec.Code)
		s.Contains(rec.Body.String(), "spending limit exceeded")
	})

	s.Run("ErrCurrencyMismatch returns 400", func() {
		s.createCardAccount("tenant-currency", 1000) // EUR account

		body := map[string]any{
			"tenant_id":       "tenant-currency",
			"idempotency_key": "idem-currency",
			"amount":          map[string]any{"value": "100.00", "currency": "USD"},
			"merchant_ref":    "test",
			"reference":       "ref-1",
		}

		rec := s.doRequest(http.MethodPost, "/authorizations", body)

		s.Equal(http.StatusBadRequest, rec.Code)
		s.Contains(rec.Body.String(), "currency mismatch")
	})

	s.Run("ErrAuthorizationNotFound returns 404", func() {
		rec := s.doRequest(http.MethodGet, "/authorizations/00000000-0000-0000-0000-000000000001?tenant_id=tenant-1", nil)

		s.Equal(http.StatusNotFound, rec.Code)
		s.Contains(rec.Body.String(), "authorization not found")
	})

	s.Run("ErrAlreadyCaptured returns 409", func() {
		s.createCardAccount("tenant-double-cap", 1000)

		// Create authorization
		authBody := map[string]any{
			"tenant_id":       "tenant-double-cap",
			"idempotency_key": "idem-auth",
			"amount":          map[string]any{"value": "100.00", "currency": "EUR"},
			"merchant_ref":    "test",
			"reference":       "ref-1",
		}
		authRec := s.doRequest(http.MethodPost, "/authorizations", authBody)
		s.Require().Equal(http.StatusCreated, authRec.Code)

		var authResp map[string]string
		json.Unmarshal(authRec.Body.Bytes(), &authResp)
		authID := authResp["authorization_id"]

		// First capture
		captureBody := map[string]any{
			"tenant_id":       "tenant-double-cap",
			"idempotency_key": "idem-cap-1",
			"amount":          map[string]any{"value": "100.00", "currency": "EUR"},
		}
		rec1 := s.doRequest(http.MethodPost, "/authorizations/"+authID+"/capture", captureBody)
		s.Require().Equal(http.StatusOK, rec1.Code)

		// Second capture (different idempotency key)
		captureBody["idempotency_key"] = "idem-cap-2"
		rec2 := s.doRequest(http.MethodPost, "/authorizations/"+authID+"/capture", captureBody)

		s.Equal(http.StatusConflict, rec2.Code)
		s.Contains(rec2.Body.String(), "already captured")
	})

	s.Run("ErrExceedsAuthorizedAmount returns 400", func() {
		s.createCardAccount("tenant-exceed-auth", 1000)

		// Create authorization for 100 EUR
		authBody := map[string]any{
			"tenant_id":       "tenant-exceed-auth",
			"idempotency_key": "idem-auth-exceed",
			"amount":          map[string]any{"value": "100.00", "currency": "EUR"},
			"merchant_ref":    "test",
			"reference":       "ref-1",
		}
		authRec := s.doRequest(http.MethodPost, "/authorizations", authBody)
		s.Require().Equal(http.StatusCreated, authRec.Code)

		var authResp map[string]string
		json.Unmarshal(authRec.Body.Bytes(), &authResp)
		authID := authResp["authorization_id"]

		// Try to capture 150 EUR (more than authorized)
		captureBody := map[string]any{
			"tenant_id":       "tenant-exceed-auth",
			"idempotency_key": "idem-cap-exceed",
			"amount":          map[string]any{"value": "150.00", "currency": "EUR"},
		}
		rec := s.doRequest(http.MethodPost, "/authorizations/"+authID+"/capture", captureBody)

		s.Equal(http.StatusBadRequest, rec.Code)
		s.Contains(rec.Body.String(), "exceeds authorized amount")
	})
}

func (s *HandlerSuite) TestRequestValidation() {
	s.Run("missing tenant_id returns 400", func() {
		body := map[string]any{
			"idempotency_key": "idem-1",
			"amount":          map[string]any{"value": "100.00", "currency": "EUR"},
			"merchant_ref":    "test",
			"reference":       "ref-1",
		}

		rec := s.doRequest(http.MethodPost, "/authorizations", body)

		s.Equal(http.StatusBadRequest, rec.Code)
		s.Contains(rec.Body.String(), "tenant_id is required")
	})

	s.Run("missing idempotency_key returns 400", func() {
		body := map[string]any{
			"tenant_id":    "tenant-1",
			"amount":       map[string]any{"value": "100.00", "currency": "EUR"},
			"merchant_ref": "test",
			"reference":    "ref-1",
		}

		rec := s.doRequest(http.MethodPost, "/authorizations", body)

		s.Equal(http.StatusBadRequest, rec.Code)
		s.Contains(rec.Body.String(), "idempotency_key is required")
	})

	s.Run("invalid JSON returns 400", func() {
		req := httptest.NewRequest(http.MethodPost, "/authorizations", bytes.NewBufferString("{invalid"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, req)

		s.Equal(http.StatusBadRequest, rec.Code)
		s.Contains(rec.Body.String(), "invalid request body")
	})

	s.Run("invalid authorization_id format returns 400", func() {
		rec := s.doRequest(http.MethodGet, "/authorizations/not-a-uuid?tenant_id=tenant-1", nil)

		s.Equal(http.StatusBadRequest, rec.Code)
		s.Contains(rec.Body.String(), "invalid authorization_id")
	})

	s.Run("zero amount returns 400", func() {
		body := map[string]any{
			"tenant_id":       "tenant-1",
			"idempotency_key": "idem-1",
			"amount":          map[string]any{"value": "0.00", "currency": "EUR"},
			"merchant_ref":    "test",
			"reference":       "ref-1",
		}

		rec := s.doRequest(http.MethodPost, "/authorizations", body)

		s.Equal(http.StatusBadRequest, rec.Code)
		s.Contains(rec.Body.String(), "amount must be positive")
	})
}

func (s *HandlerSuite) TestSuccessfulResponses() {
	s.Run("CreateAuthorization returns 201 with authorization_id", func() {
		s.createCardAccount("tenant-success", 1000)

		body := map[string]any{
			"tenant_id":       "tenant-success",
			"idempotency_key": "idem-success",
			"amount":          map[string]any{"value": "100.00", "currency": "EUR"},
			"merchant_ref":    "test-merchant",
			"reference":       "ref-success",
		}

		rec := s.doRequest(http.MethodPost, "/authorizations", body)

		s.Equal(http.StatusCreated, rec.Code)

		var resp map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		s.Require().NoError(err)
		s.NotEmpty(resp["authorization_id"])
		s.Equal("authorized", resp["status"])
	})

	s.Run("GetAuthorization returns 200 with authorization details", func() {
		s.createCardAccount("tenant-get", 1000)

		// Create authorization first
		authBody := map[string]any{
			"tenant_id":       "tenant-get",
			"idempotency_key": "idem-get",
			"amount":          map[string]any{"value": "50.00", "currency": "EUR"},
			"merchant_ref":    "test-merchant",
			"reference":       "ref-get",
		}
		authRec := s.doRequest(http.MethodPost, "/authorizations", authBody)
		s.Require().Equal(http.StatusCreated, authRec.Code)

		var authResp map[string]string
		json.Unmarshal(authRec.Body.Bytes(), &authResp)
		authID := authResp["authorization_id"]

		// Get authorization
		rec := s.doRequest(http.MethodGet, "/authorizations/"+authID+"?tenant_id=tenant-get", nil)

		s.Equal(http.StatusOK, rec.Code)

		var getResp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &getResp)
		s.Require().NoError(err)
		s.Equal(authID, getResp["authorization_id"])
		s.Equal("authorized", getResp["status"])
	})

	s.Run("CaptureAuthorization returns 200 with captured amount", func() {
		s.createCardAccount("tenant-capture", 1000)

		// Create authorization
		authBody := map[string]any{
			"tenant_id":       "tenant-capture",
			"idempotency_key": "idem-auth-cap",
			"amount":          map[string]any{"value": "100.00", "currency": "EUR"},
			"merchant_ref":    "test",
			"reference":       "ref-1",
		}
		authRec := s.doRequest(http.MethodPost, "/authorizations", authBody)
		s.Require().Equal(http.StatusCreated, authRec.Code)

		var authResp map[string]string
		json.Unmarshal(authRec.Body.Bytes(), &authResp)
		authID := authResp["authorization_id"]

		// Capture authorization
		captureBody := map[string]any{
			"tenant_id":       "tenant-capture",
			"idempotency_key": "idem-cap-success",
			"amount":          map[string]any{"value": "75.00", "currency": "EUR"},
		}
		rec := s.doRequest(http.MethodPost, "/authorizations/"+authID+"/capture", captureBody)

		s.Equal(http.StatusOK, rec.Code)

		var capResp map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &capResp)
		s.Require().NoError(err)
		s.Equal("captured", capResp["status"])
		s.Contains(capResp["captured_amount"], "75")
	})
}
