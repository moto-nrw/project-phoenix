package schedule

import (
	"context"
	"testing"
	"time"

	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The roster tells the caller whether it may act on the block (#3167). With
// the all_staff overview scope every staff member sees every running roster,
// but only the people requireCanOperate admits may act on it.
func TestTimetableOperationsRosterReportsCanOperate(t *testing.T) {
	t.Parallel()

	instanceID := int64(3167)
	activeGroupID := int64(3168)

	for _, tc := range []struct {
		name  string
		setup func(deps *timetableOpsTestDeps)
		admin bool
		want  bool
	}{
		{
			name: "planned staff",
			setup: func(deps *timetableOpsTestDeps) {
				wireAssignedStaff(deps, 701, 801, 901, instanceID)
			},
			want: true,
		},
		{
			name: "unplanned staff with all_staff overview",
			setup: func(deps *timetableOpsTestDeps) {
				deps.settings.scope = configModel.OverviewScopeAllStaff
				wireAssignedStaff(deps, 702, 802, 902, instanceID)
				deps.staffRepo.byInstance[instanceID] = []*scheduleModel.InstanceStaff{{StaffID: 999}}
			},
			want: false,
		},
		{
			name: "planned staff marked absent",
			setup: func(deps *timetableOpsTestDeps) {
				deps.settings.scope = configModel.OverviewScopeAllStaff
				wireAssignedStaff(deps, 703, 803, 903, instanceID)
				deps.staffRepo.byInstance[instanceID] = []*scheduleModel.InstanceStaff{{StaffID: 903, IsAbsent: true}}
			},
			want: false,
		},
		{
			name: "supervisor of the running session",
			setup: func(deps *timetableOpsTestDeps) {
				deps.settings.scope = configModel.OverviewScopeAllStaff
				wireAssignedStaff(deps, 704, 804, 904, instanceID)
				deps.staffRepo.byInstance[instanceID] = nil
				deps.supervisors.byActiveGroup[activeGroupID] = []*activeModel.GroupSupervisor{{StaffID: 904}}
			},
			want: true,
		},
		{
			name:  "admin",
			setup: func(*timetableOpsTestDeps) {},
			admin: true,
			want:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deps := newTimetableOpsDeps()
			tc.setup(deps)
			deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)

			roster, err := deps.service.Roster(context.Background(), 700, tc.admin, instanceID)

			require.NoError(t, err)
			require.NotNil(t, roster)
			assert.Equal(t, tc.want, roster.CanOperate)
		})
	}
}

// An admin may act on every block, but a check-in needs a staff member to
// record. The denial names that reason, so the client does not tell the
// admin they are "not planned".
func TestTimetableOperationsCheckInWithoutStaffProfileNamesTheReason(t *testing.T) {
	t.Parallel()

	instanceID := int64(3173)
	deps := newTimetableOpsDeps()
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 3174)

	_, err := deps.service.CheckInStudent(context.Background(), 712, true, instanceID, 3175)

	require.ErrorIs(t, err, ErrTimetableOperationForbidden)
	assert.ErrorContains(t, err, "no staff profile")
}

// A write response replaces the cached roster in the client, so it must not
// drop the action rights the caller just used.
func TestTimetableOperationsWriteResponsesReportCanOperate(t *testing.T) {
	t.Parallel()

	instanceID := int64(3170)
	activeGroupID := int64(3171)
	studentID := int64(3172)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 711, 811, 911, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.visitRepo.byActiveGroup[activeGroupID] = []*studentpresence.Visit{{StudentID: studentID, ActiveGroupID: activeGroupID, EntryTime: time.Now()}}
	deps.activeService.endErr = activeSvc.ErrVisitAlreadyEnded

	roster, err := deps.service.CheckOutStudent(context.Background(), 711, false, instanceID, studentID)

	require.NoError(t, err)
	require.NotNil(t, roster)
	assert.True(t, roster.CanOperate)
}
