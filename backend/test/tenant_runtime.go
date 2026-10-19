package test

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type UnitOfWorkEvidence struct {
	Kind     string
	Result   string
	Retries  int
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
		events = append(events, UnitOfWorkEvidence{Kind: string(event.Kind), Result: string(event.Result), Retries: event.Retries, Duration: event.Duration})
	})
	return ctx, func() []UnitOfWorkEvidence { return append([]UnitOfWorkEvidence(nil), events...) }
}

// ContextForScope binds a portal scope for fixtures that verify wrong-portal
// and missing-school rejection without importing runtime internals.
func ContextForScope(ctx context.Context, scope string) context.Context {
	return tenant.WithScope(ctx, scope)
}

// ContextWithTenantRuntime binds a tenant runtime to ctx, for repository
// tests that drive their own transactions through it.
func ContextWithTenantRuntime(ctx context.Context, runtime tenant.UnitOfWork) context.Context {
	return tenant.WithUnitOfWork(ctx, runtime)
}

// ErrSavepointControl is the tenant runtime's failure to control a savepoint,
// for doubles that report untrusted transaction state.
var ErrSavepointControl = tenant.ErrSavepointControl

// ContextWithoutAfterCommitHooks masks the after-commit hooks ctx inherits,
// for tests of commands that refuse to run without them.
func ContextWithoutAfterCommitHooks(ctx context.Context) context.Context {
	return tenant.ContextWithoutAfterCommitHooks(ctx)
}

// TransactionFromContext returns the ambient tenant transaction of ctx.
func TransactionFromContext(ctx context.Context) (any, bool) {
	return tenant.TransactionFromContext(ctx)
}

// RunInTenantTransaction joins the ambient transaction or opens one for the
// context's tenant.
func RunInTenantTransaction(ctx context.Context, fn func(context.Context) error) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}

// WithinTenantTransaction opens a transaction for one school and hands the
// callback the Bun transaction.
func WithinTenantTransaction(ctx context.Context, db *bun.DB, tenantID int64, fn func(context.Context, bun.Tx) error) error {
	return tenant.WithTenantTx(ctx, db, tenantID, fn)
}

// WithinCurrentTenant runs a test callback through the bound production runtime.
func WithinCurrentTenant(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithinCurrentTenant(ctx, fn)
}

// WithinAdminTransaction runs a test callback in the administrative
// transaction of the bound runtime.
func WithinAdminTransaction(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithinAdmin(ctx, fn)
}

func AttachLockWaitEvidence(db *bun.DB) {
	transactionruntime.ObserveLockWaits(db)
}

var packageTenantRuntime atomic.Pointer[tenant.UnitOfWork]

func bindPackageTenantRuntime(db *bun.DB) error {
	runtime, err := transactionruntime.NewTenantRuntime(db)
	if err != nil {
		return err
	}
	packageTenantRuntime.Store(&runtime)
	return nil
}

// Database is the Transaction Runtime value the composition roots hold.
type Database = transactionruntime.Database

// NewDatabase hands a test database to a composition root as its Transaction
// Runtime value. Its Close closes db: never close the value of a SetupTestDB
// pool.
func NewDatabase(tb testing.TB, db *bun.DB) Database {
	tb.Helper()
	database, err := transactionruntime.NewDatabase(db)
	if err != nil {
		tb.Fatalf("create root database: %v", err)
	}
	return database
}

// MustDatabase is NewDatabase for fixture helpers that hold no testing.TB. A
// pool the tenant runtime cannot bind is a broken test setup and panics.
func MustDatabase(db *bun.DB) Database {
	database, err := transactionruntime.NewDatabase(db)
	if err != nil {
		panic(fmt.Sprintf("create root database: %v", err))
	}
	return database
}

// TenantRuntime builds the production tenant runtime against a test database.
func TenantRuntime(tb testing.TB, db *bun.DB) tenant.UnitOfWork {
	tb.Helper()
	runtime, err := transactionruntime.NewTenantRuntime(db)
	if err != nil {
		tb.Fatalf("create tenant runtime: %v", err)
	}
	return runtime
}

