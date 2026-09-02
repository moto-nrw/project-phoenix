package repositories_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBindSchoolStructureRequiresQuery(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { (&repositories.Factory{}).BindSchoolStructure(nil) })
}

// The supervision blocker read used to LEFT JOIN education.groups on
// group_supervisors.group_id itself. After the cutover the owner query
// answers inside the same tenant transaction; a row whose group the owner
// cannot see keeps the former fallback label instead of an empty name.
func TestSupervisionBlockersResolveGroupNamesThroughTheOwner(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)

	staff := testpkg.CreateTestStaff(t, db, "Blocker", "Supervisor")
	activity := testpkg.CreateTestActivityGroup(t, db, "Blocker Activity")
	room := testpkg.CreateTestRoom(t, db, "Blocker Room")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	supervision := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")

	groups, err := repositories.NewSchoolStructure(db)
	require.NoError(t, err)
	factory := repositories.NewFactory(db)
	factory.BindSchoolStructure(groups)

	err = testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		rows, err := factory.GroupSupervisor.ListActiveSupervisionBlockers(ctx, staff.ID, tenantID)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, supervision.ID, rows[0].ID)
		assert.Equal(t, group.ID, rows[0].GroupID)
		assert.NotEmpty(t, rows[0].GroupName, "resolved name or the fallback label, never blank")
		return nil
	})
	require.NoError(t, err)
}

func TestStudentRosterGroupNamesAreEmptyWithoutGroup(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.CreateTestStudent(t, db, "Ohne", "Gruppe", "1a")

	groups, err := repositories.NewSchoolStructure(db)
	require.NoError(t, err)
	factory := repositories.NewFactory(db)
	factory.BindSchoolStructure(groups)

	rows, err := factory.Student.FindAllWithGroups(testpkg.Ctx(t))
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, row := range rows {
		if row.GroupID == nil {
			assert.Empty(t, row.GroupName)
		}
	}
}
