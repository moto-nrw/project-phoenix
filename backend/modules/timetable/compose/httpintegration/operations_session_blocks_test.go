package httpintegration_test

import (
	"context"
	"testing"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sessionBlocksFixture runs four blocks on one day: the caller is planned on
// the first, supervises the second, is planned but absent on the third, and a
// fourth runs in a session nobody asked about. A fifth block is still planned.
func sessionBlocksFixture(t *testing.T) (*timetableOpsTestDeps, calendar.Date, map[int64][]int64) {
	t.Helper()

	const accountID, personID, callerStaffID = int64(3281), int64(3282), int64(3283)
	const colleagueStaffID = int64(3284)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, accountID, personID, callerStaffID, 41)
	day := calendar.NewDate(2026, time.September, 16)
	deps.now = func() time.Time {
		return day.BerlinMidnight().Add(12 * time.Hour)
	}

	block := func(id, activeGroupID int64, start, end int, status string) *scheduleModels.ActivityInstance {
		inst := instanceWithTimes(id, status,
			time.Date(1, time.January, 1, start, 0, 0, 0, time.UTC),
			time.Date(1, time.January, 1, end, 0, 0, 0, time.UTC))
		inst.Date = scheduleModels.Date(day.String())
		inst.Title = "GT"
		if activeGroupID > 0 {
			inst.ActiveGroupID = &activeGroupID
		}
		return inst
	}
	deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
		block(41, 51, 13, 14, scheduleModels.InstanceStatusActive),
		block(42, 52, 13, 15, scheduleModels.InstanceStatusActive),
		block(43, 53, 14, 15, scheduleModels.InstanceStatusActive),
		block(44, 54, 14, 16, scheduleModels.InstanceStatusActive),
		block(45, 0, 16, 17, scheduleModels.InstanceStatusPlanned),
	}
	deps.staffRepo.byInstance[41] = []*scheduleModels.InstanceStaff{{StaffID: callerStaffID}, {StaffID: colleagueStaffID}}
	deps.staffRepo.byInstance[42] = []*scheduleModels.InstanceStaff{{StaffID: colleagueStaffID}}
	deps.staffRepo.byInstance[43] = []*scheduleModels.InstanceStaff{{StaffID: colleagueStaffID}, {StaffID: callerStaffID, IsAbsent: true}}
	deps.staffRepo.byInstance[44] = []*scheduleModels.InstanceStaff{{StaffID: callerStaffID}}

	supervisors := map[int64][]int64{
		51: {colleagueStaffID},
		52: {callerStaffID, colleagueStaffID},
		53: {colleagueStaffID},
		// A session without a timetable block behind it (a kiosk session).
		60: {callerStaffID},
	}
	for _, inst := range deps.instanceRepo.byDate {
		deps.instanceRepo.byID[inst.ID] = inst
	}
	for sessionID, staffIDs := range supervisors {
		for _, staffID := range staffIDs {
			deps.supervisors.byActiveGroup[sessionID] = append(deps.supervisors.byActiveGroup[sessionID], &studentpresence.StaffedSupervision{GroupSupervision: studentpresence.GroupSupervision{StaffID: staffID}})
		}
	}
	return deps, day, supervisors
}

func blocksBySession(blocks []timetable.OperationSessionBlock) map[int64]timetable.OperationSessionBlock {
	result := make(map[int64]timetable.OperationSessionBlock, len(blocks))
	for _, block := range blocks {
		result[block.ActiveGroupID] = block
	}
	return result
}

