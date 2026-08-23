package tenant

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/uptrace/bun"
)

// TenantTxMiddleware returns a Chi middleware that wraps every request in a
// tenant-scoped transaction (SET LOCAL ROLE phoenix_tenant + set_config).
// This ensures Row-Level Security (RLS) policies are enforced for both read
// and write handlers.
//
// Behaviour:
//   - tenantID > 0 (from jwt.TenantMiddleware): request runs inside WithTenantTx
//   - tenantID == 0 (platform scope or unauthenticated): passes through without tx
//
// Nesting: write handlers that call WithTenantTx internally will reuse the
// outer transaction opened here (see tx.go:25-31). If the handler responds
// with HTTP 5xx the transaction is rolled back; otherwise it commits.
func TenantTxMiddleware(db *bun.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := FromContext(r.Context())
			if tenantID == 0 {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}

			err := WithTenantTx(r.Context(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
				ctx = WithRollbackMarker(ctx)
				next.ServeHTTP(sw, r.WithContext(ctx))

				// Roll back if the handler signalled a server error so that
				// any partial writes inside the transaction are not committed.
				if sw.status >= http.StatusInternalServerError {
					return fmt.Errorf("tenant: handler returned %d; rolling back tx", sw.status)
				}
				if RollbackRequested(ctx) {
					return fmt.Errorf("tenant: handler requested rollback after status %d", sw.statusCode())
				}
				return nil
			})

			// If WithTenantTx itself failed (e.g. DB connection error) and
			// the handler never wrote a response, send a generic 500.
			if err != nil && !sw.wroteHeader {
				slog.ErrorContext(r.Context(), "tenant transaction failed",
					slog.Int64("tenant_id", tenantID),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("error", err.Error()),
				)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			logTenantRequest(r, tenantID, sw, start, err)
		})
	}
}

func logTenantRequest(r *http.Request, tenantID int64, sw *statusWriter, start time.Time, err error) {
	route := observability.RoutePattern(r)

	txOutcome := "commit"
	if err != nil {
		txOutcome = "rollback_or_error"
	}
	duration := time.Since(start)
	status := sw.statusCode()

	observability.ObserveTenantRequest(tenantID, ScopeFromContext(r.Context()), r.Method, route, status, duration, txOutcome)

	slog.InfoContext(r.Context(), "tenant request completed",
		slog.Int64("tenant_id", tenantID),
		slog.String("scope", ScopeFromContext(r.Context())),
		slog.String("method", r.Method),
		slog.String("route", route),
		slog.Int("status", status),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Int64("request_bytes", r.ContentLength),
		slog.Int64("response_bytes", sw.bytesWritten),
		slog.String("tx_outcome", txOutcome),
	)
}

// statusWriter wraps http.ResponseWriter to capture the status code written
// by the downstream handler. This lets the middleware detect 5xx responses
// and roll back the enclosing transaction.
type statusWriter struct {
	http.ResponseWriter
	status       int
	wroteHeader  bool
	bytesWritten int64
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.wroteHeader {
		sw.status = code
		sw.wroteHeader = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.wroteHeader {
		// Implicit 200 on first Write call (standard http behaviour)
		sw.status = http.StatusOK
		sw.wroteHeader = true
	}
	n, err := sw.ResponseWriter.Write(b)
	sw.bytesWritten += int64(n)
	return n, err
}

// Unwrap returns the underlying ResponseWriter so that http.ResponseController
// and type-assertion based middleware (Flusher, Hijacker, etc.) keep working.
func (sw *statusWriter) Unwrap() http.ResponseWriter {
	return sw.ResponseWriter
}

func (sw *statusWriter) Flush() {
	if flusher, ok := sw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (sw *statusWriter) statusCode() int {
	if sw.wroteHeader {
		return sw.status
	}
	return http.StatusOK
}
