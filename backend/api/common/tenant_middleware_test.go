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

func testRuntime(t *testing.T, within func(context.Context, int64, func(context.Context, any) error) error) tenant.UnitOfWork {
	t.Helper()
	runtime, err := tenant.NewUnitOfWork(
		within,
		func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, struct{}{}) },
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
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
	ctx := tenant.WithUnitOfWork(tenant.WithTenant(request.Context(), id), runtime)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request.WithContext(ctx))

	assert.Equal(t, int64(42), gotTenant)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestTenantTxMiddlewareUsesAdminTransactionForPlatformScope(t *testing.T) {
	t.Parallel()
	adminCalled := false
	runtime, err := tenant.NewUnitOfWork(
		func(context.Context, int64, func(context.Context, any) error) error {
			t.Fatal("platform request must not open a tenant transaction")
			return nil
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			adminCalled = true
			return fn(ctx, struct{}{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)

	handlerCalled := false
	handler := TenantTxMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		assert.True(t, tenant.IsAdminTx(r.Context()))
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/auth/accounts", nil)
	ctx := tenant.WithUnitOfWork(tenant.WithScope(request.Context(), tenant.ScopePlatform), runtime)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request.WithContext(ctx))

	assert.True(t, adminCalled)
	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestTenantTxMiddlewarePrefersTenantOverPlatformScope(t *testing.T) {
	t.Parallel()
	var gotTenant int64
	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, rawID int64, fn func(context.Context, any) error) error {
			gotTenant = rawID
			return fn(ctx, struct{}{})
		},
		func(context.Context, func(context.Context, any) error) error {
			t.Fatal("platform token carrying a tenant must not bypass RLS")
			return nil
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)

	handler := TenantTxMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.False(t, tenant.IsAdminTx(r.Context()))
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/auth/accounts", nil)
	ctx := tenant.WithScope(tenant.WithTenant(request.Context(), id), tenant.ScopePlatform)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request.WithContext(tenant.WithUnitOfWork(ctx, runtime)))

	assert.Equal(t, int64(42), gotTenant)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestTenantStatusWriterTracksStatusAndBytes(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	sw := newTenantStatusWriter(recorder)

	assert.Equal(t, http.StatusOK, sw.statusCode(), "unwritten response reports 200")

	n, err := sw.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	sw.WriteHeader(http.StatusTeapot)
	_, err = sw.Write([]byte(" world"))
	require.NoError(t, err)
	sw.Flush()

	assert.Equal(t, http.StatusOK, sw.statusCode(), "implicit 200 from Write is not overwritten")
	assert.Equal(t, int64(11), sw.bytesWritten)
	assert.Empty(t, recorder.Body.String(), "response stays buffered before commit")
	require.NoError(t, sw.commitResponse())
	assert.True(t, recorder.Flushed)
	assert.Equal(t, "hello world", recorder.Body.String())
}

func TestTenantTxMiddlewareDiscardsSuccessWhenCommitFails(t *testing.T) {
	t.Parallel()
	commitErr := assert.AnError
	runtime := testRuntime(t, func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
		if err := fn(ctx, struct{}{}); err != nil {
			return err
		}
		return commitErr
	})
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)

	handler := TenantTxMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/rooms", nil)
	ctx := tenant.WithUnitOfWork(tenant.WithTenant(request.Context(), id), runtime)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request.WithContext(ctx))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.NotContains(t, recorder.Body.String(), `"ok":true`)
}

func TestTenantTxMiddlewareReportsRollbackFailure(t *testing.T) {
	t.Parallel()
	runtime := testRuntime(t, func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
		return errors.Join(fn(ctx, struct{}{}), assert.AnError)
	})
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)

	handler := TenantTxMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant.MarkRollback(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/rooms", nil)
	request = request.WithContext(tenant.WithUnitOfWork(tenant.WithTenant(request.Context(), id), runtime))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestTenantStatusWriterSpoolsLargeResponse(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	sw := newTenantStatusWriter(recorder)
	body := make([]byte, tenantResponseMemoryLimit+1)

	_, err := sw.Write(body)
	require.NoError(t, err)
	assert.NotNil(t, sw.bodyFile)
	assert.LessOrEqual(t, sw.body.Len(), tenantResponseMemoryLimit)
	require.NoError(t, sw.commitResponse())
	assert.Len(t, recorder.Body.Bytes(), len(body))
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
	ctx := tenant.WithUnitOfWork(tenant.WithTenant(request.Context(), id), runtime)
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
	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(context.Context, func(context.Context, any) error) error { return assert.AnError },
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
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
	request = request.WithContext(tenant.WithUnitOfWork(tenant.WithTenant(request.Context(), id), runtime))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Equal(t, tenant.RuntimeTransaction, observed.Kind)
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
