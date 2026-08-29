package common

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
)

type tenantRequestObserverKey struct{}

type TenantRuntime = tenant.Runtime
type TenantRuntimeEvent = tenant.RuntimeEvent

const (
	TenantRuntimeTransaction   = tenant.RuntimeTransaction
	TenantRuntimeMissingTenant = tenant.RuntimeMissingTenant
)

type TenantRequestEvent struct {
	Request  *http.Request
	TenantID int64
	Scope    string
	Status   int
	Duration time.Duration
	Outcome  string
}

func TenantRuntimeMiddleware(runtime tenant.Runtime) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithRuntime(r.Context(), runtime)))
		})
	}
}

func TenantRuntimeObserverMiddleware(observer func(TenantRuntimeEvent)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithRuntimeObserver(r.Context(), observer)))
		})
	}
}

func TenantRequestObserverMiddleware(observer func(TenantRequestEvent)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), tenantRequestObserverKey{}, observer)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TenantOperationMiddleware guards handlers whose services own their
// transactions. It validates tenant presence but deliberately opens no tx.
func TenantOperationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := tenant.TenantFromContext(r.Context())
		if err != nil {
			observeTenantRequest(r, 0, http.StatusInternalServerError, 0, "missing_tenant")
			rejectTenantRequest(w, r, err)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// TenantTxMiddleware wraps a tenant-scoped route in the runtime bound by the
// Serve root. Missing tenant or runtime context fails closed before next runs.
func TenantTxMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantValue, err := tenant.TenantFromContext(r.Context())
		if err != nil {
			observeTenantRequest(r, 0, http.StatusInternalServerError, 0, "missing_tenant")
			rejectTenantRequest(w, r, err)
			return
		}
		tenantID := tenantValue.Int64()
		start := time.Now()
		sw := &tenantStatusWriter{ResponseWriter: w}

		err = tenant.WithinCurrentTenant(r.Context(), func(ctx context.Context) error {
			ctx = tenant.WithRollbackMarker(ctx)
			next.ServeHTTP(sw, r.WithContext(ctx))
			if sw.status >= http.StatusInternalServerError {
				return fmt.Errorf("tenant: handler returned %d; rolling back transaction", sw.status)
			}
			if tenant.RollbackRequested(ctx) {
				return fmt.Errorf("tenant: handler requested rollback after status %d", sw.statusCode())
			}
			return nil
		})

		if err != nil && !sw.wroteHeader {
			slog.ErrorContext(r.Context(), "tenant transaction failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("error", err.Error()),
			)
			http.Error(sw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		logTenantRequest(r, tenantID, sw, start, err)
	})
}

func rejectTenantRequest(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "tenant request rejected",
		slog.String("entry_point", "http"),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("error", err.Error()),
	)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func logTenantRequest(r *http.Request, tenantID int64, sw *tenantStatusWriter, start time.Time, err error) {
	outcome := "commit"
	if err != nil {
		outcome = "rollback_or_error"
	}
	duration := time.Since(start)
	status := sw.statusCode()
	observeTenantRequest(r, tenantID, status, duration, outcome)

	slog.InfoContext(r.Context(), "tenant request completed",
		slog.Int64("tenant_id", tenantID),
		slog.String("scope", tenant.ScopeFromContext(r.Context())),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Int("status", status),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Int64("request_bytes", r.ContentLength),
		slog.Int64("response_bytes", sw.bytesWritten),
		slog.String("tx_outcome", outcome),
	)
}

func observeTenantRequest(r *http.Request, tenantID int64, status int, duration time.Duration, outcome string) {
	observer, _ := r.Context().Value(tenantRequestObserverKey{}).(func(TenantRequestEvent))
	if observer != nil {
		observer(TenantRequestEvent{Request: r, TenantID: tenantID, Scope: tenant.ScopeFromContext(r.Context()), Status: status, Duration: duration, Outcome: outcome})
	}
}

type tenantStatusWriter struct {
	http.ResponseWriter
	status       int
	wroteHeader  bool
	bytesWritten int64
}

func (w *tenantStatusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *tenantStatusWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.status = http.StatusOK
		w.wroteHeader = true
	}
	n, err := w.ResponseWriter.Write(body)
	w.bytesWritten += int64(n)
	return n, err
}

func (w *tenantStatusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *tenantStatusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *tenantStatusWriter) statusCode() int {
	if w.wroteHeader {
		return w.status
	}
	return http.StatusOK
}
