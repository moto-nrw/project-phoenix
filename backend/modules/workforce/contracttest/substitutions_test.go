package contracttest_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/services"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type substitutionContract struct {
	overview func(context.Context, education.Caller, education.OverviewQuery) (*education.OverviewResult, error)
	assign   func(context.Context, education.Caller, education.Assignment) (*education.AssignmentResult, error)
	end      func(context.Context, education.Caller, education.EndRequest) error
}

func (s substitutionContract) Overview(c context.Context, a education.Caller, q education.OverviewQuery) (*education.OverviewResult, error) {
	return s.overview(c, a, q)
}
func (s substitutionContract) Assign(c context.Context, a education.Caller, q education.Assignment) (*education.AssignmentResult, error) {
	return s.assign(c, a, q)
}
func (s substitutionContract) End(c context.Context, a education.Caller, q education.EndRequest) error {
	return s.end(c, a, q)
}

func TestSubstitutionCapabilityPreservesCallerAndOverview(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "Substitution", "Contract")
	ctx := testpkg.Ctx(t)
	day := timezone.NewDate(2026, 9, 21)
	caller := workforce.SubstitutionCaller{AccountID: staff.ID, TenantID: testpkg.Tenant(t), Scope: "staff", Roles: []string{"teacher"}, HasPermission: func(p string) bool { return p == "substitutions:read" }}
	calls := 0
	capability := services.SubstitutionCapability(substitutionContract{overview: func(c context.Context, a education.Caller, q education.OverviewQuery) (*education.OverviewResult, error) {
		require.Equal(t, ctx, c)
		require.Equal(t, caller.TenantID, a.TenantID)
		require.Equal(t, caller.AccountID, a.AccountID)
		require.Equal(t, caller.Roles, a.Roles)
		require.Equal(t, "staff", a.Scope)
		require.True(t, a.HasPermission("substitutions:read"))
		require.False(t, a.Admin)
		if calls == 0 {
			require.Equal(t, day, *q.On)
			require.Equal(t, day, *q.ScheduleFrom)
			require.Equal(t, day, *q.ScheduleTo)
			require.True(t, q.IncludeTargets)
			require.True(t, q.IncludeScheduleTargets)
		} else {
			require.Nil(t, q.On)
			require.Nil(t, q.ScheduleFrom)
			require.Nil(t, q.ScheduleTo)
			calls++
			return nil, nil
		}
		calls++
		return &education.OverviewResult{Groups: []education.GroupRef{{ID: staff.ID, Name: "Klasse"}}, Targets: []education.StaffRef{{ID: staff.ID, FullName: "Target"}}, ScheduleTargets: []education.StaffRef{{ID: staff.ID, FullName: "Schedule"}}, GroupHandovers: []education.GroupHandover{{ID: staff.ID, Group: education.GroupRef{Name: "Klasse"}, Target: education.StaffRef{FullName: "Target"}, Period: education.Period{StartDate: day.String(), EndDate: day.String()}, CanEnd: true}}, ScheduleAppointments: []education.ScheduleAppointmentOverview{{ID: staff.ID, Date: day, StartTime: "09:00", EndTime: "10:00", Title: "Betreuung", Staff: []education.ScheduleAppointmentStaff{{AssignmentID: staff.ID, Staff: education.StaffRef{ID: staff.ID}, IsSubstitute: true, CanEnd: true}}}}, RunningSupervisions: []education.RunningSupervision{{ID: staff.ID, Name: "Raum", CanAssign: true, IsCurrentUserSupervising: true, Supervisors: []education.StaffRef{{ID: staff.ID}}, AvailableTargets: []education.StaffRef{{ID: staff.ID}}}}}, nil
	}})
	out, err := capability.Overview(ctx, caller, workforce.SubstitutionOverviewQuery{On: day.String(), ScheduleFrom: day.String(), ScheduleTo: day.String(), IncludeTargets: true, IncludeScheduleTargets: true})
	require.NoError(t, err)
	require.Equal(t, "Klasse", out.Groups[0].Name)
	require.Equal(t, "Target", out.Targets[0].FullName)
	require.Equal(t, "Schedule", out.ScheduleTargets[0].FullName)
	require.True(t, out.GroupHandovers[0].CanEnd)
	require.Equal(t, "2026-09-21", out.ScheduleAppointments[0].Date)
	require.True(t, out.ScheduleAppointments[0].Staff[0].IsSubstitute)
	require.True(t, out.RunningSupervisions[0].CanAssign)
	require.Equal(t, staff.ID, out.RunningSupervisions[0].AvailableTargets[0].ID)
	out, err = capability.Overview(ctx, caller, workforce.SubstitutionOverviewQuery{})
	require.NoError(t, err)
	require.Nil(t, out)
	require.Equal(t, 2, calls)
}