// PassthroughTenantRuntime is a tenant runtime without a database: every
// unit of work runs its callback in place, with the runtime's tenant
// scoping, after-commit hooks and observers. Unit tests use it where those
// semantics matter but no row does.
func PassthroughTenantRuntime() tenant.UnitOfWork {
	return ScriptedTenantRuntime(nil, nil)
}

// ScriptedTenantRuntime is PassthroughTenantRuntime with a scripted tenant
// transaction and retry decision, for tests of commit failures and
// retries. A nil withinTenant runs the callback in place; a nil retryable
// retries nothing.
func ScriptedTenantRuntime(withinTenant func(context.Context, int64, func(context.Context, any) error) error, retryable func(error) bool) tenant.UnitOfWork {
	if withinTenant == nil {
		withinTenant = func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		}
	}
	if retryable == nil {
		retryable = func(error) bool { return false }
	}
	runtime, err := tenant.NewUnitOfWork(
		withinTenant,
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		retryable,
	)
	if err != nil {
		panic(fmt.Sprintf("create scripted tenant runtime: %v", err))
	}
	return runtime
}

// TenantIDFromContext returns the tenant a unit of work runs for, or zero
// outside one.
func TenantIDFromContext(ctx context.Context) int64 {
	return tenant.FromContext(ctx)
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

// WithTestTenantRuntime attaches the runtime for tb's disposable clone when
// present, otherwise the package runtime.
func WithTestTenantRuntime(tb testing.TB, ctx context.Context) context.Context {
	tb.Helper()
	if value, ok := isolatedTestDatabases.Load(topLevelTestName(tb)); ok {
		return tenant.WithUnitOfWork(ctx, value.(*isolatedTestDatabase).runtime)
	}
	return WithPackageTenantRuntime(ctx)
}

// TenantRuntimeMiddleware mirrors what the production API root installs for
// every route, the tenant runtime and the session verifier, for tests that
// exercise a domain router in isolation.
func TenantRuntimeMiddleware(tb testing.TB, db *bun.DB) func(http.Handler) http.Handler {
	tb.Helper()
	runtime := TenantRuntime(tb, db)
	return func(next http.Handler) http.Handler {
		next = SessionVerifier(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), runtime)))
		})
	}
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

func (SettingsRuntimeAdapter) TodayTime() time.Time { return calendar.TodayDate().UTCMidnight() }

// LockStaffAssignment mirrors Membership.FindStaffForMutation for owner tests
// that cannot compose Membership through this package without an import cycle.
func (r SettingsRuntimeAdapter) LockStaffAssignment(ctx context.Context, staffID int64) error {
	if !r.HasTransaction(ctx) || r.TenantID(ctx) <= 0 {
		return fmt.Errorf("staff assignment requires a tenant transaction")
	}
	var id int64
	return r.DB(ctx).NewSelect().TableExpr("users.staff_school_memberships").Column("id").
		Where("tenant_id = ?", r.TenantID(ctx)).Where("id = ?", staffID).
		Where("deleted_at IS NULL").For("UPDATE").Scan(ctx, &id)
}

