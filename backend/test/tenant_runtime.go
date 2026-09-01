package test

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type UnitOfWorkEvidence struct {
	Kind     string
	Duration time.Duration
}

type DB = bun.DB

func ContextForTenant(ctx context.Context, tenantID int64) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

// CaptureUnitOfWorkEvidence exposes transaction evidence to repository tests
// without making those packages import the tenant runtime directly.
func CaptureUnitOfWorkEvidence(ctx context.Context) (context.Context, func() []UnitOfWorkEvidence) {
	events := make([]UnitOfWorkEvidence, 0)
	ctx = tenant.WithUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) {
		events = append(events, UnitOfWorkEvidence{Kind: string(event.Kind), Duration: event.Duration})
	})
	return ctx, func() []UnitOfWorkEvidence { return append([]UnitOfWorkEvidence(nil), events...) }
}

func AttachLockWaitEvidence(db *bun.DB) {
	db.AddQueryHook(database.NewLockWaitQueryHook(tenant.ObserveLockWait))
}

var packageTenantRuntime atomic.Pointer[tenant.UnitOfWork]

func newTenantRuntime(db *bun.DB) (tenant.UnitOfWork, error) {
	postgresRuntime, err := database.NewPostgresUnitOfWork(db, tenant.ObservePoolWait)
	if err != nil {
		return tenant.UnitOfWork{}, err
	}
	runtime, err := tenant.NewUnitOfWork(
		postgresRuntime.WithinTenant,
		postgresRuntime.WithinAdmin,
		tenant.SavepointFunc(postgresRuntime),
		database.IsRetryableTransactionError,
		postgresRuntime.AcquireLock,
	)
	if err != nil {
		return tenant.UnitOfWork{}, err
	}
	runtime = runtime.WithTransactionDetacher(postgresRuntime.ContextWithoutTransaction)
	runtime = runtime.WithTransactionDetacher(func(ctx context.Context) context.Context { return audit.WithTransaction(ctx, nil) })
	runtime = runtime.WithContextAdapters(postgresRuntime.ContextWithTenant, postgresRuntime.ContextWithTransaction)
	return runtime.WithContextAdapters(audit.WithTenantID, audit.WithTransaction), nil
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
func TenantRuntime(tb testing.TB, db *bun.DB) tenant.UnitOfWork {
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
	return tenant.WithUnitOfWork(ctx, *runtime)
}

// PackageTenantRuntime returns the runtime bound to this test binary's shared
// database. It is available after SetupTestDB has initialized the package.
func PackageTenantRuntime() (tenant.UnitOfWork, bool) {
	runtime := packageTenantRuntime.Load()
	if runtime == nil {
		return tenant.UnitOfWork{}, false
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
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), runtime)))
		})
	}
}

// SetTenantRuntime wires a test runtime without forcing behavior tests to
// import the tenant runtime package directly.
func SetTenantRuntime(tb testing.TB, target any, db *bun.DB) {
	tb.Helper()
	setter, ok := target.(interface{ SetTenantRuntime(tenant.UnitOfWork) })
	if ok {
		setter.SetTenantRuntime(TenantRuntime(tb, db))
		return
	}
	method := reflect.ValueOf(target).MethodByName("SetRuntime")
	if !method.IsValid() {
		tb.Fatalf("%T does not accept a tenant runtime", target)
	}
	method.Call([]reflect.Value{reflect.ValueOf(SettingsRuntime(tb, db))})
}

// SettingsRuntimeAdapter implements the consumer-owned settings runtime port.
type SettingsRuntimeAdapter struct {
	db      *bun.DB
	runtime tenant.UnitOfWork
}

func SettingsRuntime(tb testing.TB, db *bun.DB) SettingsRuntimeAdapter {
	tb.Helper()
	return SettingsRuntimeAdapter{db: db, runtime: TenantRuntime(tb, db)}
}

func (r SettingsRuntimeAdapter) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

