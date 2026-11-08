package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
)

// operatorLiveness answers the owner's verdict on the operator's session.
type operatorLiveness func(ctx context.Context, operatorID int64, familyID string) error

func (f operatorLiveness) VerifyOperatorSession(ctx context.Context, operatorID int64, familyID string) error {
	return f(ctx, operatorID, familyID)
}

func TestRequiresOperatorScope_ValidOperatorToken(t *testing.T) {
	t.Parallel()

	// Create a next handler that records if it was called
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	handler := identityoperator.RequiresOperatorScope(nextHandler)

	// Create request with operator scope claims
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	claims := routetest.Claims{
		ID:    1,
		Scope: "platform",
	}
	ctx := routetest.WithAuthenticatedContext(req.Context(), claims, nil)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, nextCalled, "next handler should have been called")
}

func TestRequiresOperatorScope_TenantToken(t *testing.T) {
	t.Parallel()

	// Create a next handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	handler := identityoperator.RequiresOperatorScope(nextHandler)

	// Create request with tenant scope claims
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	claims := routetest.Claims{
		ID:    1,
		Scope: "tenant",
	}
	ctx := routetest.WithAuthenticatedContext(req.Context(), claims, nil)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "operator authentication")
}

func TestRequiresOperatorScope_NoClaims(t *testing.T) {
	t.Parallel()

	// Create a next handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	handler := identityoperator.RequiresOperatorScope(nextHandler)

	// Create request without claims (should get empty claims)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRequiresActiveOperator_LiveSessionPasses(t *testing.T) {
	t.Parallel()

	nextCalled := false
	handler := identityoperator.RequiresActiveOperator(operatorLiveness(func(_ context.Context, operatorID int64, familyID string) error {
		assert.Equal(t, int64(42), operatorID)
		assert.Equal(t, "operator-test-family", familyID)
		return nil
	}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, requestWithOperatorClaims(42))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, nextCalled, "next handler should have been called")
}

// The owner's refusals keep their answers: an inactive operator or an ended
// family is 403, an unknown operator answers like wrong credentials, and a
// check that could not run fails closed with 503.
func TestRequiresActiveOperator_RefusalsAndFailures(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		verdict error
		status  int
		body    string
	}{
		{"inactive or ended family", identityaccess.ErrOperatorInactive, http.StatusForbidden, "Operator account is inactive"},
		{"unknown operator", identityaccess.ErrOperatorNotFound, http.StatusUnauthorized, "Invalid email or password"},
		{"lookup failed", errors.New("database unavailable"), http.StatusServiceUnavailable, "Operator status temporarily unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			nextCalled := false
			handler := identityoperator.RequiresActiveOperator(operatorLiveness(func(context.Context, int64, string) error {
				return tc.verdict
			}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
			}))
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, requestWithOperatorClaims(42))
			assert.Equal(t, tc.status, rr.Code)
			assert.Contains(t, rr.Body.String(), tc.body)
			assert.False(t, nextCalled)
		})
	}
}

func TestRequiresActiveOperator_MissingLookupFailsClosed(t *testing.T) {
	t.Parallel()

	nextCalled := false
	handler := identityoperator.RequiresActiveOperator(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, requestWithOperatorClaims(42))

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Contains(t, rr.Body.String(), "Operator status temporarily unavailable")
	assert.False(t, nextCalled, "missing operator lookup must not reach protected handlers")
}

func TestRequiresActiveOperator_MissingOperatorID(t *testing.T) {
	t.Parallel()

	nextCalled := false
	handler := identityoperator.RequiresActiveOperator(operatorLiveness(func(context.Context, int64, string) error {
		t.Fatal("the session check must not run without an authenticated operator id")
		return nil
	}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/test", nil))

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.False(t, nextCalled, "requests without operator identity must not reach protected handlers")
}

func requestWithOperatorClaims(operatorID int) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	claims := routetest.Claims{ID: operatorID, Scope: "platform", FamilyID: "operator-test-family"}
	ctx := routetest.WithAuthenticatedContext(req.Context(), claims, nil)
	return req.WithContext(ctx)
}
