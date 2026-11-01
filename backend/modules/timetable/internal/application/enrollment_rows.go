package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// The activity service, the template writes and split, the materializer and
// the roster maintenance still compute on the legacy enrollment row. They
// call the owner's enrollment capability through these reads and writes,
// which convert the rows and report failures under the operation names the
// retained repository used. A group's enrollments carry each child's alumni
// state and name from the directories, as the retained repository did.

func createLegacyStudentEnrollment(ctx context.Context, enrollments timetable.StudentEnrollmentCommand, enrollment *activityContract.StudentEnrollment) error {
	if enrollment == nil {
		return errors.New("student enrollment cannot be nil")
	}
	if err := enrollment.Validate(); err != nil {
		return err
	}
	created, err := enrollments.CreateStudentEnrollment(ctx, publicStudentEnrollmentInput(enrollment))
	if err != nil {
		return legacyDatabaseError("create", err)
	}
	*enrollment = *legacyStudentEnrollment(created)
	return nil
}

// updateLegacyStudentEnrollment reports a vanished row as the failed
// single-row update the retained repository reported.
func updateLegacyStudentEnrollment(ctx context.Context, enrollments timetable.StudentEnrollmentCommand, enrollment *activityContract.StudentEnrollment) error {
	if enrollment == nil {
		return errors.New("student enrollment cannot be nil")
	}
	if err := enrollment.Validate(); err != nil {
		return err
	}
	updated, err := enrollments.UpdateStudentEnrollment(ctx, enrollment.ID, publicStudentEnrollmentInput(enrollment))
	if err != nil {
		if errors.Is(err, timetable.ErrStudentEnrollmentNotFound) {
			return legacyDatabaseError("update student_enrollment", errors.New("expected 1 rows affected, got 0"))
		}
		return legacyStudentEnrollmentError("update", err)
	}
	*enrollment = *legacyStudentEnrollment(updated)
	return nil
}

func deleteLegacyStudentEnrollment(ctx context.Context, enrollments timetable.StudentEnrollmentCommand, id int64) error {
	if err := enrollments.DeleteStudentEnrollment(ctx, id); err != nil {
		return legacyDatabaseError("delete", err)
	}
	return nil
}

func listLegacyStudentEnrollments(ctx context.Context, enrollments timetable.StudentEnrollmentQuery, filter timetable.StudentEnrollmentFilter, operation string) ([]*activityContract.StudentEnrollment, error) {
	values, err := enrollments.ListStudentEnrollments(ctx, filter)
	if err != nil {
		return nil, legacyStudentEnrollmentError(operation, err)
	}
	return legacyStudentEnrollments(values), nil
}

// studentEnrollments lists a child's enrollments by their start.
func studentEnrollments(ctx context.Context, enrollments timetable.StudentEnrollmentQuery, studentID int64) ([]*activityContract.StudentEnrollment, error) {
	return listLegacyStudentEnrollments(ctx, enrollments, timetable.StudentEnrollmentFilter{StudentIDs: []int64{studentID}, OrderByValidFrom: true}, "find by student ID")
}

// groupStudentEnrollments lists a group's enrollments by their start, each
// with the child's alumni state and name.
func groupStudentEnrollments(ctx context.Context, enrollments timetable.StudentEnrollmentQuery, directories ports.RepositoryDirectories, groupID int64) ([]*activityContract.StudentEnrollment, error) {
	rows, err := listLegacyStudentEnrollments(ctx, enrollments, timetable.StudentEnrollmentFilter{ActivityGroupIDs: []int64{groupID}, OrderByValidFrom: true}, "find by group ID")
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	if directories.StudentAlumni == nil || directories.StudentNames == nil {
		return nil, errors.New("student enrollment repository: people directory is required")
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.StudentID)
	}
	alumni, err := directories.StudentAlumni(ctx, ids)
	if err != nil {
		return nil, err
	}
	namesByID, err := directories.StudentNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		row.StudentAlumnus = alumni[row.StudentID]
		if name, ok := namesByID[row.StudentID]; ok {
			row.StudentFirstName = name.FirstName
			row.StudentLastName = name.LastName
		}
	}
	return rows, nil
}