func (r SettingsRuntimeAdapter) HasTransaction(ctx context.Context) bool {
	_, ok := tenant.TransactionFromContext(ctx)
	return ok
}

func (r SettingsRuntimeAdapter) DB(ctx context.Context) bun.IDB {
	if raw, ok := tenant.TransactionFromContext(ctx); ok {
		if tx, valid := raw.(bun.Tx); valid {
			return tx
		}
	}
	return r.db
}

func (r SettingsRuntimeAdapter) LockStaffBalance(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return fmt.Errorf("staff id is required")
	}
	tenantID := r.TenantID(ctx)
	if tenantID <= 0 {
		return fmt.Errorf("tenant id is required")
	}
	return r.AcquireLock(ctx, fmt.Sprintf("staff-balance:%d:%d", tenantID, staffID), false)
}

func (SettingsRuntimeAdapter) TodayTime() time.Time { return timezone.TodayDate().UTCMidnight() }

func (r SettingsRuntimeAdapter) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	ctx = tenant.WithUnitOfWork(ctx, r.runtime)
	return tenant.WithTenantTx(ctx, r.db, tenantID, func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
}

func (r SettingsRuntimeAdapter) WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	ctx = tenant.WithUnitOfWork(ctx, r.runtime)
	return tenant.WithAdminTx(ctx, r.db, func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
}

func (r SettingsRuntimeAdapter) AcquireLock(ctx context.Context, key string, shared bool) error {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		if shared {
			_, err := r.db.ExecContext(ctx, "SELECT pg_advisory_xact_lock_shared(hashtextextended(?, 0))", key)
			return err
		}
		_, err := r.db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key)
		return err
	}
	return tenant.AcquireLock(ctx, key, shared)
}

func WithTenantTx(tb testing.TB, ctx context.Context, db *bun.DB, tenantID int64, fn func(context.Context, bun.Tx) error) error {
	tb.Helper()
	return tenant.WithTenantTx(WithTenantRuntime(tb, ctx, db), db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
		txCtx = audit.WithTenantID(txCtx, tenantID)
		txCtx = audit.WithTransaction(txCtx, tx)
		return fn(txCtx, tx)
	})
}

func WithinTenantContext(tb testing.TB, ctx context.Context, db *bun.DB, tenantID int64, fn func(context.Context) error) error {
	tb.Helper()
	return WithTenantTx(tb, ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

func WithAdminTx(tb testing.TB, ctx context.Context, db *bun.DB, fn func(context.Context, bun.Tx) error) error {
	tb.Helper()
	return tenant.WithAdminTx(WithTenantRuntime(tb, ctx, db), db, fn)
}

func WithinAdminContext(tb testing.TB, ctx context.Context, db *bun.DB, fn func(context.Context) error) error {
	tb.Helper()
	return WithAdminTx(tb, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

// TenantTxMiddleware is the test stand-in for the production API root plus
// api/common.TenantTxMiddleware, for tests that inject identity straight into
// the request context. It decides exactly like production: a tenant runs in
// its tenant transaction, a platform-scope request without a tenant runs
// administratively, and an authenticated request without a usable tenant is
// rejected with 500. A request carrying no scope at all is unauthenticated;
// production mounts those routes outside TenantTxMiddleware, so they pass
// through here as well.
func TenantTxMiddleware(db *bun.DB) func(http.Handler) http.Handler {
	runtime, err := newTenantRuntime(db)
	if err != nil {
		panic(err)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.WithUnitOfWork(r.Context(), runtime)
			within := tenant.WithinCurrentTenant
			if _, tenantErr := tenant.TenantFromContext(ctx); tenantErr != nil {
				switch tenant.ScopeFromContext(ctx) {
				case tenant.ScopePlatform:
					within = tenant.WithinAdmin
				case "":
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				default:
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}
			status := http.StatusOK
			err := within(ctx, func(txCtx context.Context) error {
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
	return tenant.WithUnitOfWork(ctx, TenantRuntime(tb, db))
}
