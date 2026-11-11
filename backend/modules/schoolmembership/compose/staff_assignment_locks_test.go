package compose_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	membershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStaffAssignmentLocksValidateTheWholeRosterInOneQuery(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	employment := &assignmentEmployment{rosterEmployment: rosterEmployment{profiles: make(map[int64]membershipCompose.StaffEmploymentProfile)}}
	module, err := membershipCompose.New(membershipCompose.Dependencies{DB: db, Observe: func(membershipCompose.Observation) {}, Employment: employment})
	require.NoError(t, err)
	first := testpkg.CreateTestStaff(t, db, "First", "Assignment")
	second := testpkg.CreateTestStaff(t, db, "Second", "Assignment")
	retired := testpkg.CreateTestStaff(t, db, "Retired", "Assignment")
	require.NoError(t, module.DeleteStaff(ctx, retired.ID))
	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	foreign := testpkg.CreateTestStaffForTenant(t, db, foreignTenant, "Foreign", "Assignment")
	employment.reads = 0
	queries := testpkg.CaptureQueriesForContext(t, db)
	err = testpkg.WithinCurrentTenant(queries.Context(ctx), func(txCtx context.Context) error {
		return schoolmembership.LockStaffAssignments(txCtx, module, []int64{second.ID, first.ID, second.ID})
	})
	require.NoError(t, err)
	locks := queries.Selects("users.staff_school_memberships")
	require.Len(t, locks, 1)
	require.Contains(t, locks[0], `ORDER BY "staff".id ASC FOR NO KEY UPDATE OF staff`)
	require.Zero(t, employment.reads, "assignment locks must not load employment")
	for _, id := range []int64{retired.ID, foreign.ID} {
		err := schoolmembership.LockStaffAssignments(ctx, module, []int64{first.ID, id})
		require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	}
	queries.Reset()
	require.NoError(t, schoolmembership.LockStaffAssignments(queries.Context(ctx), module, nil))
	require.ErrorIs(t, schoolmembership.LockStaffAssignments(queries.Context(ctx), module, []int64{first.ID, 0}), schoolmembership.ErrInvalidMembership)
	require.Zero(t, queries.Total(), "empty or invalid ID sets must not query")
}

// assignmentEmployment records reads through the existing employment fixture.
type assignmentEmployment struct {
	rosterEmployment
	reads int
}

func (e *assignmentEmployment) StaffEmployments(ctx context.Context, ids []int64) (map[int64]membershipCompose.StaffEmploymentProfile, error) {
	e.reads++
	return e.rosterEmployment.StaffEmployments(ctx, ids)
}
