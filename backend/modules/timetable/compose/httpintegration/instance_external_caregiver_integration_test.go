package httpintegration_test

import (
	"testing"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstance_Create_RejectsExternalCaregiver(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	external := testpkg.CreateTestGuest(t, s.db, "Musik")

	created, err := s.svc.CreateInstance(s.ctx, timetable.CreateInstanceInput{
		Date:            calendar.TodayDate().AddDays(7),
		StartTime:       time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		Title:           "Geplante Musik-AG",
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
		StaffIDs:        []int64{external.StaffID},
	})

	require.Error(t, err)
	assert.Nil(t, created)
	assert.ErrorIs(t, err, timetable.ErrInvalidInstanceReference)
	assert.Zero(t, countRowsWhere(t, s, "schedule.activity_instances", "room_id", s.roomID))
}

func TestInstance_CreateSpontaneous_AllowsExternalCaregiver(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	external := testpkg.CreateTestGuest(t, s.db, "Musik")
	spontaneous := true

	created, err := s.svc.CreateInstance(s.ctx, timetable.CreateInstanceInput{
		Date:          calendar.TodayDate(),
		StartTime:     time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:       time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		Title:         "Spontane Musik-AG",
		RoomID:        s.roomID,
		IsSpontaneous: &spontaneous,
		StaffIDs:      []int64{external.StaffID},
	})

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.True(t, created.IsSpontaneous)
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_staff", "instance_id", created.ID))
}

func TestInstance_UpdateSpontaneous_PreservesExternalCaregiver(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	instance := seedSpontaneousInstance(t, s, false)
	external := testpkg.CreateTestGuest(t, s.db, "Musik")
	staff := &scheduleModels.InstanceStaff{InstanceID: instance.ID, StaffID: external.StaffID}
	staff.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(staff).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
	require.NoError(t, err)

	updated, err := s.svc.UpdatePlanned(s.ctx, instance.ID, timetable.UpdateInstanceInput{
		Date:      calendar.Date(instance.Date),
		StartTime: instance.StartTime,
		EndTime:   instance.EndTime,
		Title:     "Spontane Musik-AG geändert",
		RoomID:    s.roomID,
		StaffIDs:  []int64{external.StaffID},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.True(t, updated.IsSpontaneous)
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_staff", "instance_id", instance.ID))
}

func TestInstance_UpdateSpontaneousToPlanned_RejectsExternalCaregiver(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	instance := seedSpontaneousInstance(t, s, false)
	external := testpkg.CreateTestGuest(t, s.db, "Musik")
	staff := &scheduleModels.InstanceStaff{InstanceID: instance.ID, StaffID: external.StaffID}
	staff.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(staff).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.svc.UpdatePlanned(s.ctx, instance.ID, timetable.UpdateInstanceInput{
		Date:             calendar.Date(instance.Date),
		StartTime:        instance.StartTime,
		EndTime:          instance.EndTime,
		Title:            "Als geplant fortsetzen",
		RoomID:           s.roomID,
		ActivityGroupID:  &s.tmplID,
		CalendarPeriodID: &s.period.ID,
		StaffIDs:         []int64{external.StaffID},
	}, nil)

	require.ErrorIs(t, err, timetable.ErrInvalidInstanceReference)
}

func TestInstance_CreatePlanned_AllowsAccountBackedGuest(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	staff, _ := testpkg.CreateTestStaffWithAccount(t, s.db, "Gast", "MitKonto")
	guest := &usersModels.Guest{StaffID: staff.ID, ActivityExpertise: "Musik"}
	guest.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(guest).ModelTableExpr(`users.guests`).Exec(s.ctx)
	require.NoError(t, err)

	created, err := s.svc.CreateInstance(s.ctx, timetable.CreateInstanceInput{
		Date:            calendar.TodayDate().AddDays(7),
		StartTime:       time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		Title:           "Geplante Musik-AG",
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
		StaffIDs:        []int64{staff.ID},
	})

	require.NoError(t, err)
	assert.NotNil(t, created)
}
