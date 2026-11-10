package test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/testdb"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// perTestDatabases is a package-level mode selected from TestMain. It is for
// packages whose tests mutate schema or other database-global state.
var perTestDatabases bool

// PerTestDatabases gives each top-level test a disposable clone. Tests keep
// calling SetupTestDB; only the exceptional package pays the isolation cost.
func PerTestDatabases() { perTestDatabases = true }

// Explicit isolated clones are substantially heavier than tenant-isolated
// tests on the package database: each clone has its own pool and CREATE
// DATABASE copies the complete template. With -p 4 and -parallel 8, allowing
// every package test to clone at once creates an I/O spike of up to 32
// databases and makes otherwise unrelated queries time out. One explicit
// clone per package bounds that spike to four databases while ordinary tests
// and packages intentionally running wholly on per-test databases remain
// parallel.
const maxConcurrentIsolatedTestDatabases = 1

var isolatedTestDatabaseSlots = newIsolatedTestDatabaseLimiter(maxConcurrentIsolatedTestDatabases)

type isolatedTestDatabase struct {
	once        sync.Once
	dropOnce    sync.Once
	db          *bun.DB
	clone       *testdb.CloneHandle
	runtime     tenant.UnitOfWork
	releaseSlot func()
	err         error
	dropErr     error
}

var isolatedTestDatabases sync.Map // top-level test name -> *isolatedTestDatabase

func hasIsolatedTestDB(tb testing.TB) bool {
	_, ok := isolatedTestDatabases.Load(topLevelTestName(tb))
	return ok
}

func setupIsolatedTestDB(tb testing.TB) *bun.DB {
	tb.Helper()
	name := topLevelTestName(tb)
	value, _ := isolatedTestDatabases.LoadOrStore(name, &isolatedTestDatabase{})
	entry := value.(*isolatedTestDatabase)
	entry.once.Do(func() {
		entry.err = entry.open(tb, name)
	})
	if entry.err != nil {
		isolatedTestDatabases.CompareAndDelete(name, entry)
		tb.Fatalf("setup isolated test database: %v", entry.err)
	}
	return entry.db
}

func (entry *isolatedTestDatabase) open(tb testing.TB, name string) (err error) {
	if err = entry.acquireSlot(tb, name); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), isolatedTestDatabaseOpenTimeout)
	defer cancel()

	entry.clone, err = testdb.CreateClone(ctx, packageCloneCfg, testdb.RunID()+"-"+name)
	if err != nil {
		return errors.Join(err, entry.drop())
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, entry.drop())
		}
	}()

	if err = setCloneSearchPath(ctx, entry.clone); err != nil {
		return err
	}
	entry.db, err = openTestPool(ctx, entry.clone.DSN)
	if err != nil {
		return err
	}
	attachTestQueryHooks(entry.db)
	if err = initCloneBootstrap(ctx, entry.db); err != nil {
		return err
	}
	entry.runtime, err = transactionruntime.NewTenantRuntime(entry.db)
	if err != nil {
		return err
	}
	if value, ok := testTenants.Load(name); ok {
		if tenantID := value.(*tenantEntry).id; tenantID != 0 {
			if err = ensureTestTenant(ctx, entry.db, tenantID); err != nil {
				return err
			}
		}
	}

	entry.registerCleanup(tb, name)
	return nil
}

func (entry *isolatedTestDatabase) acquireSlot(tb testing.TB, name string) error {
	if perTestDatabases {
		return nil
	}
	var err error
	entry.releaseSlot, err = isolatedTestDatabaseSlots.acquireForTest(tb, name)
	return err
}

func (entry *isolatedTestDatabase) registerCleanup(tb testing.TB, name string) {
	tb.Cleanup(func() {
		if err := entry.drop(); err != nil {
			tb.Errorf("drop isolated test database: %v", err)
		}
		isolatedTestDatabases.Delete(name)
		testTenants.Delete(name)
	})
}

func setCloneSearchPath(ctx context.Context, clone *testdb.CloneHandle) error {
	db, err := openTestPool(ctx, clone.DSN)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	_, err = db.ExecContext(ctx, fmt.Sprintf(
		`ALTER DATABASE %q SET search_path TO public, platform, auth, users, education, facilities, activities, active, schedule, iot, feedback, config, meta, audit`,
		clone.Name))
	return err
}

func openTestPool(ctx context.Context, dsn string) (*bun.DB, error) {
	sqlDB := OpenPostgresSQL(dsn)
	sqlDB.SetMaxOpenConns(poolSize())
	sqlDB.SetMaxIdleConns(poolSize())
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	db := NewBunDB(sqlDB)
	var one int
	if err := db.NewSelect().ColumnExpr("1").Scan(ctx, &one); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to isolated test database: %w", err)
	}
	return db, nil
}

func (entry *isolatedTestDatabase) drop() error {
	entry.dropOnce.Do(func() {
		if entry.db != nil {
			detachTestQueryHooks(entry.db)
			entry.dropErr = errors.Join(entry.dropErr, entry.db.Close())
		}
		if entry.clone != nil {
			entry.dropErr = errors.Join(entry.dropErr, entry.clone.Close())
			ctx, cancel := context.WithTimeout(context.Background(), isolatedTestDatabaseDropTimeout)
			defer cancel()
			entry.dropErr = errors.Join(entry.dropErr, testdb.DropClone(ctx, packageCloneCfg, entry.clone.Name))
		}
		if entry.releaseSlot != nil {
			entry.releaseSlot()
		}
	})
	return entry.dropErr
}

func isolatedTestDSN(tb testing.TB) (string, bool) {
	value, ok := isolatedTestDatabases.Load(topLevelTestName(tb))
	if !ok {
		return "", false
	}
	entry := value.(*isolatedTestDatabase)
	return entry.clone.DSN, true
}

func authRoleDSN(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil {
		panic(fmt.Sprintf("parse isolated test DSN: %v", err))
	}
	parsed.User = url.UserPassword("phoenix_auth", testdb.AuthRolePassword)
	return parsed.String()
}

func testRuntimeContext(tb testing.TB) context.Context {
	if value, ok := isolatedTestDatabases.Load(topLevelTestName(tb)); ok {
		return tenant.WithUnitOfWork(context.Background(), value.(*isolatedTestDatabase).runtime)
	}
	return WithPackageTenantRuntime(context.Background())
}
