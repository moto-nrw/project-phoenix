//go:build testhooks

package application

import (
	"context"
	"errors"
	"fmt"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// nativeFixtureEnrollment writes fixture enrollments through the owner's
// enrollment capability with the legacy row checks.
type nativeFixtureEnrollment struct {
	enrollments timetable.StudentEnrollmentCapability
}

func NativeEnrollmentRecords(enrollments timetable.StudentEnrollmentCapability) ports.NativeEnrollmentRecords {
	return nativeFixtureEnrollment{enrollments: enrollments}
}

func (r nativeFixtureEnrollment) Create(ctx context.Context, value *timetable.StudentEnrollment) error {
	if value == nil {
		return createLegacyStudentEnrollment(ctx, r.enrollments, nil)
	}
	row := legacyStudentEnrollment(*value)
	defer func() { *value = *decodeFixtureEnrollment(row) }()
	if err := row.Validate(); err != nil {
		return err
	}
	created, err := r.enrollments.CreateStudentEnrollment(ctx, publicStudentEnrollmentInput(row))
	if err != nil {
		return legacyDatabaseError("create", err)
	}
	*row = *legacyStudentEnrollment(created)
	return nil
}

func (r nativeFixtureEnrollment) Update(ctx context.Context, value *timetable.StudentEnrollment) error {
	if value == nil {
		return updateLegacyStudentEnrollment(ctx, r.enrollments, nil)
	}
	row := legacyStudentEnrollment(*value)
	defer func() { *value = *decodeFixtureEnrollment(row) }()
	if err := row.Validate(); err != nil {
		return err
	}
	updated, err := r.enrollments.UpdateStudentEnrollment(ctx, row.ID, publicStudentEnrollmentInput(row))
	if errors.Is(err, timetable.ErrStudentEnrollmentNotFound) {
		return legacyDatabaseError("update student_enrollment", errors.New("expected 1 rows affected, got 0"))
	}
	if err != nil {
		return legacyStudentEnrollmentError("update", err)
	}
	*row = *legacyStudentEnrollment(updated)
	return nil
}

func (r nativeFixtureEnrollment) FindByID(ctx context.Context, id any) (*timetable.StudentEnrollment, error) {
	enrollmentID, ok := LegacyGroupID(id)
	if !ok {
		return nil, legacyDatabaseError("find by id", fmt.Errorf("invalid activity record id %T", id))
	}
	value, err := r.enrollments.FindStudentEnrollment(ctx, enrollmentID)
	if err != nil {
		return nil, legacyStudentEnrollmentError("find by id", err)
	}
	return &value, nil
}

// FindActiveByStudentIDs lists the enrollments of the children active on
// date, by group name.
func (r nativeFixtureEnrollment) FindActiveByStudentIDs(ctx context.Context, ids []int64, date string) ([]*timetable.StudentEnrollment, error) {
	if len(ids) == 0 {
		return []*timetable.StudentEnrollment{}, nil
	}
	values, err := r.enrollments.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{StudentIDs: ids, ActiveOn: &date, OrderByGroupName: true})
	if err != nil {
		return nil, legacyStudentEnrollmentError("find active enrollments by student IDs", err)
	}
	result := make([]*timetable.StudentEnrollment, len(values))
	for index := range values {
		result[index] = &values[index]
	}
	return result, nil
}

func decodeFixtureEnrollment(value *activityContract.StudentEnrollment) *timetable.StudentEnrollment {
	if value == nil {
		return nil
	}
	input := publicStudentEnrollmentInput(value)
	return &timetable.StudentEnrollment{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: input.StudentID, ActivityGroupID: input.ActivityGroupID, ValidFrom: input.ValidFrom, ValidUntil: input.ValidUntil, CalendarPeriodID: input.CalendarPeriodID, EnrollmentRequestChildID: input.EnrollmentRequestChildID, SelectedWeekdays: input.SelectedWeekdays, AttendanceStatus: input.AttendanceStatus, Weekday: input.Weekday}
}
