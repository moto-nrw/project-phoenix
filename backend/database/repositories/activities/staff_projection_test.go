package activities_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The supervisor reads no longer join users.staff; the composition layer
// attaches the staff member through School Membership (#2667).
func TestPlannedActivitySupervisorsCarryTheirStaffMember(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	factory := repositories.NewFactory(db)

	staff := testpkg.CreateTestStaff(t, db, "Geplante", "Betreuung")
	activity := testpkg.CreateTestActivityGroup(t, db, "Geplantes Angebot")
	supervisor := &activitiesModels.SupervisorPlanned{
		StaffID:   staff.ID,
		GroupID:   activity.ID,
		IsPrimary: true,
		ValidFrom: timezone.TodayDate(),
	}
	require.NoError(t, factory.ActivitySupervisor.Create(ctx, supervisor))

	rows, err := factory.ActivitySupervisor.FindByGroupID(ctx, activity.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Staff)
	assert.Equal(t, staff.ID, rows[0].Staff.ID)
	assert.Equal(t, staff.PersonID, rows[0].Staff.PersonID)

	bulk, err := factory.ActivitySupervisor.FindByGroupIDs(ctx, []int64{activity.ID})
	require.NoError(t, err)
	require.Len(t, bulk, 1)
	require.NotNil(t, bulk[0].Staff)

	_, supervisors, err := factory.ActivityGroup.FindWithSupervisors(ctx, activity.ID)
	require.NoError(t, err)
	require.Len(t, supervisors, 1)
	require.NotNil(t, supervisors[0].Staff)
	assert.Equal(t, staff.PersonID, supervisors[0].Staff.PersonID)
}
