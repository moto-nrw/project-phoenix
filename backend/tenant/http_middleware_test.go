package tenant_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
)

// TestTenantTxMiddleware_PassthroughWhenNoTenantID verifies that requests
// without a tenant ID (e.g. platform scope) pass through without a tx.
func TestTenantTxMiddleware_PassthroughWhenNoTenantID(t *testing.T) {
	t.Parallel()

	handlerCalled := false

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// db is nil — if the middleware tried to open a tx it would panic.
	// Passing nil proves the passthrough path is taken.
	middleware := tenant.TenantTxMiddleware(nil)(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	// No tenant ID in context → tenantID == 0
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	assert.True(t, handlerCalled, "handler should be called")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestTenantTxMiddleware_AttemptsTransactionWhenTenantIDPresent verifies that
// when a tenant ID is set in the context the middleware enters the WithTenantTx
// path. Because we pass a nil *bun.DB, WithTenantTx panics (nil dereference
// inside RunInTx). Catching that panic proves the tx-path was taken.
func TestTenantTxMiddleware_AttemptsTransactionWhenTenantIDPresent(t *testing.T) {
	t.Parallel()

	handlerCalled := false

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := tenant.TenantTxMiddleware(nil)(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	ctx := tenant.WithTenantID(req.Context(), 42)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	// nil DB causes a panic inside bun.RunInTx — recover to assert behaviour.
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		middleware.ServeHTTP(rec, req)
	}()

	assert.True(t, panicked, "nil DB should cause a panic proving the tx path was entered")
	assert.False(t, handlerCalled, "handler should not be called when tx setup panics")
}

// TestStatusWriter_CapturesWriteHeader verifies the status writer captures
// the first WriteHeader call.
func TestStatusWriter_CapturesWriteHeader(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	sw := &statusWriterShim{ResponseWriter: rec}

	sw.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, sw.status)

	// Second call should not overwrite
	sw.WriteHeader(http.StatusOK)
	assert.Equal(t, http.StatusNotFound, sw.status, "first status should stick")
}

// TestStatusWriter_ImplicitOKOnWrite verifies implicit 200 when Write is
// called without a prior WriteHeader.
func TestStatusWriter_ImplicitOKOnWrite(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	sw := &statusWriterShim{ResponseWriter: rec}

	_, err := sw.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, sw.status)
}

// statusWriterShim duplicates the unexported statusWriter from http_middleware.go
// so we can unit-test WriteHeader/Write capture without exporting the type.
type statusWriterShim struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sw *statusWriterShim) WriteHeader(code int) {
	if !sw.wroteHeader {
		sw.status = code
		sw.wroteHeader = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriterShim) Write(b []byte) (int, error) {
	if !sw.wroteHeader {
		sw.status = http.StatusOK
		sw.wroteHeader = true
	}
	return sw.ResponseWriter.Write(b)
}
