package common

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collectObservations(observed *[]TenantRuntimeObservation) TenantRuntimeObserver {
	return func(observation TenantRuntimeObservation) { *observed = append(*observed, observation) }
}

// Regression for #2953: a service-owned transaction (as under /auth/refresh)
// rolls back on an expired token and the handler answers 401. The observer
// must see that 401, not a status-less event, so the composition root can
// tell an expected rollback from a failure.
func TestTenantRuntimeObserverReportsFinalStatusForServiceOwnedRollback(t *testing.T) {
	t.Parallel()
	runtime := testRuntime(t, func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
		return fn(ctx, struct{}{})
	})
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	tokenExpired := errors.New("refresh token expired")

	var observed []TenantRuntimeObservation
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := tenant.WithinTenant(r.Context(), id, func(context.Context) error { return tokenExpired })
		require.ErrorIs(t, err, tokenExpired)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	wrapped := TenantRuntimeObserverMiddleware(collectObservations(&observed))(handler)
	request := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	request = request.WithContext(tenant.WithUnitOfWork(request.Context(), runtime))

	wrapped.ServeHTTP(httptest.NewRecorder(), request)

	require.Len(t, observed, 1)
	assert.Equal(t, http.StatusUnauthorized, observed[0].Status)
	assert.Equal(t, "/auth/refresh", observed[0].Request.URL.Path)
	assert.Equal(t, TenantRuntimeTransaction, observed[0].Event.Kind)
	assert.Equal(t, tenant.UnitOfWorkRolledBack, observed[0].Event.Result)
	assert.ErrorIs(t, observed[0].Event.Err, tokenExpired)
}

func TestTenantRuntimeObserverReportsFinalStatusForMiddlewareRollback(t *testing.T) {
	t.Parallel()
	runtime := testRuntime(t, func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
		return fn(ctx, struct{}{})
	})
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)

	var observed []TenantRuntimeObservation
	handler := TenantTxMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant.MarkRollback(r.Context())
		http.Error(w, "gone", http.StatusGone)
	}))
	wrapped := TenantRuntimeObserverMiddleware(collectObservations(&observed))(handler)
	request := httptest.NewRequest(http.MethodGet, "/auth/guardian-invitations/abc", nil)
	request = request.WithContext(tenant.WithUnitOfWork(tenant.WithTenant(request.Context(), id), runtime))
	recorder := httptest.NewRecorder()

	wrapped.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusGone, recorder.Code)
	require.Len(t, observed, 1)
	assert.Equal(t, http.StatusGone, observed[0].Status)
	assert.Equal(t, TenantRuntimeTransaction, observed[0].Event.Kind)
	require.Error(t, observed[0].Event.Err)
}

func TestTenantRuntimeObserverReportsServerErrorStatusForFailedTransaction(t *testing.T) {
	t.Parallel()
	runtime := testRuntime(t, func(context.Context, int64, func(context.Context, any) error) error {
		return assert.AnError
	})
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)

	var observed []TenantRuntimeObservation
	handler := TenantTxMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler must not run when transaction setup fails")
	}))
	wrapped := TenantRuntimeObserverMiddleware(collectObservations(&observed))(handler)
	request := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	request = request.WithContext(tenant.WithUnitOfWork(tenant.WithTenant(request.Context(), id), runtime))

	wrapped.ServeHTTP(httptest.NewRecorder(), request)

	require.Len(t, observed, 1)
	assert.Equal(t, http.StatusInternalServerError, observed[0].Status)
	assert.ErrorIs(t, observed[0].Event.Err, assert.AnError)
}

// A streaming handler (SSE) sends its header first and keeps the connection
// open; events raised afterwards must reach the observer while the handler
// is still running instead of piling up until the client disconnects, and
// they must not overtake events that were still buffered.
func TestTenantRuntimeObserverDeliversEventsInOrderOnceHeaderWasSent(t *testing.T) {
	t.Parallel()
	var observed []TenantRuntimeObservation
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant.ObserveMissingTenant(r.Context(), tenant.ErrTenantRequired)
		require.Empty(t, observed, "event before the header must wait for the status")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: ping\n\n"))
		tenant.ObservePoolWait(r.Context(), 0)
		require.Len(t, observed, 2, "event after the header must not wait for the handler to finish")
	})
	wrapped := TenantRuntimeObserverMiddleware(collectObservations(&observed))(handler)

	wrapped.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/sse/events", nil))

	require.Len(t, observed, 2)
	assert.Equal(t, TenantRuntimeMissingTenant, observed[0].Event.Kind)
	assert.Equal(t, tenant.UnitOfWorkPoolWait, observed[1].Event.Kind)
	assert.Equal(t, http.StatusOK, observed[0].Status)
	assert.Equal(t, http.StatusOK, observed[1].Status)
}
