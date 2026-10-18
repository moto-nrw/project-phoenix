//go:build testhooks

package application

import (
	"context"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	models "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type nativeFixtureEnrollment struct {
	fixtureRecord[timetable.StudentEnrollment, activityContract.StudentEnrollment]
	rows models.StudentEnrollmentRepository
}

func NativeEnrollmentRecords(rows models.StudentEnrollmentRepository) ports.NativeEnrollmentRecords {
	return nativeFixtureEnrollment{fixtureRecord: fixtureRecord[timetable.StudentEnrollment, activityContract.StudentEnrollment]{rows: rows, encode: encodeFixtureEnrollment, decode: decodeFixtureEnrollment}, rows: rows}
}
func encodeFixtureEnrollment(value *timetable.StudentEnrollment) (*activityContract.StudentEnrollment, error) {
	if value == nil {
		return nil, nil
	}
	return legacyStudentEnrollment(*value), nil
}
func decodeFixtureEnrollment(value *activityContract.StudentEnrollment) *timetable.StudentEnrollment {
	if value == nil {
		return nil
	}
	input := publicStudentEnrollmentInput(value)
	return &timetable.StudentEnrollment{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: input.StudentID, ActivityGroupID: input.ActivityGroupID, ValidFrom: input.ValidFrom, ValidUntil: input.ValidUntil, CalendarPeriodID: input.CalendarPeriodID, EnrollmentRequestChildID: input.EnrollmentRequestChildID, SelectedWeekdays: input.SelectedWeekdays, AttendanceStatus: input.AttendanceStatus, Weekday: input.Weekday}
}
func (r nativeFixtureEnrollment) FindActiveByStudentIDs(ctx context.Context, ids []int64, date string) ([]*timetable.StudentEnrollment, error) {
	rows, err := r.rows.FindActiveByStudentIDs(ctx, ids, activityContract.Date(date))
	return fixtureNativeRows(rows, decodeFixtureEnrollment), err
}
