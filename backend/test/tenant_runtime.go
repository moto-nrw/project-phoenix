package test

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var packageTenantRuntime atomic.Pointer[tenant.Runtime]

func newTenantRuntime(db *bun.DB) (tenant.Runtime, error) {
	postgresRuntime, err := database.NewTenantRuntime(db)
	if err != nil {
		return tenant.Runtime{}, err
	}
	return tenant.NewRuntime(postgresRuntime.WithinTenant, postgresRuntime.WithinAdmin, postgresRuntime.ControlSavepoint)
}

func bindPackageTenantRuntime(db *bun.DB) error {
	runtime, err := newTenantRuntime(db)
	if err != nil {
		return err
	}
	packageTenantRuntime.Store(&runtime)
	return nil
}

// TenantRuntime builds the production tenant runtime against a test database.
func TenantRuntime(tb testing.TB, db *bun.DB) tenant.Runtime {
	tb.Helper()
	runtime, err := newTenantRuntime(db)
	if err != nil {
		tb.Fatalf("create tenant runtime: %v", err)
	}
	return runtime
}

// WithPackageTenantRuntime mirrors the runtime middleware installed by the
// production API root. It is a no-op until SetupTestDB has initialized this
// test binary's shared database.
func WithPackageTenantRuntime(ctx context.Context) context.Context {
	runtime := packageTenantRuntime.Load()
	if runtime == nil {
		return ctx
	}
	return tenant.WithRuntime(ctx, *runtime)
}

// PackageTenantRuntime returns the runtime bound to this test binary's shared
// database. It is available after SetupTestDB has initialized the package.
func PackageTenantRuntime() (tenant.Runtime, bool) {
	runtime := packageTenantRuntime.Load()
	if runtime == nil {
		return tenant.Runtime{}, false
	}
	return *runtime, true
}

// TenantRuntimeMiddleware mirrors the runtime middleware installed at the
// production API root for tests that exercise a domain router in isolation.
func TenantRuntimeMiddleware(tb testing.TB, db *bun.DB) func(http.Handler) http.Handler {
	tb.Helper()
	runtime := TenantRuntime(tb, db)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithRuntime(r.Context(), runtime)))
		})
	}
}

// SetTenantRuntime wires a test runtime without forcing behavior tests to
// import the tenant runtime package directly.
func SetTenantRuntime(tb testing.TB, target any, db *bun.DB) {
	tb.Helper()
	setter, ok := target.(interface{ SetTenantRuntime(tenant.Runtime) })
	if !ok {
		tb.Fatalf("%T does not accept a tenant runtime", target)
	}
	setter.SetTenantRuntime(TenantRuntime(tb, db))
}

func WithTenantTx(tb testing.TB, ctx context.Context, db *bun.DB, tenantID int64, fn func(context.Context, bun.Tx) error) error {
	tb.Helper()
	return tenant.WithTenantTx(WithTenantRuntime(tb, ctx, db), db, tenantID, fn)
}

func WithAdminTx(tb testing.TB, ctx context.Context, db *bun.DB, fn func(context.Context, bun.Tx) error) error {
	tb.Helper()
	return tenant.WithAdminTx(WithTenantRuntime(tb, ctx, db), db, fn)
}

// TenantTxMiddleware mirrors the production transaction boundary for API
// tests without making api/testutil own database composition.
func TenantTxMiddleware(db *bun.DB) func(http.Handler) http.Handler {
	postgresRuntime, err := database.NewTenantRuntime(db)
	if err != nil {
		panic(err)
	}
	runtime, err := tenant.NewRuntime(postgresRuntime.WithinTenant, postgresRuntime.WithinAdmin, postgresRuntime.ControlSavepoint)
	if err != nil {
		panic(err)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.WithRuntime(r.Context(), runtime)
			if _, tenantErr := tenant.TenantFromContext(ctx); tenantErr != nil {
				// NewTenantRouter is also used to mount public routes. Production
				// applies TenantTxMiddleware only after tenant authentication, so
				// preserve that route shape by passing unscoped requests through.
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			status := http.StatusOK
			err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
				txCtx = tenant.WithRollbackMarker(txCtx)
				writer := &statusWriter{ResponseWriter: w, status: &status}
				next.ServeHTTP(writer, r.WithContext(txCtx))
				if status >= http.StatusInternalServerError || tenant.RollbackRequested(txCtx) {
					return fmt.Errorf("test tenant request requires rollback")
				}
				return nil
			})
			if err != nil && status == http.StatusOK {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status *int
}

func (w *statusWriter) WriteHeader(status int) {
	*w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func WithTenantRuntime(tb testing.TB, ctx context.Context, db *bun.DB) context.Context {
	tb.Helper()
	return tenant.WithRuntime(ctx, TenantRuntime(tb, db))
}
