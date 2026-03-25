package tenant

import (
	"context"
	"fmt"
	"net/http"

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

			sw := &statusWriter{ResponseWriter: w}

			err := WithTenantTx(r.Context(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
				next.ServeHTTP(sw, r.WithContext(ctx))

				// Roll back if the handler signalled a server error so that
				// any partial writes inside the transaction are not committed.
				if sw.status >= http.StatusInternalServerError {
					return fmt.Errorf("tenant: handler returned %d; rolling back tx", sw.status)
				}
				return nil
			})

			// If WithTenantTx itself failed (e.g. DB connection error) and
			// the handler never wrote a response, send a generic 500.
			if err != nil && !sw.wroteHeader {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		})
	}
}

// statusWriter wraps http.ResponseWriter to capture the status code written
// by the downstream handler. This lets the middleware detect 5xx responses
// and roll back the enclosing transaction.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
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
	return sw.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter so that http.ResponseController
// and type-assertion based middleware (Flusher, Hijacker, etc.) keep working.
func (sw *statusWriter) Unwrap() http.ResponseWriter {
	return sw.ResponseWriter
}
