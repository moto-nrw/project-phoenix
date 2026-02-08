package operator_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/operator"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

func TestRequiresOperatorScope_ValidOperatorToken(t *testing.T) {
	// Create a next handler that records if it was called
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	handler := operator.RequiresOperatorScope(nextHandler)

	// Create request with operator scope claims
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	claims := jwt.AppClaims{
		ID:    1,
		Scope: "platform",
	}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, nextCalled, "next handler should have been called")
}

func TestRequiresOperatorScope_TenantToken(t *testing.T) {
	// Create a next handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	handler := operator.RequiresOperatorScope(nextHandler)

	// Create request with tenant scope claims
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	claims := jwt.AppClaims{
		ID:    1,
		Scope: "tenant",
	}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "operator authentication")
}

func TestRequiresOperatorScope_NoClaims(t *testing.T) {
	// Create a next handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	handler := operator.RequiresOperatorScope(nextHandler)

	// Create request without claims (should get empty claims)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestErrResponse_Render(t *testing.T) {
	errResp := &operator.ErrResponse{
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "error",
		ErrorText:      "test error",
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	err := errResp.Render(rr, req)
	require.NoError(t, err)

	// The Render method sets the status code in the request context
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
}
