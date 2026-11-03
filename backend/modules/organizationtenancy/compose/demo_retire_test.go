package compose

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// A hidden demo school (#3470): soft-deleted through its owner, out of the
// simulation, out of the capacity, and no longer to be entered. The demo
// process hides expired schools in bulk, the serving backend hides one when
// its visitor starts over.

func readyDemoSchool(t *testing.T, db *bun.DB, queue *organizationtenancy.DemoSchoolQueue, schools *organizationtenancy.DemoSchools, schoolName string, schoolID int64) string {
	t.Helper()
	slug := orderDemoSchool(t, db, schoolName)
	_, err := queue.ClaimDemoSchoolOrder(t.Context())
	require.NoError(t, err)
	require.NoError(t, schools.RememberDemoSchool(t.Context(), slug, organizationtenancy.DemoSchoolState{SchoolID: schoolID, SeedJSON: []byte(`{}`)}))
	require.NoError(t, queue.FinishDemoSchoolOrder(t.Context(), slug, 0, 0))
	return slug
}

// giveDemoAccess gives the order the demo access of its visitor. Every
// order is born with its access in one transaction; an order no access
// names is out of use, and every run retires it (#5540).
func giveDemoAccess(t *testing.T, db *bun.DB, slug string) {
	t.Helper()
	_, err := db.NewRaw(`INSERT INTO auth.demo_accesses (email, school_name, token_hash, expires_at, school_slug, first_name, last_name)
		VALUES ('besuch-' || ?0 || '@example.org', 'OGS', 'hash-' || ?0, NOW() + INTERVAL '14 days', ?0, 'Anna Lena', 'von Berg')`, slug).Exec(t.Context())
	require.NoError(t, err)
}

func schoolHidden(t *testing.T, db *bun.DB, schoolID int64) bool {
	t.Helper()
	var hidden bool
	require.NoError(t, db.NewRaw(`SELECT deleted_at IS NOT NULL FROM platform.schools WHERE id = ?`, schoolID).Scan(t.Context(), &hidden))
	return hidden
}

func TestRetiredDemoSchoolsLeaveTheSimulationAndTheCapacity(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	firstID, _ := testpkg.CreateTestTenant(t, db)
	secondID, _ := testpkg.CreateTestTenant(t, db)
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	ctx := t.Context()
	first := readyDemoSchool(t, db, queue, schools, "OGS Nord", firstID)
	second := readyDemoSchool(t, db, queue, schools, "OGS Süd", secondID)
	giveDemoAccess(t, db, second) // its visitor still uses it
	waiting := orderDemoSchool(t, db, "OGS West")
	entered := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	for _, slug := range []string{first, second} {
		require.NoError(t, testpkg.WithinAdminContext(t, ctx, db, func(ctx context.Context) error {
			return NewDemoSchoolOrders(strings.NewReader("k3m9xp"), testDemoCapacity).MarkDemoSchoolUsed(ctx, slug, entered)
		}))
	}

	// The demo process hides the schools whose last access expired; an order
	// still waiting for its seed is closed, and an unknown slug is passed over.
	// No access names either order any more, so both lose their visitor
	// (#4048).
	hidden, erased, err := queue.RetireDemoSchools(ctx, []string{first, waiting, "nobody-ordered-this"})
	require.NoError(t, err)
	assert.Equal(t, 1, hidden)
	assert.Equal(t, 2, erased)
	assert.True(t, schoolHidden(t, db, firstID))
	assert.False(t, schoolHidden(t, db, secondID))
	assert.Nil(t, demoSchoolProgress(t, db, waiting), "a waiting order nobody can enter is closed and erased, not seeded")
	none, err := queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	assert.Nil(t, none)
	hidden, erased, err = queue.RetireDemoSchools(ctx, []string{first})
	require.NoError(t, err)
	assert.Zero(t, hidden, "a school hidden before counts nothing")
	assert.Zero(t, erased, "an order erased before counts nothing")
	hidden, erased, err = queue.RetireDemoSchools(ctx, nil)
	require.NoError(t, err)
	assert.Zero(t, hidden)
	assert.Zero(t, erased)

	ready, err := queue.ReadyDemoSchools(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{second}, ready, "a hidden school is not ticked by --once")
	active, err := queue.ActiveDemoSchools(ctx, entered.Add(-time.Minute))
	require.NoError(t, err)
	assert.Equal(t, []string{second}, active, "a hidden school is not simulated, even if it was just entered")
	assert.Nil(t, demoSchoolProgress(t, db, first), "the order of a hidden school no access names is erased (#4048)")

	// The capacity: the hidden school and the closed order hold no place,
	// the remaining ready school does.
	order := func(schoolName string) error {
		return testpkg.WithinAdminContext(t, ctx, db, func(ctx context.Context) error {
			_, err := orderDemoSchoolWithin(ctx, schoolName, 3)
			return err
		})
	}
	require.NoError(t, order("OGS Ost"), "the hidden school's place is free")
	require.NoError(t, order("OGS Mitte"), "the closed order's place is free")
	require.ErrorIs(t, order("OGS Hafen"), organizationtenancy.ErrDemoCapacityReached)
}

