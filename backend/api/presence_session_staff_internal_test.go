package api

import (
	"encoding/json"
	"testing"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSessionStaffPreservesMembershipAndPersonWireFields(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "Jane", "Smith")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
	repos, err := NewActiveTestRepositories(testpkg.NewDatabase(t, db), owners.MustNewMembershipTestRepositories(testpkg.NewDatabase(t, db)))
	require.NoError(t, err)
	reposPeople := NewActiveTestPeopleRows(testpkg.NewDatabase(t, db), owners.MustNewMembershipTestRepositories(testpkg.NewDatabase(t, db)))
	ctx := testpkg.Ctx(t)
	want, err := reposPeople.Staff.FindByID(ctx, staff.ID)
	require.NoError(t, err)
	want.Person, err = reposPeople.Person.FindByID(ctx, staff.PersonID)
	require.NoError(t, err)
	projected, err := testpkg.SupervisionRowStaff(ctx, repos.ActiveGroup, group.ID)
	require.NoError(t, err)
	require.Len(t, projected, 1)
	require.NotNil(t, projected[0])
	require.NotNil(t, projected[0].Person)
	wantJSON, err := json.Marshal(want)
	require.NoError(t, err)
	gotJSON, err := json.Marshal(projected[0])
	require.NoError(t, err)
	require.JSONEq(t, string(wantJSON), string(gotJSON))
	require.Equal(t, "Jane Smith", projected[0].Person.GetFullName())
}
