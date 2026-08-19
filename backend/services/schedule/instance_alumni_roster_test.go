// Manual roster writes must refuse graduated (alumnus) students (#405 review).
//
// Materialization already skips them, but Create and UpdatePlanned write
// instance_students directly from a caller-supplied id list. A planner form
// opened before the graduation and saved afterwards — or any direct request
// carrying the id — would recreate exactly the roster rows the transition's
// archive pass removed, and slot lists, staffing ratios and exports would count
// the departed child again.
//
// The gate stops where the archive pass stops (today or later). A past-dated
// roster is frozen history that deliberately keeps the graduate's row, so the
// last test pins that such a block stays editable.
package schedule_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// graduate flips a student fixture to the alumnus soft-delete state, exactly as
// a grade transition's apply does.
func graduate(t *testing.T, s *lifecycleSetup, studentID int64) {
	t.Helper()
	_, err := s.db.NewUpdate().
		TableExpr("users.students").
		Set("status = ?", string(usersModel.StudentStatusAlumnus)).
		Where("id = ?", studentID).
		Exec(s.ctx)
	require.NoError(t, err)
}

func TestInstance_Create_RejectsGraduatedStudent(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	graduate(t, s, s.student1)

	inst, err := s.svc.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date:            timezone.TodayDate().AddDays(7),
		StartTime:       time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		Title:           "Alumnus must not be planned",
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
		StaffIDs:        []int64{s.staffID},
		StudentIDs:      []int64{s.student2, s.student1},
	})

	require.Error(t, err)
	assert.Nil(t, inst)
	assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceReference)

	// Nothing was written — not even the still-active companion's row.
	assert.Equal(t, 0, countRowsWhere(t, s, "schedule.instance_students", "student_id", s.student2))
}

func TestInstance_UpdatePlanned_RejectsGraduatedStudent(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	date := timezone.TodayDate().AddDays(7)
	inst, err := s.svc.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date:            date,
		StartTime:       time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		Title:           "Planner block",
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
		StaffIDs:        []int64{s.staffID},
		StudentIDs:      []int64{s.student2},
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	// The child graduates while the planner form is open; the staffer then saves
	// the roster they loaded before the transition.
	graduate(t, s, s.student1)

	_, err = s.svc.UpdatePlanned(s.ctx, inst.ID, scheduleSvc.UpdateInstanceInput{
		Date:            date,
		StartTime:       inst.StartTime,
		EndTime:         inst.EndTime,
		Title:           inst.Title,
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
		StaffIDs:        []int64{s.staffID},
		StudentIDs:      []int64{s.student2, s.student1},
	}, nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceReference)

	// The pre-existing roster survives the refused edit, and the graduate never
	// got a row.
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_students", "student_id", s.student2))
	assert.Equal(t, 0, countRowsWhere(t, s, "schedule.instance_students", "student_id", s.student1))
}

// TestInstance_UpdatePlanned_PastDatedRosterKeepsGraduate pins the boundary: the
// graduation's archive pass only clears rows dated today or later, and the roster
// read keeps showing a graduate on a past-dated block. Rejecting that write would
// make the block uneditable forever without protecting anything, so a past-dated
// save may still carry the departed child.
func TestInstance_UpdatePlanned_PastDatedRosterKeepsGraduate(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	past := timezone.TodayDate().AddDays(-14)
	inst, err := s.svc.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date:            past,
		StartTime:       time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		Title:           "Frozen history block",
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
		StaffIDs:        []int64{s.staffID},
		StudentIDs:      []int64{s.student1},
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	graduate(t, s, s.student1)

	updated, err := s.svc.UpdatePlanned(s.ctx, inst.ID, scheduleSvc.UpdateInstanceInput{
		Date:            past,
		StartTime:       inst.StartTime,
		EndTime:         inst.EndTime,
		Title:           "Frozen history block (renamed)",
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
		StaffIDs:        []int64{s.staffID},
		StudentIDs:      []int64{s.student1},
	}, nil)

	require.NoError(t, err, "a past-dated block must stay editable after a graduation")
	assert.Equal(t, "Frozen history block (renamed)", updated.Title)
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_students", "student_id", s.student1))
}