// LiveStaffIDs stands in for the School Membership liveness query the
// production roots bind Workforce to (#2753). The test package cannot compose
// that module (its own tests import this package), so it mirrors the owner's
// one statement directly. Keep it in step with modules/schoolmembership if the
// owner's semantics move.
func (r SettingsRuntimeAdapter) LiveStaffIDs(ctx context.Context, membershipIDs []int64) ([]int64, error) {
	ids := []int64{}
	if len(membershipIDs) == 0 {
		return ids, nil
	}
	query := r.DB(ctx).NewSelect().
		TableExpr(`users.staff_school_memberships AS "staff"`).
		ColumnExpr(`"staff".id`).
		Where(`"staff".id IN (?)`, bun.List(membershipIDs)).
		Where(`"staff".deleted_at IS NULL`).
		OrderExpr(`"staff".id ASC`)
	if tenantID := r.TenantID(ctx); tenantID > 0 {
		query = query.Where(`"staff".tenant_id = ?`, tenantID)
	}
	if err := query.Scan(ctx, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

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
	return tenant.WithTenantTx(WithTenantRuntime(tb, ctx, db), db, tenantID, fn)
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
	runtime, err := transactionruntime.NewTenantRuntime(db)
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
					writeTestProblem(w, http.StatusInternalServerError, codeGeneralServer)
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
				writeTestProblem(w, http.StatusInternalServerError, codeGeneralServer)
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

// TenantTransactionRuntime is the tenant runtime of the retained School
// Structure services (#2742) for suites that compose them without the
// legacy composition: transactions, the caller's school and the rollback
// mark come from the shared tenant runtime, like the production binding.
type TenantTransactionRuntime struct{}

// WithinTenant retains the explicit-school transaction boundary for owner
// tests that cannot import the production composition root without a cycle.
func (TenantTransactionRuntime) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		return err
	}
	return tenant.WithinTenant(ctx, id, fn)
}

func (TenantTransactionRuntime) WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	return WithinAdminTransaction(ctx, fn)
}

func (TenantTransactionRuntime) RegisterAfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

func (TenantTransactionRuntime) WithTenantID(ctx context.Context, tenantID int64) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

// RunInTx joins the ambient transaction or opens one for the context's
// tenant.
func (TenantTransactionRuntime) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	return RunInTenantTransaction(ctx, fn)
}

// TenantID is the school the caller acts for, 0 when none.
func (TenantTransactionRuntime) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

// MarkRollback asks the request's tenant transaction to roll back.
func (TenantTransactionRuntime) MarkRollback(ctx context.Context) { tenant.MarkRollback(ctx) }

// TenantQuery0 arranges the explicit unit required by raw owner adapters.
func TenantQuery0[T any](tb testing.TB, ctx context.Context, db *bun.DB, query func(context.Context) (T, error)) (value T, err error) {
	tb.Helper()
	err = WithinTenantContext(tb, ctx, db, tenant.FromContext(ctx), func(txCtx context.Context) error {
		value, err = query(txCtx)
		return err
	})
	return value, err
}

// TenantQuery1 arranges the explicit unit required by raw owner adapters.
func TenantQuery1[T any, A0 any](tb testing.TB, ctx context.Context, db *bun.DB, query func(context.Context, A0) (T, error), arg0 A0) (value T, err error) {
	tb.Helper()
	err = WithinTenantContext(tb, ctx, db, tenant.FromContext(ctx), func(txCtx context.Context) error {
		value, err = query(txCtx, arg0)
		return err
	})
	return value, err
}

// TenantQuery2 arranges the explicit unit required by raw owner adapters.
func TenantQuery2[T any, A0 any, A1 any](tb testing.TB, ctx context.Context, db *bun.DB, query func(context.Context, A0, A1) (T, error), arg0 A0, arg1 A1) (value T, err error) {
	tb.Helper()
	err = WithinTenantContext(tb, ctx, db, tenant.FromContext(ctx), func(txCtx context.Context) error {
		value, err = query(txCtx, arg0, arg1)
		return err
	})
	return value, err
}

// TenantQuery3 arranges the explicit unit required by raw owner adapters.
func TenantQuery3[T any, A0 any, A1 any, A2 any](tb testing.TB, ctx context.Context, db *bun.DB, query func(context.Context, A0, A1, A2) (T, error), arg0 A0, arg1 A1, arg2 A2) (value T, err error) {
	tb.Helper()
	err = WithinTenantContext(tb, ctx, db, tenant.FromContext(ctx), func(txCtx context.Context) error {
		value, err = query(txCtx, arg0, arg1, arg2)
		return err
	})
	return value, err
}
