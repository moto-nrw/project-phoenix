package httpintegration_test

import (
	"errors"
	"testing"
	"time"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeDutyScenario turns the scenario template into a duty without a room
// and without children (#3822).
func makeDutyScenario(t *testing.T, materializeDate calendar.Date, requiredStaff *int) *scenarioSetup {
	t.Helper()
	s := makeScenario(t, activitiesModels.WeekdayMonday, materializeDate)
	_, err := s.db.NewDelete().TableExpr(`activities.student_enrollments`).
		Where("activity_group_id = ?", s.template.ID).Exec(s.ctx)
	require.NoError(t, err)
	_, err = s.db.NewUpdate().TableExpr(`activities.groups`).
		Set("type = ?", timetable.GroupTypeDuty).Set("planned_room_id = NULL").
		Set("max_participants = NULL").Set("required_staff = ?", requiredStaff).
		Where("id = ?", s.template.ID).Exec(s.ctx)
	require.NoError(t, err)
	return s
}

func TestDutyWithoutRoomMaterializesAndIsNeverStarted(t *testing.T) {
	t.Parallel()

	materializeDate := calendar.NewDate(2026, time.April, 20) // Mon
	s := makeDutyScenario(t, materializeDate, nil)
	defer s.runCleanup(t)

	result, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDays(6), timetable.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, 1, result.InstancesCreated, "a duty needs no room to be planned")
	assert.Zero(t, result.InstanceStudentsCreated)
	assert.Equal(t, 1, result.InstanceStaffCreated)

	rows := listInstancesForDate(t, s.db, s.template.ID, materializeDate)
	require.Len(t, rows, 1)
	assert.Zero(t, rows[0].RoomID, "room_id is stored as NULL")
	var roomIsNull bool
	require.NoError(t, s.db.NewRaw(`SELECT room_id IS NULL FROM schedule.activity_instances WHERE id = ?`, rows[0].ID).
		Scan(s.ctx, &roomIsNull))
	assert.True(t, roomIsNull)

	_, err = s.factory.Instance.Start(s.ctx, rows[0].ID, s.staffID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, timetable.ErrInvalidInstanceTransition), "got %v", err)
}

func TestDutyWithoutRoomCanBeMoved(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	_, err := s.db.NewUpdate().TableExpr(`activities.groups`).
		Set("type = ?", timetable.GroupTypeDuty).Set("planned_room_id = NULL").
		Where("id = ?", s.tmplID).Exec(s.ctx)
	require.NoError(t, err)
	instance := seedInstance(t, s, false, false)
	_, err = s.db.NewUpdate().TableExpr(`schedule.activity_instances`).
		Set("room_id = NULL").Where("id = ?", instance.ID).Exec(s.ctx)
	require.NoError(t, err)

	updated, err := s.svc.UpdatePlanned(s.ctx, instance.ID, timetable.UpdateInstanceInput{
		Date:            calendar.Date(instance.Date),
		StartTime:       instance.StartTime.Add(30 * time.Minute),
		EndTime:         instance.EndTime.Add(30 * time.Minute),
		Title:           "Verschobene Busaufsicht",
		RoomID:          0,
		ActivityGroupID: &s.tmplID,
	}, nil)
	require.NoError(t, err)
	assert.Zero(t, updated.RoomID)

	var roomIsNull bool
	require.NoError(t, s.db.NewRaw(`SELECT room_id IS NULL FROM schedule.activity_instances WHERE id = ?`, instance.ID).
		Scan(s.ctx, &roomIsNull))
	assert.True(t, roomIsNull)
}

func TestDutyAutoStartSkipsDuty(t *testing.T) {
	t.Parallel()

	materializeDate := calendar.NewDate(2026, time.April, 20) // Mon
	s := makeDutyScenario(t, materializeDate, nil)
	defer s.runCleanup(t)

	_, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate, timetable.MaterializationSourceManual)
	require.NoError(t, err)

	// Inside the 14:00–15:00 window of the scenario timeframe.
	now := time.Date(2026, time.April, 20, 14, 30, 0, 0, time.Local)
	result, err := s.factory.AutoStart.RunForTenant(s.ctx, now)
	require.NoError(t, err)
	assert.Equal(t, 1, result.SkippedDuty)
	assert.Zero(t, result.Started)
	assert.Zero(t, result.Failed)
}

func TestDutyIsAGapBelowRequiredStaff(t *testing.T) {
	t.Parallel()

	materializeDate := calendar.NewDate(2026, time.April, 20) // Mon
	required := 2
	s := makeDutyScenario(t, materializeDate, &required)
	defer s.runCleanup(t)

	_, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	rows := listInstancesForDate(t, s.db, s.template.ID, materializeDate)
	require.Len(t, rows, 1)

	gaps, err := s.factory.TimetableData.Data.ListUnderstaffedInstances(s.ctx, materializeDate, materializeDate)
	require.NoError(t, err)
	require.Len(t, gaps, 1, "one present person, two required")
	assert.Equal(t, rows[0].ID, gaps[0].Instance.ID)

	_, err = s.db.NewUpdate().TableExpr(`activities.groups`).Set("required_staff = 1").
		Where("id = ?", s.template.ID).Exec(s.ctx)
	require.NoError(t, err)
	gaps, err = s.factory.TimetableData.Data.ListUnderstaffedInstances(s.ctx, materializeDate, materializeDate)
	require.NoError(t, err)
	assert.Empty(t, gaps, "the floor is met")
}
