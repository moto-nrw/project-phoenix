//go:build testhooks

package api

import (
	"context"

	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/moto-ogs/project-phoenix/workflows/staffoffboarding"
	offboardingcompose "github.com/moto-ogs/project-phoenix/workflows/staffoffboarding/compose"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The Timetable planning writes the People Directory offboarding suite races
// against the retirement of a staff member, in terms that suite may name.

// AssignInstanceStaffForTest plans a staff member onto a block through the
// Timetable owner's instance staff command.
func AssignInstanceStaffForTest(ctx context.Context, capability timetable.Capability, instanceID, staffID int64) error {
	_, err := capability.CreateInstanceStaff(ctx, timetable.InstanceStaffInput{InstanceID: instanceID, StaffID: staffID})
	return err
}

// MoveActivityInstanceForTest moves a block to another day through the
// retained activity instance rows.
func MoveActivityInstanceForTest(ctx context.Context, rows owners.TimetableRepositories, instanceID int64, date calendar.Date) error {
	instances := rows.NativeInstances()
	stored, err := instances.FindByID(ctx, instanceID)
	if err != nil {
		return err
	}
	stored.Date = date
	return instances.Update(ctx, stored)
}

// ReassignInstanceStaffForTest moves a planned assignment to another staff
// member through the Timetable owner's instance staff command.
func ReassignInstanceStaffForTest(ctx context.Context, staff timetable.InstanceStaffCapability, assignmentID, staffID int64) error {
	stored, err := staff.FindInstanceStaff(ctx, assignmentID)
	if err != nil {
		return err
	}
	_, err = staff.UpdateInstanceStaff(ctx, stored.ID, timetable.InstanceStaffInput{
		InstanceID: stored.InstanceID, StaffID: staffID, RoomID: stored.RoomID, IsPrimary: stored.IsPrimary,
		IsSubstitute: stored.IsSubstitute, IsAbsent: stored.IsAbsent, AbsenceReason: stored.AbsenceReason, SickAbsenceID: stored.SickAbsenceID,
	})
	return err
}

// MarkStaffAbsentForTest saves one Vertretungsplan change that marks the
// staff members absent on the block, through the Timetable owner's
// deviation writes.
func MarkStaffAbsentForTest(ctx context.Context, module TimetableTestModule, instanceID int64, staffIDs ...int64) error {
	absences := make([]timetable.DeviationAbsenceInput, 0, len(staffIDs))
	for _, staffID := range staffIDs {
		absences = append(absences, timetable.DeviationAbsenceInput{StaffID: staffID})
	}
	_, err := module.TimetableData.Deviations.ApplyDeviations(ctx, instanceID, timetable.ApplyDeviationsInput{Absences: absences})
	return err
}

// SubstituteStaffForTest saves one Sammel-Vertretung in which substituteID
// covers absentID on the days.
func SubstituteStaffForTest(ctx context.Context, module TimetableTestModule, absentID, substituteID int64, days ...calendar.Date) error {
	_, err := module.TimetableData.Deviations.ApplyBulkSubstitution(ctx, timetable.BulkSubstitutionInput{
		AbsentStaffID: absentID, SubstituteStaffID: &substituteID, Dates: days,
	})
	return err
}

// MaterializeDayForTest materializes the school's templates on one day, as
// the planner's manual run does.
func MaterializeDayForTest(ctx context.Context, module TimetableTestModule, day calendar.Date) error {
	_, err := module.Materialization.MaterializeForTenant(ctx, day, day, timetable.MaterializationSourceManual)
	return err
}

// ReplanTemplateDayForTest re-plans one template's occurrences of one day.
func ReplanTemplateDayForTest(ctx context.Context, module TimetableTestModule, templateID int64, day calendar.Date) error {
	_, err := module.Instance.ReplanWeek(ctx, day, day, &templateID, nil)
	return err
}

// AlignSeriesStaffForTest aligns the template's planned occurrences from
// the day on with its supervisors, for the staff members.
func AlignSeriesStaffForTest(ctx context.Context, module TimetableTestModule, templateID int64, staffIDs []int64, from calendar.Date) error {
	return module.TimetableData.Templates.AlignPlannedInstanceStaff(ctx, templateID, staffIDs, from)
}

// StaffOffboardForTest builds the production staff offboarding over deps,
// acting as the given staff member and account, and returns its immediate
// deletion for suites whose test role may not name the workflow (#6702).
func StaffOffboardForTest(deps offboardingcompose.Dependencies, actorStaffID, actorAccountID int64) (func(context.Context, int64) error, error) {
	deps.Authorize = func(ctx context.Context) (staffoffboarding.Actor, error) {
		return staffoffboarding.Actor{StaffID: actorStaffID, AccountID: actorAccountID, TenantID: tenant.FromContext(ctx), Username: "test-admin"}, nil
	}
	deps.Cleanup = func(context.Context, int64) error { return nil }
	workflow, err := NewStaffOffboardingWorkflow(deps)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, staffID int64) error {
		_, err := workflow.Offboard(ctx, staffID)
		return err
	}, nil
}
