package compose_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// #6316: the runtime decides a retry on the operation's own error only. A
// failed savepoint control (ROLLBACK TO, RELEASE after rollback) is fatal:
// 500, no retry, nothing written (E19). A failure of the outer ROLLBACK is
// diagnostics attached to the operation's error and decides nothing.

// cleanupSite is the cleanup statement a cell fails.
type cleanupSite string

const (
	siteNone             cleanupSite = "none"
	siteSavepointRestore cleanupSite = "ROLLBACK TO SAVEPOINT"
	siteSavepointRelease cleanupSite = "RELEASE SAVEPOINT"
	siteOuterRollback    cleanupSite = "ROLLBACK"
)

// faultKinds are the failures a main statement or a cleanup statement ends
// with; "" is an error without SQLSTATE, "bad_conn" is driver.ErrBadConn.
var mainKinds = []string{"", "23514", "57014", "40P01", "40001"}
var cleanupKinds = []string{"", "23514", "57014", "40P01", "40001", "bad_conn"}

func injected(kind string) error {
	switch kind {
	case "":
		return errors.New("injected failure without SQLSTATE")
	case "bad_conn":
		return driver.ErrBadConn
	default:
		return pgdriver.NewError(map[byte]string{'S': "ERROR", 'V': "ERROR", 'C': kind, 'M': "injected " + kind})
	}
}

func kindName(kind string) string {
	if kind == "" {
		return "no_sqlstate"
	}
	return kind
}

// cleanupFault fails the first cleanup statement at site once it is armed.
// The real statement runs first, so the connection's state stays consistent
// and only the reported result differs.
type cleanupFault struct {
	mu    sync.Mutex
	site  cleanupSite
	err   error
	fired bool
}

func (f *cleanupFault) arm(site cleanupSite, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.site, f.err, f.fired = site, err, false
}

func (f *cleanupFault) take(site cleanupSite) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fired || f.site != site {
		return nil
	}
	f.fired = true
	return f.err
}

func (f *cleanupFault) didFire() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fired
}

type cleanupConnector struct {
	driver.Connector
	fault *cleanupFault
}

func (c cleanupConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &cleanupConn{Conn: conn, fault: c.fault}, nil
}

type cleanupConn struct {
	driver.Conn
	fault *cleanupFault
}

func (c *cleanupConn) site(query string) cleanupSite {
	query = strings.TrimSpace(query)
	for _, site := range []cleanupSite{siteSavepointRestore, siteSavepointRelease} {
		if strings.HasPrefix(query, string(site)) {
			return site
		}
	}
	return siteNone
}

func (c *cleanupConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	result, err := c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
	if site := c.site(query); site != siteNone {
		if fault := c.fault.take(site); fault != nil {
			return nil, fault
		}
	}
	return result, err
}

func (c *cleanupConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}

func (c *cleanupConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return cleanupTx{Tx: tx, fault: c.fault}, nil
}

func (c *cleanupConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *cleanupConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *cleanupConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

type cleanupTx struct {
	driver.Tx
	fault *cleanupFault
}

func (t cleanupTx) Rollback() error {
	err := t.Tx.Rollback()
	if fault := t.fault.take(siteOuterRollback); fault != nil {
		return fault
	}
	return err
}

// retryOutcome is what a cell observes: the unit's error (500 when set,
// 200 otherwise), how often the callback ran and how many rows committed.
type retryOutcome struct {
	failed   bool
	attempts int
	rows     int
}

func (o retryOutcome) String() string {
	status := 200
	if o.failed {
		status = 500
	}
	return fmt.Sprintf("%d/%d attempts/%d rows", status, o.attempts, o.rows)
}

type retryRig struct {
	super  *bun.DB
	fault  *cleanupFault
	ctx    context.Context
	tenant tenant.TenantID
}

func newRetryRig(t *testing.T) *retryRig {
	t.Helper()
	super := testpkg.SetupIsolatedTestDB(t)
	_, err := super.ExecContext(context.Background(), `CREATE TABLE public.retry_probe (attempt int NOT NULL);
GRANT SELECT, INSERT ON public.retry_probe TO phoenix_tenant, phoenix_admin`)
	require.NoError(t, err)
	fault := &cleanupFault{}
	pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(sql.OpenDB(cleanupConnector{
		Connector: pgdriver.NewConnector(pgdriver.WithDSN(testpkg.IsolatedTestDSN(t))), fault: fault,
	})))
	t.Cleanup(func() { _ = pool.Close() })
	runtime, err := compose.NewTenantRuntime(pool)
	require.NoError(t, err)
	return &retryRig{
		super: super,

		fault:  fault,
		ctx:    tenant.WithUnitOfWork(context.Background(), runtime),
		tenant: school(t, testpkg.Tenant(t)),
	}
}

// raise ends the current statement with a real server error of kind; an
// empty kind is an operation error without SQLSTATE.
func raise(ctx context.Context, kind string) error {
	if kind == "" {
		return errors.New("operation failed without SQLSTATE")
	}
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return errors.New("no transaction")
	}
	_, err := raw.(bun.Tx).ExecContext(ctx, fmt.Sprintf(
		`DO $$ BEGIN RAISE EXCEPTION 'probe %s' USING ERRCODE = '%s'; END $$`, kind, kind))
	if err == nil {
		return errors.New("raise returned no error")
	}
	return err
}

