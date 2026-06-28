package operator_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/operator"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

type operatorLookupStub struct {
	getOperatorFn func(context.Context, int64) (*platformModels.Operator, error)
}

func (s operatorLookupStub) GetOperator(ctx context.Context, id int64) (*platformModels.Operator, error) {
	if s.getOperatorFn != nil {
		return s.getOperatorFn(ctx, id)
	}
	return nil, nil
}

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

func TestRequiresActiveOperator_ActiveOperator(t *testing.T) {
	nextCalled := false
	handler := operator.RequiresActiveOperator(operatorLookupStub{
		getOperatorFn: func(_ context.Context, id int64) (*platformModels.Operator, error) {
			assert.Equal(t, int64(42), id)
			op := &platformModels.Operator{Active: true}
			op.ID = id
			return op, nil
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithOperatorClaims(42)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, nextCalled, "next handler should have been called")
}

func TestRequiresActiveOperator_MissingLookupFailsClosed(t *testing.T) {
	nextCalled := false
	handler := operator.RequiresActiveOperator(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithOperatorClaims(42)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Contains(t, rr.Body.String(), "Operator status temporarily unavailable")
	assert.False(t, nextCalled, "missing operator lookup must not reach protected handlers")
}

func TestRequiresActiveOperator_InactiveOperator(t *testing.T) {
	nextCalled := false
	handler := operator.RequiresActiveOperator(operatorLookupStub{
		getOperatorFn: func(_ context.Context, id int64) (*platformModels.Operator, error) {
			op := &platformModels.Operator{Active: false}
			op.ID = id
			return op, nil
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithOperatorClaims(42)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "Operator account is inactive")
	assert.False(t, nextCalled, "inactive operators must not reach protected handlers")
}

func TestRequiresActiveOperator_NilOperatorFailsClosed(t *testing.T) {
	nextCalled := false
	handler := operator.RequiresActiveOperator(operatorLookupStub{
		getOperatorFn: func(context.Context, int64) (*platformModels.Operator, error) {
			return nil, nil
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithOperatorClaims(42)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid email or password")
	assert.False(t, nextCalled, "nil operator lookups must not reach protected handlers")
}

func TestRequiresActiveOperator_OperatorNotFound(t *testing.T) {
	nextCalled := false
	handler := operator.RequiresActiveOperator(operatorLookupStub{
		getOperatorFn: func(context.Context, int64) (*platformModels.Operator, error) {
			return nil, &platformSvc.OperatorNotFoundError{OperatorID: 42}
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithOperatorClaims(42)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.False(t, nextCalled, "missing operators must not reach protected handlers")
}

func TestRequiresActiveOperator_LookupErrorFailsClosed(t *testing.T) {
	nextCalled := false
	handler := operator.RequiresActiveOperator(operatorLookupStub{
		getOperatorFn: func(context.Context, int64) (*platformModels.Operator, error) {
			return nil, errors.New("database unavailable")
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithOperatorClaims(42)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.False(t, nextCalled, "lookup failures must not reach protected handlers")
}

func TestRequiresActiveOperator_MissingOperatorID(t *testing.T) {
	nextCalled := false
	handler := operator.RequiresActiveOperator(operatorLookupStub{
		getOperatorFn: func(context.Context, int64) (*platformModels.Operator, error) {
			t.Fatal("operator lookup should not run without an authenticated operator id")
			return nil, nil
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.False(t, nextCalled, "requests without operator identity must not reach protected handlers")
}

func requestWithOperatorClaims(operatorID int) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	claims := jwt.AppClaims{ID: operatorID, Scope: "platform"}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	return req.WithContext(ctx)
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
