package presenceprojection

import (
	"context"
	"testing"
	"time"

	presenceContract "github.com/moto-nrw/project-phoenix/modules/presenceprojection/presencecontract"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// presenceRows is one planned participant in the test's tenant and one in
// another tenant on the same day.
type presenceRows struct {
	date                         calendar.Date
	own, foreign                 *testpkg.ActivityInstance
	ownStudent, foreignStudent   int64
	ownParticipant               *testpkg.InstanceStudent
	foreignParticipant           *testpkg.InstanceStudent
	instanceIDs, studentIDs      []int64
	ownCheckedIn, foreignChecked time.Time
}

func seedPresenceRows(t *testing.T, db *bun.DB) presenceRows {
	t.Helper()
	ctx := context.Background()
	rows := presenceRows{date: calendar.NewDate(2026, 9, 7)}
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	room := testpkg.CreateTestRoom(t, db, "Own room")
	foreignRoom := testpkg.CreateTestRoomForTenant(t, db, otherTenant, "Foreign room")
	rows.own = testpkg.CreateTestActivityInstance(t, db, rows.date, room.ID, testpkg.ActivityInstanceOpts{Title: "Own block"})
	rows.foreign = testpkg.CreateTestActivityInstanceForTenant(t, db, otherTenant, rows.date, foreignRoom.ID, testpkg.ActivityInstanceOpts{Title: "Foreign block"})
	rows.ownStudent = testpkg.CreateTestStudent(t, db, "Own", "Child", "2a").ID
	rows.foreignStudent = testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Foreign", "Child", "2a").ID
	rows.ownCheckedIn = time.Date(2026, 9, 7, 12, 5, 0, 0, time.UTC)
	rows.ownParticipant = testpkg.CreateTestInstanceStudent(t, db, rows.own.ID, rows.ownStudent, testpkg.ScheduleAttendanceStatusPresent,
		testpkg.InstanceStudentOpts{CheckedInAt: &rows.ownCheckedIn})
	rows.foreignChecked = rows.ownCheckedIn
	rows.foreignParticipant = &testpkg.InstanceStudent{InstanceID: rows.foreign.ID, StudentID: rows.foreignStudent,
		Status: testpkg.ScheduleAttendanceStatusPresent, CheckedInAt: &rows.foreignChecked}
	rows.foreignParticipant.TenantID = otherTenant
	testpkg.InsertInstanceStudentRow(t, ctx, db, rows.foreignParticipant)
	rows.instanceIDs = []int64{rows.own.ID, rows.foreign.ID}
	rows.studentIDs = []int64{rows.ownStudent, rows.foreignStudent}
	return rows
}

func TestListLegacyInstancesReturnsOnlyTheNamedTenantsBlocks(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	rows := seedPresenceRows(t, db)
	day := rows.date.String()

	instances, err := ListLegacyInstances(context.Background(), db, testpkg.Tenant(t), presenceContract.LegacyInstanceFilter{IDs: rows.instanceIDs, Date: &day})

	require.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, rows.own.ID, instances[0].ID)
	assert.Equal(t, testpkg.Tenant(t), instances[0].TenantID)
	assert.Equal(t, rows.date, instances[0].Date)
	assert.Equal(t, "Own block", instances[0].Title)
	assert.Equal(t, statusPlanned, instances[0].Status)
	assert.Equal(t, rows.own.RoomID, instances[0].RoomID)
}

func TestListLegacyParticipantsReturnsOnlyTheNamedTenantsAttendance(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	rows := seedPresenceRows(t, db)

	participants, err := ListLegacyParticipants(context.Background(), db, testpkg.Tenant(t), presenceContract.LegacyParticipantFilter{InstanceIDs: rows.instanceIDs, StudentIDs: rows.studentIDs})

	require.NoError(t, err)
	require.Len(t, participants, 1)
	participant := participants[0]
	assert.Equal(t, rows.ownParticipant.ID, participant.ID)
	assert.Equal(t, testpkg.Tenant(t), participant.TenantID)
	assert.Equal(t, rows.own.ID, participant.InstanceID)
	assert.Equal(t, rows.ownStudent, participant.StudentID)
	assert.Equal(t, testpkg.ScheduleAttendanceStatusPresent, participant.Status)
	require.NotNil(t, participant.CheckedInAt)
	assert.True(t, rows.ownCheckedIn.Equal(*participant.CheckedInAt))
}

func TestListScheduledInstancesForStudentReturnsOnlyTheNamedTenantsBlocks(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	rows := seedPresenceRows(t, db)
	ctx := context.Background()

	own, err := ListScheduledInstancesForStudent(ctx, db, testpkg.Tenant(t), rows.ownStudent, rows.date, rows.date)
	require.NoError(t, err)
	require.Len(t, own, 1)
	assert.Equal(t, rows.own.ID, own[0].Instance.ID)
	assert.Equal(t, testpkg.Tenant(t), own[0].Instance.TenantID)
	assert.Equal(t, "Own block", own[0].Instance.Title)
	assert.Equal(t, rows.ownParticipant.ID, own[0].Participant.ID)
	assert.Equal(t, testpkg.ScheduleAttendanceStatusPresent, own[0].Participant.Status)

	foreign, err := ListScheduledInstancesForStudent(ctx, db, testpkg.Tenant(t), rows.foreignStudent, rows.date, rows.date)
	require.NoError(t, err)
	assert.Empty(t, foreign, "another tenant's child must not appear under this tenant")
}
