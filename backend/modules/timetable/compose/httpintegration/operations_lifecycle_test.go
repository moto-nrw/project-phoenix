package httpintegration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimetableOperationsStartRequiresAStaffIdentity(t *testing.T) {
	t.Parallel()

	deps := newTimetableOpsDeps()
	deps.settings.scope = overviewScopeAdmins
	deps.personService.accountPerson = &usersModels.Person{}
	deps.personService.accountPerson.ID = 430

	result, err := deps.service.Start(context.Background(), 630, true, 340)

	require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
	assert.Nil(t, result)
	assert.Empty(t, deps.instanceService.started)
}

func TestTimetableOperationsStartDelegatesWhenStaffIsAssigned(t *testing.T) {
	t.Parallel()

	staffID := int64(230)
	instanceID := int64(350)
	deps := newTimetableOpsDeps()
	deps.personService.accountPerson = &usersModels.Person{}
	deps.personService.accountPerson.ID = 440
	deps.personService.staffByPersonID[440] = &usersModels.Staff{}
	deps.personService.staffByPersonID[440].ID = staffID
	deps.instanceRepo.byID[instanceID] = instanceWithTimes(instanceID, scheduleModels.InstanceStatusPlanned, opsNow, opsNow.Add(time.Hour))
	deps.staffRepo.byInstance[instanceID] = []*scheduleModels.InstanceStaff{{StaffID: staffID}}

	result, err := deps.service.Start(context.Background(), 640, false, instanceID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, instanceID, deps.instanceService.started[0].instanceID)
	assert.Equal(t, staffID, deps.instanceService.started[0].staffID)
}

func TestTimetableOperationsReopenRequiresCompleterOrAdmin(t *testing.T) {
	t.Parallel()

	instanceID := int64(428)
	deps := newTimetableOpsDeps()
	completed := activeInstance(instanceID, 306)
	completed.Status = scheduleModels.InstanceStatusCompleted
	deps.instanceRepo.byID[instanceID] = completed

	result, err := deps.service.Reopen(context.Background(), 694, false, instanceID)
	require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
	assert.Nil(t, result)

	wireAssignedStaff(deps, 694, 516, 275, instanceID)
	result, err = deps.service.Reopen(context.Background(), 694, false, instanceID)
	require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
	assert.Nil(t, result)

	completedBy := int64(694)
	completed.CompletedBy = &completedBy
	deps.staffRepo.byInstance[instanceID] = nil
	result, err = deps.service.Reopen(context.Background(), 694, false, instanceID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, instanceID, result.InstanceID)

	other := int64(701)
	completed.CompletedBy = &other
	result, err = deps.service.Reopen(context.Background(), 694, true, instanceID)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestTimetableOperationsCompleteDelegatesAfterPermissionCheck(t *testing.T) {
	t.Parallel()

	instanceID := int64(405)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 673, 495, 254, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 294)

	result, err := deps.service.Complete(context.Background(), 673, false, instanceID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []int64{instanceID}, deps.instanceService.completed)
}

