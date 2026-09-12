package schedule

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"

	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type timetableAttendanceActor struct {
	ID       int
	TenantID int64
	Scope    string
}

func TestTimetableAbsenceWithoutBlockAssignment(t *testing.T) {
	t.Parallel()
	for _, reportScope := range []string{"admins", "all_staff"} {
		for _, substatus := range []string{"sick", "excused", "field_trip"} {
			for _, retract := range []bool{false, true} {
				t.Run(reportScope+"/"+substatus+"/"+fmt.Sprint(retract), func(t *testing.T) {
					t.Parallel()
					deps := newTimetableOpsDeps()
					const accountID, instanceID, studentID = int64(3421), int64(3422), int64(3423)
					wireAssignedStaff(deps, accountID, 3424, 3425, instanceID)
					deps.staffRepo.byInstance[instanceID] = nil
					deps.settings.scope = configModel.OverviewScopeAllStaff
					deps.settings.attendanceScope = configModel.AttendanceEditScopeOwn
					deps.settings.absenceScope = reportScope
					deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 3426)
					row := &scheduleModel.InstanceStudent{InstanceID: instanceID, StudentID: studentID, Status: "expected"}
					row.ID = 3427
					deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = row
					deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{row}
					deps.students.byID[studentID] = &usersModel.Student{PersonID: 3428, SchoolClass: "1a"}
					deps.personService.people[3428] = &usersModel.Person{FirstName: "Test", LastName: "Kind"}
					ctx := testpkg.IdentityContext(tenant.WithTenantID(context.Background(), 3429), accountID, 3429, "", []string{"schedules:read"})
					roster, err := deps.service.Roster(ctx, accountID, false, instanceID)
					require.NoError(t, err)
					require.False(t, roster.CanOperate)
					require.False(t, roster.CanEditAttendance)
					require.Equal(t, reportScope == "all_staff", roster.CanReportAbsence)
					status := "absent"
					patch := scheduleModel.AttendanceFieldPatch{Status: &status, Substatus: &substatus}
					if retract {
						row.Status, row.Substatus = "absent", &substatus
						status = "expected"
						patch = scheduleModel.AttendanceFieldPatch{Status: &status, SubstatusClear: true}
					}
					_, err = deps.service.PatchAttendance(ctx, accountID, false, instanceID, studentID, patch)
					if reportScope == "all_staff" && substatus != "field_trip" {
						require.NoError(t, err)
						require.Len(t, deps.studentRepo.updates, 1)
					} else {
						require.ErrorIs(t, err, ErrTimetableOperationForbidden)
						require.Empty(t, deps.studentRepo.updates)
					}
					_, err = deps.service.Complete(ctx, accountID, false, instanceID)
					require.ErrorIs(t, err, ErrTimetableOperationForbidden)
				})
			}
		}
	}
}

func TestTimetableBlockAbsenceRespectsDirectReportScope(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"admins", "all_staff", "unknown"} {
		for _, substatus := range []string{"sick", "excused", "field_trip"} {
			for _, retract := range []bool{false, true} {
				t.Run(scope+"/"+substatus+"/"+fmt.Sprint(retract), func(t *testing.T) {
					t.Parallel()
					deps := newTimetableOpsDeps()
					const accountID, instanceID, studentID = int64(3401), int64(3402), int64(3403)
					wireAssignedStaff(deps, accountID, 3404, 3405, instanceID)
					deps.settings.absenceScope = scope
					deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 3406)
					row := &scheduleModel.InstanceStudent{InstanceID: instanceID, StudentID: studentID, Status: "expected"}
					row.ID = 3407
					deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = row
					deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{row}
					deps.students.byID[studentID] = &usersModel.Student{PersonID: 3408, SchoolClass: "1a"}
					deps.personService.people[3408] = &usersModel.Person{FirstName: "Test", LastName: "Kind"}
					status := "absent"
					patch := scheduleModel.AttendanceFieldPatch{Status: &status, Substatus: &substatus}
					if retract {
						row.Status, row.Substatus = "absent", &substatus
						status = "expected"
						patch = scheduleModel.AttendanceFieldPatch{Status: &status, SubstatusClear: true}
					}
					_, err := deps.service.PatchAttendance(context.Background(), accountID, false, instanceID, studentID, patch)
					if scope == "all_staff" || substatus == "field_trip" {
						require.NoError(t, err)
						require.Len(t, deps.studentRepo.updates, 1)
					} else {
						require.ErrorIs(t, err, ErrTimetableOperationForbidden)
						require.Empty(t, deps.studentRepo.updates)
					}
					roster, err := deps.service.Roster(context.Background(), accountID, false, instanceID)
					require.NoError(t, err)
					require.True(t, roster.CanOperate)
					require.True(t, roster.CanEditAttendance, "absence scope must not restrict attendance")
					require.Equal(t, scope == "all_staff", roster.CanReportAbsence)
				})
			}
		}
	}
}

