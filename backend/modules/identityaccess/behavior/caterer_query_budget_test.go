package behavior_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
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
		querybudgets.AssertQueryBudget(t, "modules.identityaccess.caterer_invitations.reads", counter.Measurement(large))
		return nil
	}))
}

// The school sees what became of each invitation's mail, read for the whole
// list in one statement, however many invitations it sent (#5960, #2940).
func TestCatererInvitationMailStatesShowEachMailInOneRead(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caterers := setupAuthFactory(t, db).Caterers()
	school := testpkg.Tenant(t)
	staff := testpkg.CreateTestAccount(t, db, "caterer-mail-staff")
	add := func(count int) {
		for range count {
			invite(t, db, caterers, school, staff.ID, fmt.Sprintf("kueche-mail-%d@example.com", time.Now().UnixNano()))
		}
	}
	add(2)
	require.NoError(t, inSchool(t, db, school, func(ctx context.Context) error {
		listed, err := caterers.ListCatererInvitations(ctx)
		require.NoError(t, err)
		require.Len(t, listed, 2)
		bounced, waiting := listed[0].ID, listed[1].ID
		_, err = db.ExecContext(ctx, `UPDATE platform.email_outbox SET status = 'sent', sent_at = NOW(), delivery_state = 'failed'
			WHERE kind = 'caterer_invitation' AND related_entity_id = ?`, bounced)
		require.NoError(t, err)
		states, err := caterers.CatererInvitationMailStates(ctx, []int64{bounced, waiting})
		require.NoError(t, err)
		assert.Equal(t, "undeliverable", states[bounced].Outcome, "the school sees that the mail did not arrive")
		assert.Equal(t, "queued", states[waiting].Outcome)

		counter := testpkg.CaptureQueriesForContext(t, db)
		countedCtx := counter.Context(ctx)
		run := func() []string {
			invitations, err := caterers.ListCatererInvitations(ctx)
			require.NoError(t, err)
			ids := make([]int64, 0, len(invitations))
			for _, invitation := range invitations {
				ids = append(ids, invitation.ID)
			}
			counter.Reset()
			_, err = caterers.CatererInvitationMailStates(countedCtx, ids)
			require.NoError(t, err)
			return counter.Operation("SELECT")
		}
		small := run()
		add(4)
		large := run()
		assert.Equal(t, len(small), len(large), "the read must not grow a statement per invitation")
		querybudgets.AssertQueryBudget(t, "modules.identityaccess.caterer_invitation_mail_states.reads", counter.Measurement(large))
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
	querybudgets.AssertQueryBudget(t, "modules.identityaccess.caterer_schools.reads", counter.Measurement(large))
}

// The operator lists the caterer accounts and their second factor in a
// fixed number of statements, however many caterers there are (#4497,
// #2940).
func TestCatererOperatorListQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caterers := setupAuthFactory(t, db).Caterers()
	staff := testpkg.CreateTestAccount(t, db, "caterer-operator-budget")
	add := func() {
		school, _ := testpkg.CreateTestTenant(t, db)
		email := fmt.Sprintf("kueche-operator-%d@example.com", time.Now().UnixNano())
		acceptance, err := accept(t, db, caterers, school, invite(t, db, caterers, school, staff.ID, email), identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword})
		require.NoError(t, err)
		forgetCaterer(t, db, acceptance)
	}
	counter := testpkg.CaptureQueriesForContext(t, db)
	run := func() []string {
		counter.Reset()
		_, err := caterers.ListCaterersForOperator(counter.Context(context.Background()))
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	add()
	small := run()
	add()
	add()
	large := run()
	assert.Equal(t, len(small), len(large), "the list must not grow a statement per caterer")
	querybudgets.AssertQueryBudget(t, "modules.identityaccess.caterers.operator_list", counter.Measurement(large))
}