func TestTimetableOperationsPermissionBranches(t *testing.T) {
	t.Parallel()

	instanceID := int64(409)
	activeGroupID := int64(296)

	t.Run("assigned active supervisor can operate without instance staff assignment", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 675, 496, 256, instanceID)
		deps.staffRepo.byInstance[instanceID] = nil
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		deps.supervisors.byActiveGroup[activeGroupID] = []*studentpresence.StaffedSupervision{{GroupSupervision: studentpresence.GroupSupervision{StaffID: 256}}}

		_, err := deps.service.Complete(context.Background(), 675, false, instanceID)

		require.NoError(t, err)
		assert.Equal(t, []int64{instanceID}, deps.instanceService.completed)
	})

	t.Run("unassigned staff is forbidden", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.settings.mode = groupModeFixedGroups
		wireAssignedStaff(deps, 676, 497, 257, instanceID)
		deps.staffRepo.byInstance[instanceID] = []*scheduleModels.InstanceStaff{{StaffID: 258}}
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)

		_, err := deps.service.Roster(context.Background(), 676, false, instanceID)

		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
	})

	t.Run("all_staff scope grants visibility but no action rights", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.settings.scope = overviewScopeAllStaff
		wireAssignedStaff(deps, 693, 515, 274, instanceID)
		deps.staffRepo.byInstance[instanceID] = nil
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)

		_, err := deps.service.Roster(context.Background(), 693, false, instanceID)
		require.NoError(t, err)

		_, err = deps.service.Complete(context.Background(), 693, false, instanceID)
		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
		assert.Empty(t, deps.instanceService.completed)
	})

	t.Run("all_staff scope rejects non-running rosters", func(t *testing.T) {
		now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
		for _, tc := range []struct {
			name     string
			instance *scheduleModels.ActivityInstance
		}{
			{
				name:     "planned",
				instance: instanceWithTimes(instanceID, scheduleModels.InstanceStatusPlanned, now, now.Add(time.Hour)),
			},
			{
				name:     "completed",
				instance: instanceWithTimes(instanceID, scheduleModels.InstanceStatusCompleted, now.Add(-time.Hour), now),
			},
			{
				name:     "cancelled",
				instance: instanceWithTimes(instanceID, scheduleModels.InstanceStatusCancelled, now, now.Add(time.Hour)),
			},
			{
				name:     "active without active group",
				instance: instanceWithTimes(instanceID, scheduleModels.InstanceStatusActive, now, now.Add(time.Hour)),
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				deps := newTimetableOpsDeps()
				deps.settings.scope = overviewScopeAllStaff
				wireAssignedStaff(deps, 694, 516, 275, instanceID)
				deps.staffRepo.byInstance[instanceID] = nil
				deps.instanceRepo.byID[instanceID] = tc.instance

				_, err := deps.service.Roster(context.Background(), 694, false, instanceID)

				require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
			})
		}
	})

	// The organisational group mode no longer opens running modules (#2380):
	// on its own it leaves the school on the restrictive default.
	t.Run("open care alone does not open foreign modules", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.settings.mode = groupModeOpenCare
		wireAssignedStaff(deps, 698, 518, 277, instanceID)
		deps.staffRepo.byInstance[instanceID] = []*scheduleModels.InstanceStaff{{StaffID: 279}}
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)

		_, err := deps.service.Roster(context.Background(), 698, false, instanceID)

		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
	})

	t.Run("overview scope resolution failure keeps fixed-group checks", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.settings.stringErr = errors.New("settings unavailable")
		wireAssignedStaff(deps, 694, 516, 275, instanceID)
		deps.staffRepo.byInstance[instanceID] = nil
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)

		_, err := deps.service.Complete(context.Background(), 694, false, instanceID)

		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
		assert.Empty(t, deps.instanceService.completed)
	})

	// NewOperations refuses a composition without settings, so "missing
	// settings" is the tenant without a configured overview scope.
	t.Run("unset overview scope keeps fixed-group checks", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 696, 517, 276, instanceID)
		deps.staffRepo.byInstance[instanceID] = nil
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		deps.settings.scope = ""

		_, err := deps.service.Complete(context.Background(), 696, false, instanceID)

		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
		assert.Empty(t, deps.instanceService.completed)
	})

	t.Run("all_staff scope still requires a staff identity", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.settings.scope = overviewScopeAllStaff

		_, err := deps.service.Roster(context.Background(), 695, false, instanceID)

		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
	})

	t.Run("admin can operate regardless of overview scope", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.settings.scope = overviewScopeOwn

		_, err := deps.service.Complete(context.Background(), 697, true, instanceID)

		require.NoError(t, err)
		assert.Equal(t, []int64{instanceID}, deps.instanceService.completed)
	})

	t.Run("missing account id is forbidden before repository lookup", func(t *testing.T) {
		deps := newTimetableOpsDeps()

		_, err := deps.service.Roster(context.Background(), 0, false, instanceID)

		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
	})

	t.Run("missing person cannot use staff-only operations", func(t *testing.T) {
		deps := newTimetableOpsDeps()

		_, err := deps.service.Start(context.Background(), 677, false, instanceID)

		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
	})
}