func TestRetiringDemoSchoolWithAnActiveAccessLeavesItOpen(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	slug := readyDemoSchool(t, db, queue, schools, "OGS Nord", schoolID)
	_, err = db.NewRaw(`INSERT INTO auth.demo_accesses (email, school_name, token_hash, expires_at, school_slug, first_name, last_name)
		VALUES ('besuch@example.test', 'OGS Nord', 'active-access', NOW() + INTERVAL '1 day', ?, 'Zyxwolda', 'Quorbelt')`, slug).Exec(t.Context())
	require.NoError(t, err)

	hidden, erased, err := queue.RetireDemoSchools(t.Context(), []string{slug})
	require.NoError(t, err)
	assert.Zero(t, hidden)
	assert.Zero(t, erased)
	assert.False(t, schoolHidden(t, db, schoolID))
	assert.NotNil(t, demoSchoolProgress(t, db, slug))
}

// While the seed of an order holds its lease, the run neither hides its
// school nor erases its visitor. Since #5540 the run does not wait for the
// lease either: it leaves the order for the next run, which retires it.
func TestRetiringDemoSchoolDefersDuringItsSeedLease(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	slug := readyDemoSchool(t, db, queue, schools, "OGS Nord", schoolID)

	require.NoError(t, schools.WithDemoLease(t.Context(), "demo school seed:"+slug, func(context.Context) error {
		hidden, erased := retireNow(t, queue, []string{slug})
		assert.Zero(t, hidden, "the run hides no school during its seed lease")
		assert.Zero(t, erased, "the run erases no visitor during the seed lease")
		assert.False(t, schoolHidden(t, db, schoolID))
		assert.NotNil(t, demoSchoolProgress(t, db, slug))
		return nil
	}))

	// The next run is told nothing: it finds the order in the database.
	hidden, erased := retireNow(t, queue, nil)
	assert.Equal(t, 1, hidden)
	assert.Equal(t, 1, erased)
}

// While another run holds the run lock, this run neither hides nor erases
// anything. Since #5540 it does not wait for the lock either: it skips, and
// the next run retires the order.
func TestRetiringDemoSchoolsSkipsDuringAnotherRunsLease(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	slug := readyDemoSchool(t, db, queue, schools, "OGS Nord", schoolID)

	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	_, err = tx.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:visitor-erasure', 0))`).Exec(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	hidden, erased := retireNow(t, queue, []string{slug})
	assert.Zero(t, hidden, "the run hides no school during another run's lease")
	assert.Zero(t, erased)
	assert.False(t, schoolHidden(t, db, schoolID))
	require.NoError(t, tx.Rollback())

	hidden, erased = retireNow(t, queue, nil)
	assert.Equal(t, 1, hidden)
	assert.Equal(t, 1, erased)
}

// The serving backend hides the visitor's school when the visitor starts over
// (#3470), inside its administrative transaction and with its restricted role.
func TestDemoSchoolOrdersRetireOneSchoolForARestart(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	slug := readyDemoSchool(t, db, queue, schools, "OGS Nord", schoolID)
	orders := NewDemoSchoolOrders(strings.NewReader("k3m9xp"), testDemoCapacity)

	require.NoError(t, testpkg.WithinAdminContext(t, t.Context(), db, func(ctx context.Context) error {
		return orders.RetireDemoSchool(ctx, slug)
	}))
	assert.True(t, schoolHidden(t, db, schoolID))
	require.NoError(t, testpkg.WithinAdminContext(t, t.Context(), db, func(ctx context.Context) error {
		return orders.RetireDemoSchool(ctx, "nobody-ordered-this")
	}), "an unknown order hides nothing and fails nothing")
	require.Error(t, orders.RetireDemoSchool(t.Context(), ""))
}
