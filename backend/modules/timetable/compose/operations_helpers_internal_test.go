package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Overdue metadata is computed from the block's own date: a block right after
// midnight is ten minutes ahead, not a day overdue.
func TestTimetableOperationsPlannedNowOverdueMetadataUsesInstanceDate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 10, 23, 55, 0, 0, time.UTC)
	tomorrowStart := time.Date(2026, time.May, 11, 0, 5, 0, 0, time.UTC)
	inst := instanceWithTimes(335, scheduleModels.InstanceStatusPlanned, tomorrowStart, tomorrowStart.Add(time.Hour))
	deps := newTimetableOpsDeps()
	candidate := plannedNowCandidate{instance: inst, staffRows: []*scheduleModels.InstanceStaff{{StaffID: 225}}}

	result := deps.service.mapPlannedInstance(candidate, now, 225, nil)

	assert.False(t, result.IsOverdue)
	assert.Equal(t, 10, result.MinutesUntilStart)
}

// The planned-now card counts the same rows the roster groups: a status-day
// absence on an unbooked day belongs under "nicht eingeplant", or the card
// reports 0 while the slide-over shows one (#1747 review).
func TestTimetableOperationsPlannedCardCountsStatusDayNonBookings(t *testing.T) {
	t.Parallel()

	statusDayID := int64(9101)
	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	inst := instanceWithTimes(369, scheduleModels.InstanceStatusPlanned, now, now.Add(time.Hour))
	rows := []*scheduleModels.InstanceStudent{
		{StudentID: 540, Status: scheduleModels.AttendanceStatusAbsent, StudentStatusDayID: &statusDayID},
		{StudentID: 541, Status: scheduleModels.AttendanceStatusAbsent},
		{StudentID: 542, Status: scheduleModels.AttendanceStatusExpected},
		{StudentID: 543, Status: scheduleModels.AttendanceStatusExpected},
	}
	careDay := map[int64]timetable.CareDayStatus{
		540: timetable.CareDayNotScheduled,
		541: timetable.CareDayNotScheduled,
		542: timetable.CareDayNotScheduled,
		543: timetable.CareDayScheduled,
	}
	deps := newTimetableOpsDeps()
	candidate := plannedNowCandidate{instance: inst, staffRows: []*scheduleModels.InstanceStaff{{StaffID: 249}}, studentRows: rows}

	result := deps.service.mapPlannedInstance(candidate, now, 249, careDay)

	assert.Equal(t, 1, result.ExpectedStudentsCount)
	assert.Equal(t, 0, result.PresentStudentsCount)
	assert.Equal(t, 2, result.NotScheduledCount, "the status-day non-booking and the unbooked expected row")
}

func TestTimetableOperationHelpers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	assert.True(t, plannedNowWindow(instanceWithTimes(406, scheduleModels.InstanceStatusPlanned, now.Add(-16*time.Minute), now.Add(time.Hour)), now, 0))
	assert.True(t, plannedNowWindow(instanceWithTimes(407, scheduleModels.InstanceStatusPlanned, now.Add(14*time.Minute), now.Add(2*time.Hour)), now, 0))
	assert.False(t, plannedNowWindow(instanceWithTimes(408, scheduleModels.InstanceStatusPlanned, now.Add(16*time.Minute), now.Add(2*time.Hour)), now, 0))
	assert.True(t, plannedNowWindow(instanceWithTimes(409, scheduleModels.InstanceStatusPlanned, now.Add(90*time.Minute), now.Add(3*time.Hour)), now, 120))
	assert.False(t, plannedNowWindow(instanceWithTimes(410, scheduleModels.InstanceStatusPlanned, now.Add(-time.Hour), now), now, 0))
	spontaneous := instanceWithTimes(411, scheduleModels.InstanceStatusPlanned, now.Add(-time.Hour), now)
	spontaneous.IsSpontaneous = true
	assert.True(t, plannedNowWindow(spontaneous, now, 0))
	assert.True(t, staffAssigned([]*scheduleModels.InstanceStaff{{StaffID: 255}}, 255))
	assert.False(t, staffAssigned([]*scheduleModels.InstanceStaff{{StaffID: 255, IsAbsent: true}}, 255))
	planned, ok := findPlanned([]*scheduleModels.InstanceStudent{{StudentID: 556}}, 556)
	require.True(t, ok)
	assert.Equal(t, int64(556), planned.StudentID)
}