func TestSubstitutionCapabilityPreservesAssignmentPresenceAndWholeDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "Assignment", "Contract")
	ctx := testpkg.Ctx(t)
	day := timezone.NewDate(2026, 9, 21)
	for _, populated := range []bool{false, true} {
		t.Run(fmt.Sprint(populated), func(t *testing.T) {
			input := workforce.SubstitutionAssignment{GroupHandover: &workforce.GroupHandoverAssignment{GroupID: staff.ID, TargetStaffID: staff.ID, StartDate: day.String(), EndDate: day.String()}, AdditionalSupervision: &workforce.AdditionalSupervisionAssignment{ActiveGroupID: staff.ID, TargetStaffID: staff.ID}, ScheduleSubstitution: &workforce.ScheduleSubstitutionAssignment{InstanceID: staff.ID}}
			if populated {
				input.ScheduleSubstitution.Absences = []workforce.ScheduleAbsenceChange{{StaffID: staff.ID}}
				input.ScheduleSubstitution.Substitutions = []workforce.ScheduleSubstitutionChange{{AbsentStaffID: staff.ID, SubstituteStaffID: staff.ID}}
				input.ScheduleSubstitution.SubstitutionRemovals = []workforce.ScheduleSubstitutionRemoval{{StaffID: staff.ID}}
				input.ScheduleSubstitution.Presences = []workforce.SchedulePresenceChange{{StaffID: staff.ID}}
				input.ScheduleSubstitution.WholeDays = &workforce.ScheduleWholeDayAssignment{AbsentStaffID: staff.ID, SubstituteStaffID: &staff.ID, Dates: []string{day.String()}}
			}
			capability := services.SubstitutionCapability(substitutionContract{assign: func(c context.Context, _ education.Caller, a education.Assignment) (*education.AssignmentResult, error) {
				require.Equal(t, ctx, c)
				require.Equal(t, staff.ID, a.GroupHandover.GroupID)
				require.Equal(t, day, *a.GroupHandover.StartDate)
				require.Equal(t, day, *a.GroupHandover.EndDate)
				require.Equal(t, staff.ID, a.AdditionalSupervision.ActiveGroupID)
				if populated {
					require.Equal(t, staff.ID, a.ScheduleSubstitution.Absences[0].StaffID)
					require.Equal(t, staff.ID, a.ScheduleSubstitution.Substitutions[0].SubstituteStaffID)
					require.Equal(t, staff.ID, a.ScheduleSubstitution.SubstitutionRemovals[0].StaffID)
					require.Equal(t, staff.ID, a.ScheduleSubstitution.Presences[0].StaffID)
					require.Equal(t, []timezone.Date{day}, a.ScheduleSubstitution.WholeDays.Dates)
				} else {
					require.Nil(t, a.ScheduleSubstitution.Absences)
					require.Nil(t, a.ScheduleSubstitution.Substitutions)
					require.Nil(t, a.ScheduleSubstitution.SubstitutionRemovals)
					require.Nil(t, a.ScheduleSubstitution.Presences)
					require.Nil(t, a.ScheduleSubstitution.WholeDays)
				}
				result := &education.AssignmentResult{ID: staff.ID, Group: &education.GroupRef{ID: staff.ID, Name: "Klasse"}, Period: &education.Period{StartDate: day.String()}, Target: education.StaffRef{ID: staff.ID}, CanEnd: true, ScheduleSubstitution: &education.ScheduleSubstitutionResult{InstanceID: staff.ID, TotalAffected: 1}}
				if populated {
					result.ScheduleSubstitution.AffectedAppointments = []education.ScheduleAffectedAppointment{{InstanceID: staff.ID, Title: "Betreuung", StartTime: "09:00", Action: "substituted"}}
					result.ScheduleSubstitution.Warnings = []education.ScheduleTimeConflict{{Kind: "overlap", OtherID: staff.ID, Message: "conflict"}}
					result.ScheduleSubstitution.Days = []education.ScheduleSubstitutionDayResult{{Date: day, AffectedAppointments: result.ScheduleSubstitution.AffectedAppointments, Warnings: result.ScheduleSubstitution.Warnings}}
				}
				return result, nil
			}})
			out, err := capability.Assign(ctx, workforce.SubstitutionCaller{}, input)
			require.NoError(t, err)
			require.Equal(t, "Klasse", out.Group.Name)
			require.Equal(t, day.String(), out.Period.StartDate)
			require.True(t, out.CanEnd)
			require.Equal(t, 1, out.ScheduleSubstitution.TotalAffected)
			if populated {
				require.Equal(t, "substituted", out.ScheduleSubstitution.AffectedAppointments[0].Action)
				require.Equal(t, "conflict", out.ScheduleSubstitution.Warnings[0].Message)
				require.Equal(t, day.String(), out.ScheduleSubstitution.Days[0].Date)
				require.Equal(t, out.ScheduleSubstitution.Warnings, out.ScheduleSubstitution.Days[0].Warnings)
			} else {
				require.Nil(t, out.ScheduleSubstitution.AffectedAppointments)
				require.Nil(t, out.ScheduleSubstitution.Warnings)
				require.Nil(t, out.ScheduleSubstitution.Days)
			}
		})
	}
}

