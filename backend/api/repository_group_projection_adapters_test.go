package api

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The test repositories bind the School Structure query at construction; without it
// the group-enriched adapters could not name a group, so it refuses.
func TestNewFactoryRequiresSchoolStructure(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { newPeopleRepositorySuiteFactory(testpkg.Database{}, nil, nil, nil, nil, nil, nil, nil) })
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

	factory := NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))

	err := testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		rows, err := factory.GroupSupervisor.ListActiveSupervisionBlockers(ctx, staff.ID)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, supervision.ID, rows[0].ID)
		assert.Equal(t, group.ID, rows[0].GroupID)
		assert.NotEmpty(t, rows[0].GroupName, "resolved name or the fallback label, never blank")
		return nil
	})
	require.NoError(t, err)
}

func TestSupervisionBlockersExcludeFutureSupervisions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)

	staff := testpkg.CreateTestStaff(t, db, "Future", "Blocker")
	activity := testpkg.CreateTestActivityGroup(t, db, "Future Blocker Activity")
	room := testpkg.CreateTestRoom(t, db, "Future Blocker Room")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	supervision := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
	_, err := db.NewUpdate().
		TableExpr("active.group_supervisors").
		Set("start_date = CURRENT_DATE + 1").
		Where("id = ?", supervision.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	factory := NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	err = testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		rows, err := factory.GroupSupervisor.ListActiveSupervisionBlockers(ctx, staff.ID)
		require.NoError(t, err)
		assert.Empty(t, rows)
		return nil
	})
	require.NoError(t, err)
}

func TestStudentRosterGroupNamesAreEmptyWithoutGroup(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.CreateTestStudent(t, db, "Ohne", "Gruppe", "1a")

	factory := NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))

	rows, err := factory.Student.FindAllWithGroups(testpkg.Ctx(t))
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, row := range rows {
		if row.Record.GroupID == nil {
			assert.Empty(t, row.GroupName)
		}
	}
}
