package account_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The demo role parent (#3468): the parent session the demo token redeems
// into has full parents portal rights for exactly the one child of the
// visitor's parent and sees no other child. The parents portal is driven
// through its composed flows with the account of the issued token, and a
// request made there waits for the school.
func TestDemoParentHasFullRightsForExactlyTheOwnChild(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	_, module := testutil.SetupStudentModule(t)
	_, visitor := testpkg.CreateTestStaffWithAccount(t, env.db, "Kim", "Beispiel")
	testpkg.EnsureAccountTenant(t, env.db, visitor.ID, testpkg.Tenant(t))
	env.grantRole(t, visitor.ID, "user")
	own := testpkg.CreateTestParentGuardianChain(t, env.db)
	foreign := testpkg.CreateTestParentGuardianChain(t, env.db)
	token, slug := env.requestOwnSchool(t, env.address())
	seedDemoSchoolWithParent(t, env.db, slug, testpkg.Tenant(t), visitor.ID, own.AccountID)

	claims := env.enterAs(t, token, "parent").claims(t)
	require.Equal(t, "parent", claims.Scope)
	parent := int64(claims.ID)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, env.db))

	children := testutil.ParentPortalChildIDs(t, ctx, env.db, module, parent)
	require.Len(t, children, 1, "the visitor sees exactly the one child")
	assert.Equal(t, own.StudentID, children[0])

	// A new pickup time is a request the school decides on.
	testutil.EnableParentPickupTimeRequests(t, env.db, testpkg.Tenant(t))
	pickupChange := map[string]any{"weekdays": []any{map[string]any{
		"weekday": 2, "scheduled": true, "pickup": "15:30", "mode": "pickup",
	}}}
	_, err := testutil.SubmitParentCareScheduleRequest(t, ctx, env.db, module, parent, foreign.StudentID, pickupChange)
	require.Error(t, err, "no request for a foreign child")

	requestID, err := testutil.SubmitParentCareScheduleRequest(t, ctx, env.db, module, parent, own.StudentID, pickupChange)
	require.NoError(t, err, "the visitor may ask for a new pickup time")

	// The request waits for the OGS, where the visitor finds it as the lead.
	var status string
	require.NoError(t, env.db.NewRaw(`SELECT status FROM schedule.care_schedule_change_requests WHERE id = ? AND student_id = ? AND tenant_id = ?`,
		requestID, own.StudentID, testpkg.Tenant(t)).Scan(context.Background(), &status))
	assert.Equal(t, "pending", status)
}