func TestTimetableOperationsDependencyAndErrorBranches(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "timetable attendance validation failed", (&timetable.AttendanceValidationError{}).Error())

	_, err := NewOperations(OperationDependencies{})
	require.Error(t, err, "a composition without its required ports must not build")

	deps := newTimetableOpsDeps()
	// A settings fault must fail closed for every caller shape (#2380).
	deps.settings.stringErr = errors.New("settings down")
	assert.False(t, deps.service.operationalOverview(context.Background(), true, true))
	assert.False(t, deps.service.operationalOverview(context.Background(), false, true))
	assert.False(t, deps.service.operationalOverview(context.Background(), false, false))

	deps.personService.accountPerson = &usersModels.Person{}
	deps.personService.accountPerson.ID = 505
	deps.personService.staffErr = helperNotFound{}
	staffID, hasStaff, err := deps.service.resolveStaffID(context.Background(), 683)
	require.NoError(t, err)
	assert.Zero(t, staffID)
	assert.False(t, hasStaff)

	deps.instanceRepo.err = helperNotFound{}
	inst, err := deps.service.loadInstance(context.Background(), 415)
	require.ErrorIs(t, err, timetable.ErrTimetableOperationNotFound)
	assert.Nil(t, inst)

	deps.instanceRepo.err = nil
	deps.instanceRepo.byID[416] = nil
	inst, err = deps.service.loadInstance(context.Background(), 416)
	require.ErrorIs(t, err, timetable.ErrTimetableOperationNotFound)
	assert.Nil(t, inst)
}

func TestTimetableOperationsLoadInstancePropagatesRepositoryErrors(t *testing.T) {
	t.Parallel()

	deps := newTimetableOpsDeps()
	deps.instanceRepo.err = errors.New("db down")

	inst, err := deps.service.loadInstance(context.Background(), 417)

	require.EqualError(t, err, "db down")
	assert.Nil(t, inst)
}

func TestTimetableOperationDirectHelperBranches(t *testing.T) {
	t.Parallel()

	t.Run("load roster template group ignores missing and unbound groups", func(t *testing.T) {
		svc := newTimetableOpsDeps().service

		group, err := svc.loadRosterTemplateGroup(context.Background(), nil)
		require.NoError(t, err)
		assert.Nil(t, group)

		zero := int64(0)
		group, err = svc.loadRosterTemplateGroup(context.Background(), &zero)
		require.NoError(t, err)
		assert.Nil(t, group)

		missing := int64(599)
		group, err = svc.loadRosterTemplateGroup(context.Background(), &missing)
		require.NoError(t, err)
		assert.Nil(t, group)
	})

	t.Run("load roster template group propagates ordinary errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.activityGroups.err = errors.New("group failed")
		groupID := int64(600)

		group, err := deps.service.loadRosterTemplateGroup(context.Background(), &groupID)

		require.EqualError(t, err, "group failed")
		assert.Nil(t, group)
	})

	// Skipping nil room rows is the root's room binding now
	// (database/repositories timetableRoomNames); the owner maps the names.
	t.Run("room name map names every room", func(t *testing.T) {
		names, err := newTimetableOpsDeps().service.roomNameMap(context.Background())

		require.NoError(t, err)
		require.NotNil(t, names[810])
		assert.Equal(t, "Lernraum", *names[810])
	})

	t.Run("logger falls back to default", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.service.deps.Logger = nil

		assert.NotNil(t, deps.service.logger())
	})
}

func TestTimetableOperationsBroadcastBranches(t *testing.T) {
	t.Parallel()

	ctx := tenant.WithTenantID(context.Background(), 722)

	t.Run("skips when broadcaster is nil", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.instanceRepo.byID[414] = activeInstance(414, 300)
		deps.service.deps.Announcer = nil

		deps.service.broadcastAttendanceChanged(ctx, 414)

		assert.Empty(t, deps.announcer.calls)
	})

	t.Run("skips inactive instance without active group", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		start := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
		deps.instanceRepo.byID[414] = instanceWithTimes(414, scheduleModels.InstanceStatusPlanned, start, start.Add(time.Hour))

		deps.service.broadcastAttendanceChanged(ctx, 414)

		assert.Empty(t, deps.announcer.calls)
	})

	t.Run("logs and continues when broadcast fails", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.announcer.err = errors.New("send failed")
		deps.instanceRepo.byID[414] = activeInstance(414, 300)

		deps.service.broadcastAttendanceChanged(ctx, 414)

		assert.Equal(t, []announcement{{tenantID: 722, activeGroupID: 300, instanceID: 414}}, deps.announcer.calls)
	})

	t.Run("logs and skips when instance lookup fails", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.instanceRepo.err = errors.New("instance failed")

		deps.service.broadcastAttendanceChanged(ctx, 414)

		assert.Empty(t, deps.announcer.calls)
	})
}
