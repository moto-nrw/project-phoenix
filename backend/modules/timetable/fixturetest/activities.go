//go:build testhooks

package fixturetest

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

// EnrollmentWriter creates the retained protected roster rows exercised by integration suites.
type EnrollmentWriter interface {
	Create(context.Context, *activityContract.StudentEnrollment) error
}

// EnrollmentWriterOf writes the retained rows through the owner's
// enrollment command, with the retained row checks.
func EnrollmentWriterOf(enrollments timetable.StudentEnrollmentCommand) EnrollmentWriter {
	return enrollmentWriter{enrollments: enrollments}
}

type enrollmentWriter struct {
	enrollments timetable.StudentEnrollmentCommand
}

func (w enrollmentWriter) Create(ctx context.Context, row *activityContract.StudentEnrollment) error {
	if err := row.Validate(); err != nil {
		return err
	}
	var validUntil *string
	if row.ValidUntil != nil {
		text := row.ValidUntil.String()
		validUntil = &text
	}
	created, err := w.enrollments.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: row.StudentID, ActivityGroupID: row.ActivityGroupID, ValidFrom: row.ValidFrom.String(), ValidUntil: validUntil,
		CalendarPeriodID: row.CalendarPeriodID, EnrollmentRequestChildID: row.EnrollmentRequestChildID,
		SelectedWeekdays: row.SelectedWeekdays, AttendanceStatus: row.AttendanceStatus, Weekday: row.Weekday,
	})
	if err != nil {
		return err
	}
	row.ID, row.CreatedAt, row.UpdatedAt = created.ID, created.CreatedAt, created.UpdatedAt
	row.SetTenantID(created.TenantID)
	return nil
}

// SupervisorWriter creates the retained planned supervisor rows exercised
// by integration suites.
type SupervisorWriter interface {
	Create(context.Context, *activityContract.SupervisorPlanned) error
}

// SupervisorWriterOf writes the retained rows through the owner's planned
// supervisor command, with the retained row checks.
func SupervisorWriterOf(supervisors timetable.PlannedSupervisorCommand) SupervisorWriter {
	return supervisorWriter{supervisors: supervisors}
}

type supervisorWriter struct {
	supervisors timetable.PlannedSupervisorCommand
}

func (w supervisorWriter) Create(ctx context.Context, row *activityContract.SupervisorPlanned) error {
	if err := row.Validate(); err != nil {
		return err
	}
	validFrom, validUntil := row.ValidityDateStrings()
	created, err := w.supervisors.CreatePlannedSupervisor(ctx, timetable.PlannedSupervisorInput{
		StaffID: row.StaffID, GroupID: row.GroupID, IsPrimary: row.IsPrimary, ValidFrom: validFrom, ValidUntil: validUntil,
		CalendarPeriodID: row.CalendarPeriodID, Weekday: row.Weekday,
	})
	if err != nil {
		return err
	}
	row.ID, row.CreatedAt, row.UpdatedAt = created.ID, created.CreatedAt, created.UpdatedAt
	row.SetTenantID(created.TenantID)
	return nil
}
