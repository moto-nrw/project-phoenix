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
	waiting := orderDemoSchool(t, db, "OGS West")
	entered := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	for _, slug := range []string{first, second} {
		require.NoError(t, testpkg.WithinAdminContext(t, ctx, db, func(ctx context.Context) error {
			return NewDemoSchoolOrders(strings.NewReader("k3m9xp"), testDemoCapacity).MarkDemoSchoolUsed(ctx, slug, entered)
		}))
	}

	// The demo process hides the schools whose last access expired; an order
	// still waiting for its seed is closed, and an unknown slug is passed over.
	hidden, err := queue.RetireDemoSchools(ctx, []string{first, waiting, "nobody-ordered-this"})
	require.NoError(t, err)
	assert.Equal(t, 1, hidden)
	assert.True(t, schoolHidden(t, db, firstID))
	assert.False(t, schoolHidden(t, db, secondID))
	assert.Equal(t, organizationtenancy.DemoSchoolFailed, demoSchoolProgress(t, db, waiting).Status, "a waiting order nobody can enter is not seeded")
	none, err := queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	assert.Nil(t, none)
	hidden, err = queue.RetireDemoSchools(ctx, []string{first})
	require.NoError(t, err)
	assert.Zero(t, hidden, "a school hidden before counts nothing")
	hidden, err = queue.RetireDemoSchools(ctx, nil)
	require.NoError(t, err)
	assert.Zero(t, hidden)

	ready, err := queue.ReadyDemoSchools(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{second}, ready, "a hidden school is not ticked by --once")
	active, err := queue.ActiveDemoSchools(ctx, entered.Add(-time.Minute))
	require.NoError(t, err)
	assert.Equal(t, []string{second}, active, "a hidden school is not simulated, even if it was just entered")
	assert.Equal(t, organizationtenancy.DemoSchoolReady, demoSchoolProgress(t, db, first).Status, "the order keeps its progress; the school itself is what is gone")

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