// activeStudentEnrollments lists the enrollments of the children active on
// the date, by group name, each with its group.
func activeStudentEnrollments(ctx context.Context, owner timetable.Query, studentIDs []int64, onDate activityContract.Date) ([]*activityContract.StudentEnrollment, error) {
	if len(studentIDs) == 0 {
		return []*activityContract.StudentEnrollment{}, nil
	}
	date := onDate.String()
	rows, err := listLegacyStudentEnrollments(ctx, owner, timetable.StudentEnrollmentFilter{StudentIDs: studentIDs, ActiveOn: &date, OrderByGroupName: true}, "find active enrollments by student IDs")
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	groupIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		groupIDs = append(groupIDs, row.ActivityGroupID)
	}
	groups, err := owner.ListGroups(ctx, timetable.GroupFilter{IDs: groupIDs})
	if err != nil {
		return nil, legacyDatabaseError("find active enrollment groups", err)
	}
	byID := make(map[int64]*activityContract.Group, len(groups))
	for _, group := range groups {
		byID[group.ID] = legacyGroup(group)
	}
	for _, row := range rows {
		row.ActivityGroup = byID[row.ActivityGroupID]
	}
	return rows, nil
}

func capLegacyStudentEnrollments(ctx context.Context, enrollments timetable.StudentEnrollmentCommand, groupID int64, validUntil activityContract.Date) (int64, error) {
	rows, err := enrollments.CapActiveStudentEnrollments(ctx, groupID, validUntil.String())
	return rows, legacyStudentEnrollmentError("cap active enrollments by group", err)
}

func setLegacyStudentEnrollmentValidUntil(ctx context.Context, enrollments timetable.StudentEnrollmentCommand, id int64, validUntil activityContract.Date) error {
	return legacyStudentEnrollmentError("set enrollment valid_until", enrollments.SetStudentEnrollmentValidUntil(ctx, id, validUntil.String()))
}

func closeOpenLegacyStudentEnrollments(ctx context.Context, enrollments timetable.StudentEnrollmentCommand, groupID int64, periodID *int64, validUntil activityContract.Date) error {
	return legacyStudentEnrollmentError("close open enrollments by group and period", enrollments.CloseOpenStudentEnrollments(ctx, groupID, periodID, validUntil.String()))
}

func legacyStudentEnrollments(values []timetable.StudentEnrollment) []*activityContract.StudentEnrollment {
	result := make([]*activityContract.StudentEnrollment, 0, len(values))
	for _, value := range values {
		result = append(result, legacyStudentEnrollment(value))
	}
	return result
}

func legacyStudentEnrollment(value timetable.StudentEnrollment) *activityContract.StudentEnrollment {
	result := &activityContract.StudentEnrollment{StudentID: value.StudentID, ActivityGroupID: value.ActivityGroupID,
		ValidFrom: activityContract.Date(value.ValidFrom), CalendarPeriodID: value.CalendarPeriodID,
		EnrollmentRequestChildID: value.EnrollmentRequestChildID, SelectedWeekdays: value.SelectedWeekdays,
		AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday}
	if value.ValidUntil != nil {
		date := activityContract.Date(*value.ValidUntil)
		result.ValidUntil = &date
	}
	for weekday, date := range value.KeptWithoutBooking {
		if result.KeptWithoutBooking == nil {
			result.KeptWithoutBooking = make(map[int]activityContract.Date, len(value.KeptWithoutBooking))
		}
		result.KeptWithoutBooking[weekday] = activityContract.Date(date)
	}
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	result.SetTenantID(value.TenantID)
	return result
}

func publicStudentEnrollmentInput(value *activityContract.StudentEnrollment) timetable.StudentEnrollmentInput {
	var validUntil *string
	if value.ValidUntil != nil {
		text := value.ValidUntil.String()
		validUntil = &text
	}
	return timetable.StudentEnrollmentInput{StudentID: value.StudentID, ActivityGroupID: value.ActivityGroupID,
		ValidFrom: value.ValidFrom.String(), ValidUntil: validUntil, CalendarPeriodID: value.CalendarPeriodID,
		EnrollmentRequestChildID: value.EnrollmentRequestChildID, SelectedWeekdays: value.SelectedWeekdays,
		AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday,
		KeptWithoutBooking: keptWeekdayStrings(value.KeptWithoutBooking, nil)}
}

// keptWeekdayStrings copies a row's per-weekday keep decision (#5102), all
// weekdays when only is nil, else the one weekday only.
func keptWeekdayStrings(kept map[int]activityContract.Date, only *int) map[int]string {
	var result map[int]string
	for weekday, date := range kept {
		if only != nil && *only != weekday {
			continue
		}
		if result == nil {
			result = make(map[int]string, len(kept))
		}
		result[weekday] = date.String()
	}
	return result
}

func legacyStudentEnrollmentError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, timetable.ErrStudentEnrollmentNotFound) || errors.Is(err, timetable.ErrInvalidStudentEnrollmentQuery) {
		return legacyNotFoundError(operation)
	}
	return legacyDatabaseError(operation, err)
}