// run executes one unit: the callback writes one row, then fails in a
// savepoint with mainKind on the first failingAttempts attempts.
func (r *retryRig) run(t *testing.T, admin bool, mainKind string, failingAttempts int) retryOutcome {
	t.Helper()
	ctx, cancel := context.WithTimeout(r.ctx, 20*time.Second)
	defer cancel()
	_, err := r.super.ExecContext(ctx, `TRUNCATE public.retry_probe`)
	require.NoError(t, err)

	var attempts atomic.Int32
	body := func(ctx context.Context) error {
		attempt := int(attempts.Add(1))
		raw, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return errors.New("no transaction")
		}
		if _, err := raw.(bun.Tx).ExecContext(ctx, `INSERT INTO public.retry_probe (attempt) VALUES (?)`, attempt); err != nil {
			return err
		}
		return tenant.WithSavepoint(ctx, func(ctx context.Context) error {
			if attempt > failingAttempts {
				return nil
			}
			return raise(ctx, mainKind)
		})
	}
	if admin {
		err = tenant.WithinAdminRetry(ctx, body)
	} else {
		err = tenant.WithinTenantRetry(ctx, r.tenant, body)
	}
	require.NoError(t, ctx.Err(), "the unit ended within its deadline")
	var rows int
	require.NoError(t, r.super.NewRaw(`SELECT count(*) FROM public.retry_probe`).Scan(context.Background(), &rows))
	return retryOutcome{failed: err != nil, attempts: int(attempts.Load()), rows: rows}
}

var (
	failedOnce   = retryOutcome{failed: true, attempts: 1}
	retriedOnce  = retryOutcome{attempts: 2, rows: 1}
	retriedFully = retryOutcome{failed: true, attempts: 4}
)

// expectedCell is the #6316 contract for a main failure that occurs once.
func expectedCell(mainKind string, site cleanupSite) retryOutcome {
	retryableMain := mainKind == "40P01" || mainKind == "40001"
	switch site {
	case siteSavepointRestore, siteSavepointRelease:
		return failedOnce
	}
	if mainKind == "57014" {
		// pgdriver closes the connection on 57014, so ROLLBACK TO SAVEPOINT
		// fails for real: savepoint control failed, fatal.
		return failedOnce
	}
	if retryableMain {
		return retriedOnce
	}
	return failedOnce
}

func TestRetryDecidesOnTheOperationErrorOnly(t *testing.T) {
	t.Parallel()
	rig := newRetryRig(t)
	type cell struct {
		site cleanupSite
		kind string
	}
	cells := []cell{{site: siteNone}}
	for _, site := range []cleanupSite{siteSavepointRestore, siteSavepointRelease, siteOuterRollback} {
		for _, kind := range cleanupKinds {
			cells = append(cells, cell{site: site, kind: kind})
		}
	}
	for _, unit := range []struct {
		name  string
		admin bool
	}{{"tenant", false}, {"admin", true}} {
		for _, mainKind := range mainKinds {
			for _, c := range cells {
				name := fmt.Sprintf("%s/main_%s/%s_%s", unit.name, kindName(mainKind), strings.ReplaceAll(string(c.site), " ", "_"), kindName(c.kind))
				if c.site == siteNone {
					name = fmt.Sprintf("%s/main_%s/cleanup_ok", unit.name, kindName(mainKind))
				}
				t.Run(name, func(t *testing.T) {
					rig.fault.arm(c.site, injected(c.kind))
					got := rig.run(t, unit.admin, mainKind, 1)
					want := expectedCell(mainKind, c.site)
					t.Logf("CELL %s %s", name, got)
					require.Equal(t, want, got)
					if c.site != siteNone && mainKind != "57014" {
						require.True(t, rig.fault.didFire(), "the cleanup fault fired")
					}
				})
			}
		}
	}
}

// A real deadlock or serialization failure with an intact savepoint keeps
// the full retry budget, also when the outer ROLLBACK of a failed attempt
// reports an error of its own.
func TestRetryKeepsItsBudgetDespiteOuterRollbackDiagnostics(t *testing.T) {
	t.Parallel()
	rig := newRetryRig(t)
	for _, admin := range []bool{false, true} {
		for _, mainKind := range []string{"40P01", "40001"} {
			for _, kind := range append([]string{"ok"}, cleanupKinds...) {
				name := fmt.Sprintf("admin_%t/main_%s/outer_rollback_%s", admin, mainKind, kindName(kind))
				t.Run(name, func(t *testing.T) {
					if kind == "ok" {
						rig.fault.arm(siteNone, nil)
					} else {
						rig.fault.arm(siteOuterRollback, injected(kind))
					}
					got := rig.run(t, admin, mainKind, 4)
					t.Logf("CELL %s %s", name, got)
					require.Equal(t, retriedFully, got)
				})
			}
		}
	}
}
