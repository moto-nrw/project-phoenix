package common

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
)

type tenantRequestObserverKey struct{}
type tenantRuntimeObserverKey struct{}

type TenantRuntime = tenant.UnitOfWork
type TenantRuntimeEvent = tenant.RuntimeEvent

const (
	TenantRuntimeTransaction                              = tenant.RuntimeTransaction
	TenantRuntimeMissingTenant                            = tenant.RuntimeMissingTenant
	TenantRuntimeResponseWrite tenant.UnitOfWorkEventKind = "response_write"
)

type TenantRequestEvent struct {
	Request  *http.Request
	TenantID int64
	Scope    string
	Status   int
	Duration time.Duration
	Outcome  string
}

func TenantRuntimeMiddleware(runtime tenant.UnitOfWork) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), runtime)))
		})
	}
}

func TenantRuntimeObserverMiddleware(observer func(context.Context, TenantRuntimeEvent)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCtx := context.WithValue(r.Context(), tenantRuntimeObserverKey{}, observer)
			withObserver := tenant.WithRuntimeObserver(requestCtx, func(event TenantRuntimeEvent) {
				observer(requestCtx, event)
			})
			next.ServeHTTP(w, r.WithContext(withObserver))
		})
	}
}

func observeTenantRuntime(ctx context.Context, event TenantRuntimeEvent) {
	observer, _ := ctx.Value(tenantRuntimeObserverKey{}).(func(context.Context, TenantRuntimeEvent))
	if observer != nil {
		observer(ctx, event)
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
			tenant.ObserveMissingTenant(r.Context(), err)
			observeTenantRequest(r, 0, http.StatusInternalServerError, 0, "missing_tenant")
			rejectTenantRequest(w, r, err)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// TenantTxMiddleware wraps a tenant-scoped route in the runtime bound by the
// Serve root. A request with a validated tenant runs in that tenant's
// transaction, whatever scope the token carries. Only a platform-scope token
// without a tenant runs in the cross-tenant administrative transaction; the
// operator portal uses it on shared routes such as /auth/permissions and
// /auth/accounts. Everything else fails closed before next runs.
func TenantTxMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantValue, err := tenant.TenantFromContext(r.Context())
		if err == nil {
			runRequestTransaction(w, r, next, tenantValue.Int64(), tenant.WithinCurrentTenant)
			return
		}
		if tenant.ScopeFromContext(r.Context()) == tenant.ScopePlatform {
			runRequestTransaction(w, r, next, 0, platformAdminTransaction)
			return
		}
		tenant.ObserveMissingTenant(r.Context(), err)
		observeTenantRequest(r, 0, http.StatusInternalServerError, 0, "missing_tenant")
		rejectTenantRequest(w, r, err)
	})
}

// platformAdminTransaction is the one sanctioned BYPASSRLS path on tenant
// routes. It exists as a named function so the branch is visible in traces
// and cannot be confused with a missing-tenant fallback.
func platformAdminTransaction(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithinAdmin(ctx, fn)
}

func runRequestTransaction(
	w http.ResponseWriter,
	r *http.Request,
	next http.Handler,
	tenantID int64,
	within func(context.Context, func(context.Context) error) error,
) {
	start := time.Now()
	sw := newTenantStatusWriter(w)
	defer sw.cleanupBodyFile()

	err := within(r.Context(), func(ctx context.Context) error {
		ctx = tenant.WithRollbackMarker(ctx)
		next.ServeHTTP(sw, r.WithContext(ctx))
		if sw.status >= http.StatusInternalServerError {
			return &requestRollbackError{reason: fmt.Sprintf("handler returned %d", sw.status)}
		}
		if tenant.RollbackRequested(ctx) {
			return &requestRollbackError{reason: fmt.Sprintf("handler requested rollback after status %d", sw.statusCode())}
		}
		return nil
	})

	_, requestRollback := err.(*requestRollbackError)
	if err != nil && !requestRollback {
		sw.reset()
		http.Error(sw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	if writeErr := sw.commitResponse(); writeErr != nil {
		observeTenantRuntime(r.Context(), TenantRuntimeEvent{Kind: TenantRuntimeResponseWrite, Err: writeErr})
	}

	logTenantRequest(r, tenantID, sw, start, err)
}

type requestRollbackError struct{ reason string }

func (e *requestRollbackError) Error() string {
	return "tenant: " + e.reason + "; rolling back transaction"
}

func rejectTenantRequest(w http.ResponseWriter, r *http.Request, err error) {
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
	target         http.ResponseWriter
	header         http.Header
	initialHeader  http.Header
	body           bytes.Buffer
	bodyFile       *os.File
	status         int
	wroteHeader    bool
	bytesWritten   int64
	flushRequested bool
}

const tenantResponseMemoryLimit = 1 << 20

func newTenantStatusWriter(target http.ResponseWriter) *tenantStatusWriter {
	initial := target.Header().Clone()
	return &tenantStatusWriter{target: target, header: initial.Clone(), initialHeader: initial}
}

func (w *tenantStatusWriter) Header() http.Header { return w.header }

func (w *tenantStatusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
}

func (w *tenantStatusWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.status = http.StatusOK
		w.wroteHeader = true
	}
	if w.bodyFile == nil && w.body.Len()+len(body) > tenantResponseMemoryLimit {
		file, err := os.CreateTemp("", "phoenix-tenant-response-*")
		if err != nil {
			return 0, err
		}
		if _, err := file.Write(w.body.Bytes()); err != nil {
			_ = file.Close()
			_ = os.Remove(file.Name())
			return 0, err
		}
		w.body.Reset()
		w.bodyFile = file
	}

	var n int
	var err error
	if w.bodyFile != nil {
		n, err = w.bodyFile.Write(body)
	} else {
		n, err = w.body.Write(body)
	}
	w.bytesWritten += int64(n)
	return n, err
}

func (w *tenantStatusWriter) Flush() {
	w.flushRequested = true
}

func (w *tenantStatusWriter) statusCode() int {
	if w.wroteHeader {
		return w.status
	}
	return http.StatusOK
}

func (w *tenantStatusWriter) reset() {
	w.header = w.initialHeader.Clone()
	w.body.Reset()
	w.cleanupBodyFile()
	w.status = 0
	w.wroteHeader = false
	w.bytesWritten = 0
	w.flushRequested = false
}

func (w *tenantStatusWriter) commitResponse() error {
	defer w.cleanupBodyFile()
	targetHeader := w.target.Header()
	clear(targetHeader)
	for key, values := range w.header {
		targetHeader[key] = append([]string(nil), values...)
	}
	w.target.WriteHeader(w.statusCode())
	if w.bodyFile != nil {
		if _, err := w.bodyFile.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if _, err := io.Copy(w.target, w.bodyFile); err != nil {
			return err
		}
	} else if _, err := w.target.Write(w.body.Bytes()); err != nil {
		return err
	}
	if w.flushRequested {
		if flusher, ok := w.target.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	return nil
}

func (w *tenantStatusWriter) cleanupBodyFile() {
	if w.bodyFile == nil {
		return
	}
	_ = w.bodyFile.Close()
	_ = os.Remove(w.bodyFile.Name())
	w.bodyFile = nil
}