func TestTimetableSchoolWideAttendanceDoesNotGrantLifecycleRights(t *testing.T) {
	t.Parallel()
	deps := newTimetableOpsDeps()
	const accountID, instanceID, groupID, studentID = int64(3181), int64(3182), int64(3183), int64(3184)
	wireAssignedStaff(deps, accountID, 3185, 3186, instanceID)
	deps.staffRepo.byInstance[instanceID] = nil
	deps.settings.scope = configModel.OverviewScopeAllStaff
	deps.settings.attendanceScope = configModel.AttendanceEditScopeAllStaff
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, groupID)
	ctx := tenant.WithTenantID(context.Background(), 3187)
	ctx = testpkg.IdentityContext(ctx, accountID, 3187, "", []string{"schedules:read"})

	roster, err := deps.service.CheckInStudent(ctx, accountID, false, instanceID, studentID)
	require.NoError(t, err)
	require.Len(t, deps.activeService.created, 1)
	assert.False(t, roster.CanOperate, "attendance editing must not imply lifecycle access")
	assert.True(t, roster.CanEditAttendance)
	roster, err = deps.service.Roster(ctx, accountID, false, instanceID)
	require.NoError(t, err)
	assert.False(t, roster.CanOperate)
	assert.True(t, roster.CanEditAttendance)
	_, err = deps.service.Start(ctx, accountID, false, instanceID)
	require.ErrorIs(t, err, ErrTimetableOperationForbidden)
	_, err = deps.service.Complete(ctx, accountID, false, instanceID)
	require.ErrorIs(t, err, ErrTimetableOperationForbidden)
	assert.Empty(t, deps.supervisors.byActiveGroup[groupID])
	assert.Empty(t, deps.staffRepo.byInstance[instanceID])

	// A settings change is effective on the next call with the same identity.
	deps.settings.attendanceScope = configModel.AttendanceEditScopeOwn
	_, err = deps.service.CheckInStudent(ctx, accountID, false, instanceID, studentID)
	require.ErrorIs(t, err, ErrTimetableOperationForbidden)
	require.Len(t, deps.activeService.created, 1)
}

func TestTimetableSchoolWideAttendancePreservesAccessBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		change  func(*timetableOpsTestDeps, *timetableAttendanceActor, *[]string)
		allowed bool
	}{
		{name: "verified OGS staff", allowed: true},
		{name: "own scope without assignment", change: func(d *timetableOpsTestDeps, _ *timetableAttendanceActor, _ *[]string) {
			d.settings.attendanceScope = configModel.AttendanceEditScopeOwn
		}},
		{name: "personal visibility", change: func(d *timetableOpsTestDeps, _ *timetableAttendanceActor, _ *[]string) {
			d.settings.scope = configModel.OverviewScopeOwn
		}},
		{name: "unknown edit scope", change: func(d *timetableOpsTestDeps, _ *timetableAttendanceActor, _ *[]string) {
			d.settings.attendanceScope = "unknown"
		}},
		{name: "settings failure", change: func(d *timetableOpsTestDeps, _ *timetableAttendanceActor, _ *[]string) {
			d.settings.stringErr = errors.New("settings unavailable")
		}},
		{name: "missing action permission", change: func(_ *timetableOpsTestDeps, _ *timetableAttendanceActor, p *[]string) { *p = nil }},
		{name: "not staff", change: func(d *timetableOpsTestDeps, _ *timetableAttendanceActor, _ *[]string) {
			d.personService.accountPerson = nil
		}},
		{name: "other tenant", change: func(_ *timetableOpsTestDeps, c *timetableAttendanceActor, _ *[]string) { c.TenantID++ }},
		{name: "other actor", change: func(_ *timetableOpsTestDeps, c *timetableAttendanceActor, _ *[]string) { c.ID++ }},
		{name: "school portal", change: func(_ *timetableOpsTestDeps, c *timetableAttendanceActor, _ *[]string) { c.Scope = "school" }},
		{name: "parent portal", change: func(_ *timetableOpsTestDeps, c *timetableAttendanceActor, _ *[]string) { c.Scope = "parent" }},
		{name: "operator portal", change: func(_ *timetableOpsTestDeps, c *timetableAttendanceActor, _ *[]string) { c.Scope = "platform" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deps := newTimetableOpsDeps()
			const accountID, instanceID, groupID, studentID, visitID = int64(3191), int64(3192), int64(3193), int64(3194), int64(3195)
			wireAssignedStaff(deps, accountID, 3196, 3197, instanceID)
			deps.staffRepo.byInstance[instanceID] = nil
			deps.settings.scope = configModel.OverviewScopeAllStaff
			deps.settings.attendanceScope = configModel.AttendanceEditScopeAllStaff
			deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, groupID)
			claims := timetableAttendanceActor{ID: int(accountID), TenantID: 3198}
			permissions := []string{"schedules:read"}
			if tc.change != nil {
				tc.change(deps, &claims, &permissions)
			}
			ctx := tenant.WithTenantID(context.Background(), 3198)
			ctx = testpkg.IdentityContext(ctx, int64(claims.ID), claims.TenantID, claims.Scope, permissions)
			_, checkInErr := deps.service.CheckInStudent(ctx, accountID, false, instanceID, studentID)
			visit := &studentpresence.Visit{ID: visitID, StudentID: studentID, ActiveGroupID: groupID}
			deps.visitRepo.byActiveGroup[groupID] = []*studentpresence.Visit{visit}
			_, checkOutErr := deps.service.CheckOutStudent(ctx, accountID, false, instanceID, studentID)
			if tc.allowed {
				require.NoError(t, checkInErr)
				require.NoError(t, checkOutErr)
				require.Len(t, deps.activeService.created, 1)
				require.Equal(t, []int64{visitID}, deps.activeService.ended)
			} else {
				require.Error(t, checkInErr)
				require.Error(t, checkOutErr)
				require.Empty(t, deps.activeService.created)
				require.Empty(t, deps.activeService.ended)
			}
		})
	}
}

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
