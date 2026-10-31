package behavior_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The school's caterer page lists its invitations in a fixed number of
// statements, however many it sent (ADR 0077, #2940).
func TestCatererInvitationsQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caterers := setupAuthFactory(t, db).Caterers()
	school := testpkg.Tenant(t)
	staff := testpkg.CreateTestAccount(t, db, "caterer-budget-staff")
	add := func(count int) {
		for range count {
			invite(t, db, caterers, school, staff.ID, fmt.Sprintf("kueche-budget-%d@example.com", time.Now().UnixNano()))
		}
	}
	add(2)
	require.NoError(t, inSchool(t, db, school, func(ctx context.Context) error {
		counter := testpkg.CaptureQueriesForContext(t, db)
		countedCtx := counter.Context(ctx)
		run := func() []string {
			counter.Reset()
			_, err := caterers.ListCatererInvitations(countedCtx)
			require.NoError(t, err)
			return counter.Operation("SELECT")
		}
		small := run()
		add(4)
		large := run()
		assert.Equal(t, len(small), len(large), "the list must not grow a statement per invitation")
		querybudgets.AssertQueryBudget(t, "modules.identityaccess.caterer_invitations.reads", large)
		return nil
	}))
}

// The caterer portal names a caterer's schools in a fixed number of
// statements, however many schools it supplies (ADR 0077, #2940).
func TestCatererSchoolsQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caterers := setupAuthFactory(t, db).Caterers()
	first := testpkg.Tenant(t)
	staff := testpkg.CreateTestAccount(t, db, "caterer-schools-budget")
	email := fmt.Sprintf("kueche-schulen-%d@example.com", time.Now().UnixNano())
	registration := identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword}
	acceptance, err := accept(t, db, caterers, first, invite(t, db, caterers, first, staff.ID, email), registration)
	require.NoError(t, err)
	forgetCaterer(t, db, acceptance)

	counter := testpkg.CaptureQueriesForContext(t, db)
	run := func() []string {
		counter.Reset()
		_, _, err := caterers.CatererSchools(counter.Context(context.Background()), acceptance.AccountID)
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	small := run()
	for range 2 {
		school, _ := testpkg.CreateTestTenant(t, db)
		_, err := accept(t, db, caterers, school, invite(t, db, caterers, school, staff.ID, email), identityaccess.CatererRegistration{Password: catererPassword})
		require.NoError(t, err)
	}
	large := run()
	assert.Equal(t, len(small), len(large), "the read must not grow a statement per school")
	querybudgets.AssertQueryBudget(t, "modules.identityaccess.caterer_schools.reads", large)
}
