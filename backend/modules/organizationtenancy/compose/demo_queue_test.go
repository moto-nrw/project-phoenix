package compose

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// order queues a demo school the way the serving backend does: inside an
// administrative transaction, with its restricted role.
func orderDemoSchool(t *testing.T, db *bun.DB, schoolName string) string {
	t.Helper()
	var slug string
	require.NoError(t, testpkg.WithinAdminContext(t, t.Context(), db, func(ctx context.Context) error {
		var err error
		slug, err = NewDemoSchoolOrders(strings.NewReader("k3m9xp")).OrderDemoSchool(ctx, schoolName, "Kim Beispiel")
		return err
	}))
	return slug
}

func demoSchoolProgress(t *testing.T, db *bun.DB, slug string) *organizationtenancy.DemoSchoolProgress {
	t.Helper()
	var progress *organizationtenancy.DemoSchoolProgress
	require.NoError(t, testpkg.WithinAdminContext(t, t.Context(), db, func(ctx context.Context) error {
		var err error
		progress, err = NewDemoSchoolOrders(strings.NewReader("k3m9xp")).DemoSchoolProgress(ctx, slug)
		return err
	}))
	return progress
}

// The queue is shared state, so the test owns its database.
func TestDemoSchoolQueueSeedsInOrderRepeatsOnceAndFails(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	ctx := t.Context()

	first, second := orderDemoSchool(t, db, "OGS Nord"), orderDemoSchool(t, db, "OGS Süd")
	assert.Regexp(t, `^ogs-nord-[a-z0-9]{6}$`, first)
	assert.Regexp(t, `^ogs-sued-[a-z0-9]{6}$`, second)
	assert.Equal(t, organizationtenancy.DemoSchoolPreparing, demoSchoolProgress(t, db, first).Status)
	assert.Nil(t, demoSchoolProgress(t, db, "nobody-ordered-this"))

	// Orders leave the queue oldest first, and a claimed order is not handed out twice.
	order, err := queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, organizationtenancy.DemoSchoolOrder{Slug: first, SchoolName: "OGS Nord", PersonName: "Kim Beispiel", Attempts: 1}, *order)
	other, err := queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	require.NotNil(t, other)
	assert.Equal(t, second, other.Slug)
	none, err := queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	assert.Nil(t, none, "further workers find nothing to seed")

	// The first order is seeded and opened with the visitor's account.
	require.NoError(t, schools.RememberDemoSchool(ctx, first, organizationtenancy.DemoSchoolState{SchoolID: schoolID, SeedJSON: []byte(`{"profile":"vollbetrieb"}`)}))
	require.Error(t, schools.RememberDemoSchool(ctx, first, organizationtenancy.DemoSchoolState{SchoolID: schoolID, SeedJSON: []byte(`{}`)}), "seeded credentials are never overwritten")
	require.NoError(t, queue.FinishDemoSchoolOrder(ctx, first, 4711))
	progress := demoSchoolProgress(t, db, first)
	assert.Equal(t, organizationtenancy.DemoSchoolProgress{Status: organizationtenancy.DemoSchoolReady, SchoolID: schoolID, VisitorAccountID: 4711}, *progress)
	ready, err := queue.ReadyDemoSchools(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{first}, ready)

	// The second order fails: it is repeated once and then closed for good.
	failed, err := queue.FailDemoSchoolOrder(ctx, second, 2)
	require.NoError(t, err)
	assert.False(t, failed)
	again, err := queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	require.NotNil(t, again)
	assert.Equal(t, second, again.Slug)
	assert.Equal(t, 2, again.Attempts)
	failed, err = queue.FailDemoSchoolOrder(ctx, second, 2)
	require.NoError(t, err)
	assert.True(t, failed)
	assert.Equal(t, organizationtenancy.DemoSchoolFailed, demoSchoolProgress(t, db, second).Status)
	none, err = queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	assert.Nil(t, none)
}

// A stopped demo process leaves its claims behind; the next one takes them
// over, and an order whose seed was already stored is not seeded again.
func TestDemoSchoolQueueReleasesTheClaimsOfAStoppedProcess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	ctx := t.Context()

	slug := orderDemoSchool(t, db, "OGS Nord")
	_, err = queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	// What stopped this seed was not the order's fault: it returns uncounted.
	require.NoError(t, queue.ReturnDemoSchoolOrder(ctx, slug))
	returned, err := queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	require.NotNil(t, returned)
	assert.Equal(t, 1, returned.Attempts)
	require.NoError(t, schools.RememberDemoSchool(ctx, slug, organizationtenancy.DemoSchoolState{SchoolID: schoolID, SeedJSON: []byte(`{}`)}))

	require.NoError(t, queue.ReleaseDemoSchoolOrders(ctx))
	order, err := queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, slug, order.Slug)
	assert.True(t, order.Seeded)
}

// The serving backend queues and reads progress; the seed state with its
// credentials stays out of its reach, and it cannot open a school itself.
func TestDemoSchoolOrdersCannotReadOrForgeSeedState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, statement := range []string{
		`SELECT seed_state FROM platform.demo_school_states LIMIT 1`,
		`INSERT INTO platform.demo_school_states (name, status) VALUES ('forged-order', 'failed')`,
		`UPDATE platform.demo_school_states SET status = 'ready'`,
		`DELETE FROM platform.demo_school_states`,
	} {
		err := testpkg.WithAdminTx(t, t.Context(), db, func(ctx context.Context, tx bun.Tx) error {
			_, err := tx.NewRaw(statement).Exec(ctx)
			return err
		})
		assert.Error(t, err, statement)
	}
}
