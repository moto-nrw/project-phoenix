package schedule_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStaffAssignmentServiceListForStaff verifies the "Mein Tag" read (#1844):
// a staff member's own Betreuungsplan blocks come back enriched with room and
// activity-group names, the Vertretungsplan flags, ordered by start time, and
// scoped strictly to the calling staff member.
func TestStaffAssignmentServiceListForStaff(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	staff := testpkg.CreateTestStaff(t, db, "Anna", "Assign")
	other := testpkg.CreateTestStaff(t, db, "Ben", "Other")
	room := testpkg.CreateTestRoom(t, db, "Raum 12")
	group := testpkg.CreateTestActivityGroup(t, db, "Lernzeit")

	date := timezone.NewDate(2026, 3, 10)

	// A normal planned block with a template (group + room).
	lernzeit := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &group.ID,
		StartHHMM:       "10:00",
		EndHHMM:         "11:30",
		Title:           "Lernzeit",
	})
	// A later, cancelled block ("fällt aus") the staff member covers as substitute.
	ag := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00",
		EndHHMM:   "15:00",
		Title:     "Fußball-AG",
		Status:    scheduleModel.InstanceStatusCancelled,
	})

	isLernzeit := testpkg.CreateTestInstanceStaff(t, db, lernzeit.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
	isAG := testpkg.CreateTestInstanceStaff(t, db, ag.ID, staff.ID, testpkg.InstanceStaffOpts{IsSubstitute: true})
	// Another person on the same block — must never leak into Anna's list.
	isOther := testpkg.CreateTestInstanceStaff(t, db, lernzeit.ID, other.ID, testpkg.InstanceStaffOpts{})

	t.Cleanup(func() {
		testpkg.CleanupInstanceStaffFixtures(t, db, isLernzeit.ID, isAG.ID, isOther.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", lernzeit.ID, ag.ID)
		testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)
		testpkg.CleanupStaffFixtures(t, db, staff.ID, other.ID)
		testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)
	})

	repos := repositories.NewFactory(db)
	service := scheduleSvc.NewStaffAssignmentService(scheduleSvc.StaffAssignmentDependencies{
		InstanceStaffRepo:    repos.InstanceStaff,
		ActivityInstanceRepo: repos.ActivityInstance,
		RoomRepo:             repos.Room,
		ActivityGroupRepo:    repos.ActivityGroup,
	}, nil)
	ctx := testpkg.TenantContext(tenantID)

	got, err := service.ListAssignmentsForStaff(ctx, staff.ID, date, date)
	require.NoError(t, err)
	require.Len(t, got, 2, "Anna has two blocks; Ben's assignment must not leak in")

	// Ordered by start time: Lernzeit (10:00) before the AG (14:00).
	first := got[0]
	assert.Equal(t, "Lernzeit", first.Title)
	require.NotNil(t, first.GroupName)
	assert.Equal(t, group.Name, *first.GroupName)
	assert.Equal(t, room.Name, first.RoomName)
	assert.Equal(t, "10:00", first.StartTime.Format("15:04"))
	assert.Equal(t, "11:30", first.EndTime.Format("15:04"))
	assert.True(t, first.IsPrimary)
	assert.False(t, first.Cancelled)

	second := got[1]
	assert.Equal(t, "Fußball-AG", second.Title)
	assert.True(t, second.Cancelled, "cancelled instance surfaces as a cancelled block")
	assert.True(t, second.IsSubstitute)
	assert.Nil(t, second.GroupName, "spontaneous block has no activity group")

	// Self-scoping: Ben sees only his single (Lernzeit) block, never Anna's.
	bens, err := service.ListAssignmentsForStaff(ctx, other.ID, date, date)
	require.NoError(t, err)
	require.Len(t, bens, 1)
	assert.Equal(t, "Lernzeit", bens[0].Title)
}