// SessionBlocks answers in bulk what requireCanOperate answers one block at a
// time (#3281): planned staff and supervisors operate a running block, an
// absent plan entry does not, and an admin operates every block.
func TestTimetableOperationsSessionBlocksMirrorCanOperate(t *testing.T) {
	t.Parallel()

	deps, day, supervisors := sessionBlocksFixture(t)

	blocks, err := deps.service.SessionBlocks(context.Background(), 3281, false, day, supervisors)

	require.NoError(t, err)
	bySession := blocksBySession(blocks)
	require.Len(t, bySession, 3, "only running blocks behind the requested sessions")
	assert.Equal(t, timetable.OperationSessionBlock{
		ActiveGroupID: 51, InstanceID: 41, Title: "GT", StartTime: "13:00", EndTime: "14:00",
		IsAssigned: true, CanOperate: true,
	}, bySession[51])
	assert.Equal(t, timetable.OperationSessionBlock{
		ActiveGroupID: 52, InstanceID: 42, Title: "GT", StartTime: "13:00", EndTime: "15:00",
		IsAssigned: false, CanOperate: true,
	}, bySession[52], "a supervisor operates a block without being planned on it")
	assert.Equal(t, timetable.OperationSessionBlock{
		ActiveGroupID: 53, InstanceID: 43, Title: "GT", StartTime: "14:00", EndTime: "15:00",
		IsAssigned: false, CanOperate: false,
	}, bySession[53], "an absent plan entry grants nothing")

	// The bulk verdict never drifts from the single-block check. A roster
	// reports that check as CanOperate; the all-staff overview lets the
	// caller read every running roster without granting action rights.
	deps.settings.scope = overviewScopeAllStaff
	for _, block := range blocks {
		single, err := deps.service.Roster(context.Background(), 3281, false, block.InstanceID)
		require.NoError(t, err)
		assert.Equal(t, single.CanOperate, block.CanOperate, "block %d", block.InstanceID)
	}

	t.Run("an admin operates every block", func(t *testing.T) {
		t.Parallel()
		deps, day, supervisors := sessionBlocksFixture(t)
		blocks, err := deps.service.SessionBlocks(context.Background(), 3281, true, day, supervisors)
		require.NoError(t, err)
		for _, block := range blocks {
			assert.True(t, block.CanOperate, "block %d", block.InstanceID)
		}
		assert.False(t, blocksBySession(blocks)[52].IsAssigned, "admin rights are no plan entry")
	})

	t.Run("an account without staff profile operates nothing", func(t *testing.T) {
		t.Parallel()
		deps, day, supervisors := sessionBlocksFixture(t)
		deps.personService.accountPerson = nil
		blocks, err := deps.service.SessionBlocks(context.Background(), 3281, false, day, supervisors)
		require.NoError(t, err)
		require.Len(t, blocks, 3)
		for _, block := range blocks {
			assert.False(t, block.CanOperate, "block %d", block.InstanceID)
			assert.False(t, block.IsAssigned, "block %d", block.InstanceID)
		}
	})

	t.Run("the school portal counts the plan entry of today only", func(t *testing.T) {
		t.Parallel()
		deps, day, supervisors := sessionBlocksFixture(t)
		ctx := testpkg.IdentityContext(tenant.WithTenantID(context.Background(), 3285), 3281, 3285, "school", []string{"schedules:read"})
		blocks, err := deps.service.SessionBlocks(ctx, 3281, true, day, supervisors)
		require.NoError(t, err)
		bySession := blocksBySession(blocks)
		assert.True(t, bySession[51].CanOperate)
		assert.False(t, bySession[52].CanOperate, "the school portal ignores supervisions and admin flags")

		deps.now = func() time.Time {
			return day.AddDays(1).BerlinMidnight().Add(8 * time.Hour)
		}
		blocks, err = deps.service.SessionBlocks(ctx, 3281, true, day, supervisors)
		require.NoError(t, err)
		assert.False(t, blocksBySession(blocks)[51].CanOperate, "yesterday's plan entry grants nothing")
	})

	t.Run("no sessions answer an empty list", func(t *testing.T) {
		t.Parallel()
		deps, day, _ := sessionBlocksFixture(t)
		deps.instanceRepo.byDate = nil
		blocks, err := deps.service.SessionBlocks(context.Background(), 3281, false, day, nil)
		require.NoError(t, err)
		assert.Empty(t, blocks)
		assert.NotNil(t, blocks)
	})
}
