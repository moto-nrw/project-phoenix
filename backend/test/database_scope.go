package test

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/testdb"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// perTestDatabases is a package-level mode selected from TestMain. It is for
// packages whose tests mutate schema or other database-global state.
var perTestDatabases bool

// PerTestDatabases gives each top-level test a disposable clone. Tests keep
// calling SetupTestDB; only the exceptional package pays the isolation cost.
func PerTestDatabases() { perTestDatabases = true }

type isolatedTestDatabase struct {
	once    sync.Once
	db      *bun.DB
	clone   *testdb.CloneHandle
	runtime tenant.UnitOfWork
	err     error
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
		tb.Fatalf("setup isolated test database: %v", entry.err)
	}
	return entry.db
}

func (entry *isolatedTestDatabase) open(tb testing.TB, name string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	entry.clone, err = testdb.CreateClone(ctx, packageCloneCfg, testdb.RunID()+"-"+name)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			entry.drop()
		}
	}()

	if err = setCloneSearchPath(ctx, entry.clone); err != nil {
		return err
	}
	entry.db, err = openTestPool(entry.clone.DSN)
	if err != nil {
		return err
	}
	if err = initCloneBootstrap(ctx, entry.db); err != nil {
		return err
	}
	entry.runtime, err = newTenantRuntime(entry.db)
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

	tb.Cleanup(func() {
		entry.drop()
		isolatedTestDatabases.Delete(name)
		testTenants.Delete(name)
	})
	return nil
}

func setCloneSearchPath(ctx context.Context, clone *testdb.CloneHandle) error {
	db, err := openTestPool(clone.DSN)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	_, err = db.ExecContext(ctx, fmt.Sprintf(
		`ALTER DATABASE %q SET search_path TO public, platform, auth, users, education, facilities, activities, active, schedule, iot, feedback, config, meta, audit`,
		clone.Name))
	return err
}

func openTestPool(dsn string) (*bun.DB, error) {
	sqlDB := OpenPostgresSQL(dsn)
	sqlDB.SetMaxOpenConns(poolSize())
	sqlDB.SetMaxIdleConns(poolSize())
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	db := NewBunDB(sqlDB)
	var one int
	if err := db.NewSelect().ColumnExpr("1").Scan(context.Background(), &one); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to isolated test database: %w", err)
	}
	return db, nil
}

func (entry *isolatedTestDatabase) drop() {
	if entry.db != nil {
		_ = entry.db.Close()
	}
	if entry.clone == nil {
		return
	}
	_ = entry.clone.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = testdb.DropClone(ctx, packageCloneCfg, entry.clone.Name)
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
