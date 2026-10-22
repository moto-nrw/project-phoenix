package common

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
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
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-unerwarteter-fehler","title":"Internal Server Error","status":500,"detail":"Internal Server Error","instance":"","code":"general.server"}`, recorder.Body.String())
}

type failingTenantResponseWriter struct{ header http.Header }

func (w *failingTenantResponseWriter) Header() http.Header     { return w.header }
func (*failingTenantResponseWriter) WriteHeader(int)           {}
func (*failingTenantResponseWriter) Write([]byte) (int, error) { return 0, assert.AnError }

func TestTenantTxMiddlewareObservesResponseWriteFailure(t *testing.T) {
	t.Parallel()
	runtime := testRuntime(t, func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
		return fn(ctx, struct{}{})
	})
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)

	var observed TenantRuntimeEvent
	handler := TenantTxMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("buffered response"))
	}))
	handler = TenantRuntimeObserverMiddleware(func(observation TenantRuntimeObservation) {
		observed = observation.Event
	})(handler)
	request := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	ctx := tenant.WithUnitOfWork(tenant.WithTenant(request.Context(), id), runtime)

	handler.ServeHTTP(&failingTenantResponseWriter{header: make(http.Header)}, request.WithContext(ctx))

	assert.Equal(t, TenantRuntimeResponseWrite, observed.Kind)
	assert.ErrorIs(t, observed.Err, assert.AnError)
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
	handler = TenantRuntimeObserverMiddleware(func(observation TenantRuntimeObservation) { observed = observation.Event })(handler)
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
	handler = TenantRuntimeObserverMiddleware(func(TenantRuntimeObservation) { observed = true })(handler)
	request := httptest.NewRequest(http.MethodGet, "/api/notifications/push/public-key", nil)
	request = request.WithContext(tenant.WithTenant(request.Context(), id))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.False(t, observed)
}

func TestTenantTxMiddlewareCommitsBodylessResponseWithoutBody(t *testing.T) {
	t.Parallel()
	runtime := testRuntime(t, func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
		return fn(ctx, struct{}{})
	})
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)

	for _, tc := range []struct {
		name    string
		status  int
		handler http.HandlerFunc
	}{
		{name: "respond helper no content", status: http.StatusNoContent, handler: func(w http.ResponseWriter, r *http.Request) {
			Respond(w, r, http.StatusNoContent, nil, "")
		}},
		{name: "raw writer no content", status: http.StatusNoContent, handler: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
			_, _ = w.Write([]byte(`{"status":"success"}`))
		}},
		{name: "respond helper reset content", status: http.StatusResetContent, handler: func(w http.ResponseWriter, r *http.Request) {
			Respond(w, r, http.StatusResetContent, nil, "")
		}},
		{name: "raw writer reset content", status: http.StatusResetContent, handler: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusResetContent)
			_, _ = w.Write([]byte(`{"status":"success"}`))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var observed []TenantRuntimeEvent
			var mu sync.Mutex
			handler := TenantRuntimeObserverMiddleware(func(observation TenantRuntimeObservation) {
				if observation.Event.Kind != TenantRuntimeResponseWrite {
					return
				}
				mu.Lock()
				defer mu.Unlock()
				observed = append(observed, observation.Event)
			})(TenantTxMiddleware(tc.handler))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := tenant.WithUnitOfWork(tenant.WithTenant(r.Context(), id), runtime)
				handler.ServeHTTP(w, r.WithContext(ctx))
			}))
			t.Cleanup(server.Close)

			request, err := http.NewRequestWithContext(t.Context(), http.MethodPut, server.URL+"/api/notifications/email-subscriptions/news", nil)
			require.NoError(t, err)
			response, err := server.Client().Do(request)
			require.NoError(t, err)
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())

			assert.Equal(t, tc.status, response.StatusCode)
			assert.Empty(t, body)
			assert.Empty(t, response.Header.Get("Content-Type"))
			mu.Lock()
			defer mu.Unlock()
			assert.Empty(t, observed)
		})
	}
}