func TestSubstitutionCapabilityRejectsDatesBeforeCallingModule(t *testing.T) {
	t.Parallel()
	capability := services.SubstitutionCapability(substitutionContract{})
	ctx := context.Background()
	caller := workforce.SubstitutionCaller{}
	for _, query := range []workforce.SubstitutionOverviewQuery{{On: "bad"}, {ScheduleFrom: "bad"}, {ScheduleTo: "bad"}} {
		_, err := capability.Overview(ctx, caller, query)
		require.ErrorIs(t, err, workforce.ErrSubstitutionInvalidPeriod)
	}
	for _, input := range []workforce.SubstitutionAssignment{
		{GroupHandover: &workforce.GroupHandoverAssignment{StartDate: "bad"}},
		{GroupHandover: &workforce.GroupHandoverAssignment{EndDate: "bad"}},
		{ScheduleSubstitution: &workforce.ScheduleSubstitutionAssignment{WholeDays: &workforce.ScheduleWholeDayAssignment{Dates: []string{"bad"}}}},
		{ScheduleSubstitution: &workforce.ScheduleSubstitutionAssignment{WholeDays: &workforce.ScheduleWholeDayAssignment{Dates: []string{""}}}},
	} {
		_, err := capability.Assign(ctx, caller, input)
		require.ErrorIs(t, err, workforce.ErrSubstitutionInvalidPeriod)
	}
}

func TestSubstitutionCapabilityPreservesOperationErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, pair := range [][2]error{{education.ErrNotFound, workforce.ErrSubstitutionNotFound}, {education.ErrForbidden, workforce.ErrSubstitutionForbidden}, {education.ErrInvalidTarget, workforce.ErrSubstitutionInvalidTarget}, {education.ErrInvalidPeriod, workforce.ErrSubstitutionInvalidPeriod}, {education.ErrNotRunning, workforce.ErrSubstitutionNotRunning}, {education.ErrAlreadyAssigned, workforce.ErrSubstitutionAlreadyAssigned}, {education.ErrConflict, workforce.ErrSubstitutionConflict}, {education.ErrSelfAssignment, workforce.ErrSubstitutionSelfAssignment}} {
		for _, structured := range []bool{false, true} {
			cause := errors.New("storage failed")
			var failure error = fmt.Errorf("operation: %w", pair[0])
			if structured {
				failure = &education.OperationError{Target: pair[0], Code: "detail", Message: "operation failed", Cause: cause}
			}
			capability := services.SubstitutionCapability(substitutionContract{end: func(context.Context, education.Caller, education.EndRequest) error { return failure }, assign: func(context.Context, education.Caller, education.Assignment) (*education.AssignmentResult, error) {
				return nil, failure
			}, overview: func(context.Context, education.Caller, education.OverviewQuery) (*education.OverviewResult, error) {
				return nil, failure
			}})
			err := capability.End(ctx, workforce.SubstitutionCaller{}, workforce.SubstitutionEndRequest{})
			require.ErrorIs(t, err, pair[1])
			require.Equal(t, failure.Error(), err.Error())
			if structured {
				var detail *workforce.SubstitutionOperationError
				require.ErrorAs(t, err, &detail)
				require.Equal(t, "detail", detail.Code)
				require.ErrorIs(t, err, cause)
			} else {
				require.ErrorIs(t, err, pair[0])
			}
			_, err = capability.Assign(ctx, workforce.SubstitutionCaller{}, workforce.SubstitutionAssignment{})
			require.ErrorIs(t, err, pair[1])
			_, err = capability.Overview(ctx, workforce.SubstitutionCaller{}, workforce.SubstitutionOverviewQuery{})
			require.ErrorIs(t, err, pair[1])
		}
	}
}
