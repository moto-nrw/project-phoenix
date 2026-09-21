package repositories_test

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSessionStaffPreservesMembershipAndPersonWireFields(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "Jane", "Smith")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
	repos, err := repositories.NewActiveTestRepositories(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	want, err := repos.Staff.FindByID(ctx, staff.ID)
	require.NoError(t, err)
	want.Person, err = repos.Person.FindByID(ctx, staff.PersonID)
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
