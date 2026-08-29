package common

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRuntime(t *testing.T, within func(context.Context, int64, func(context.Context, any) error) error) tenant.Runtime {
	t.Helper()
	runtime, err := tenant.NewRuntime(
		within,
		func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, struct{}{}) },
		func(context.Context, tenant.SavepointAction) error { return nil },
	)
	require.NoError(t, err)
	return runtime
}

func TestTenantTxMiddlewareRejectsMissingTenant(t *testing.T) {
	t.Parallel()
	called := false
	var observed TenantRequestEvent
	handler := TenantTxMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	handler = TenantRequestObserverMiddleware(func(event TenantRequestEvent) { observed = event })(handler)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/rooms", nil))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.False(t, called)
	assert.Equal(t, "missing_tenant", observed.Outcome)
}

func TestTenantTxMiddlewareUsesValidatedTenantAndRollbackMarker(t *testing.T) {
	t.Parallel()
	var gotTenant int64
	runtime := testRuntime(t, func(ctx context.Context, rawID int64, fn func(context.Context, any) error) error {
		gotTenant = rawID
		return fn(ctx, struct{}{})
	})
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)

	handler := TenantTxMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, tenant.HasAfterCommitHooks(r.Context()))
		tenant.MarkRollback(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/rooms", nil)
	ctx := tenant.WithRuntime(tenant.WithTenant(request.Context(), id), runtime)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request.WithContext(ctx))

	assert.Equal(t, int64(42), gotTenant)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestTenantTxMiddlewareUsesAdminTransactionForPlatformScope(t *testing.T) {
	t.Parallel()
	adminCalled := false
	runtime, err := tenant.NewRuntime(
		func(context.Context, int64, func(context.Context, any) error) error {
			t.Fatal("platform request must not open a tenant transaction")
			return nil
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			adminCalled = true
			return fn(ctx, struct{}{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
	)
	require.NoError(t, err)

	handlerCalled := false
	handler := TenantTxMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		assert.True(t, tenant.IsAdminTx(r.Context()))
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/auth/accounts", nil)
	ctx := tenant.WithRuntime(tenant.WithScope(request.Context(), tenant.ScopePlatform), runtime)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request.WithContext(ctx))

	assert.True(t, adminCalled)
	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestTenantTxMiddlewareObservesTransactionFailure(t *testing.T) {
	t.Parallel()
	runtimeErr := assert.AnError
	runtime := testRuntime(t, func(context.Context, int64, func(context.Context, any) error) error {
		return runtimeErr
	})
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	var observed TenantRequestEvent
	handler := TenantTxMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler must not run when transaction setup fails")
	}))
	handler = TenantRequestObserverMiddleware(func(event TenantRequestEvent) { observed = event })(handler)
	request := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	ctx := tenant.WithRuntime(tenant.WithTenant(request.Context(), id), runtime)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request.WithContext(ctx))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Equal(t, "rollback_or_error", observed.Outcome)
}

func TestTenantOperationMiddlewareRejectsMissingTenant(t *testing.T) {
	t.Parallel()
	called := false
	var observed TenantRequestEvent
	handler := TenantOperationMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	handler = TenantRequestObserverMiddleware(func(event TenantRequestEvent) { observed = event })(handler)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/notifications/push/subscriptions", nil))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.False(t, called)
	assert.Equal(t, "missing_tenant", observed.Outcome)
}

func TestTenantOperationMiddlewareObservesServiceOwnedTransactionFailure(t *testing.T) {
	t.Parallel()
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	runtime, err := tenant.NewRuntime(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(context.Context, func(context.Context, any) error) error { return assert.AnError },
		func(context.Context, tenant.SavepointAction) error { return nil },
	)
	require.NoError(t, err)
	var observed tenant.RuntimeEvent
	handler := TenantOperationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := tenant.WithAdminTx(r.Context(), struct{}{}, func(context.Context, struct{}) error {
			t.Fatal("callback must not run when transaction setup fails")
			return nil
		})
		require.Error(t, err)
		http.Error(w, "failed", http.StatusInternalServerError)
	}))
	handler = TenantRuntimeObserverMiddleware(func(event tenant.RuntimeEvent) { observed = event })(handler)
	request := httptest.NewRequest(http.MethodPost, "/api/notifications/push/subscriptions", nil)
	request = request.WithContext(tenant.WithRuntime(tenant.WithTenant(request.Context(), id), runtime))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Equal(t, tenant.RuntimeTransaction, observed.Outcome)
	require.ErrorIs(t, observed.Err, assert.AnError)
}

func TestTenantOperationMiddlewareDoesNotInventTransactionOutcome(t *testing.T) {
	t.Parallel()
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	observed := false
	handler := TenantOperationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "failed before transaction", http.StatusInternalServerError)
	}))
	handler = TenantRuntimeObserverMiddleware(func(tenant.RuntimeEvent) { observed = true })(handler)
	request := httptest.NewRequest(http.MethodGet, "/api/notifications/push/public-key", nil)
	request = request.WithContext(tenant.WithTenant(request.Context(), id))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.False(t, observed)
}
