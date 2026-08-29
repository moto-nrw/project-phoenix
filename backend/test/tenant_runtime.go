package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// TenantRuntime builds the production tenant runtime against a test database.
func TenantRuntime(tb testing.TB, db *bun.DB) tenant.Runtime {
	tb.Helper()
	postgresRuntime, err := database.NewTenantRuntime(db)
	if err != nil {
		tb.Fatalf("create tenant runtime: %v", err)
	}
	runtime, err := tenant.NewRuntime(postgresRuntime.WithinTenant, postgresRuntime.WithinAdmin, postgresRuntime.ControlSavepoint)
	if err != nil {
		tb.Fatalf("bind tenant runtime: %v", err)
	}
	return runtime
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
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
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