func TestTenantTxMiddlewarePreservesInterimResponseBeforeFinalResponse(t *testing.T) {
	t.Parallel()
	runtime := testRuntime(t, func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
		return fn(ctx, struct{}{})
	})
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	for _, tc := range []struct {
		name          string
		handler       http.HandlerFunc
		status        int
		contentType   string
		bodyAssertion func(*testing.T, []byte)
	}{
		{
			name: "bodyless final response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Link", "</app.css>; rel=preload; as=style")
				w.WriteHeader(http.StatusEarlyHints)
				Respond(w, r, http.StatusResetContent, nil, "")
			},
			status: http.StatusResetContent,
			bodyAssertion: func(t *testing.T, body []byte) {
				assert.Empty(t, body)
			},
		},
		{
			name: "error final response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusEarlyHints)
				http.Error(w, "failed", http.StatusInternalServerError)
			},
			status:      http.StatusInternalServerError,
			contentType: problemContentType,
			bodyAssertion: func(t *testing.T, body []byte) {
				assert.Contains(t, string(body), `"status":500`)
				assert.Contains(t, string(body), `"code":"general.server"`)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handler := ProblemResponseMiddleware(TenantTxMiddleware(tc.handler))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := tenant.WithUnitOfWork(tenant.WithTenant(r.Context(), id), runtime)
				handler.ServeHTTP(w, r.WithContext(ctx))
			}))
			t.Cleanup(server.Close)

			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
			require.NoError(t, err)
			response, err := server.Client().Do(request)
			require.NoError(t, err)
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())

			assert.Equal(t, tc.status, response.StatusCode)
			assert.Equal(t, tc.contentType, response.Header.Get("Content-Type"))
			tc.bodyAssertion(t, body)
		})
	}
}

func TestTenantTxMiddlewareIgnoresInterimResponseAfterFinalResponse(t *testing.T) {
	t.Parallel()
	runtime := testRuntime(t, func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
		return fn(ctx, struct{}{})
	})
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	for _, tc := range []struct {
		name       string
		body       string
		writeFinal func(http.ResponseWriter)
	}{
		{name: "explicit status", writeFinal: func(w http.ResponseWriter) { w.WriteHeader(http.StatusOK) }},
		{name: "implicit status", body: "complete", writeFinal: func(w http.ResponseWriter) { _, _ = w.Write([]byte("complete")) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			recorder := newStatusRecordingResponseWriter()
			handler := ProblemResponseMiddleware(TenantTxMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				tc.writeFinal(w)
				w.WriteHeader(http.StatusEarlyHints)
			})))
			request := httptest.NewRequest(http.MethodGet, "/api/example", nil)
			request = request.WithContext(tenant.WithUnitOfWork(tenant.WithTenant(request.Context(), id), runtime))
			handler.ServeHTTP(recorder, request)

			assert.Equal(t, []int{http.StatusOK}, recorder.statuses)
			assert.Equal(t, tc.body, recorder.body.String())
		})
	}
}

func TestProblemResponseMiddlewareIgnoresInterimResponseAfterError(t *testing.T) {
	t.Parallel()
	recorder := newStatusRecordingResponseWriter()
	handler := ProblemResponseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.WriteHeader(http.StatusEarlyHints)
	}))
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/example", nil))

	assert.Equal(t, []int{http.StatusInternalServerError}, recorder.statuses)
	assert.Contains(t, recorder.body.String(), `"code":"general.server"`)
}

type statusRecordingResponseWriter struct {
	header   http.Header
	statuses []int
	body     bytes.Buffer
}

func newStatusRecordingResponseWriter() *statusRecordingResponseWriter {
	return &statusRecordingResponseWriter{header: make(http.Header)}
}

func (w *statusRecordingResponseWriter) Header() http.Header { return w.header }

func (w *statusRecordingResponseWriter) WriteHeader(status int) {
	w.statuses = append(w.statuses, status)
}

func (w *statusRecordingResponseWriter) Write(body []byte) (int, error) {
	return w.body.Write(body)
}
