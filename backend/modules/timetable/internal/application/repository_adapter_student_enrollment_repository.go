package application

import (
	"context"
	"errors"
	"fmt"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type timetableStudentEnrollmentRepository struct {
	timetable timetable.Capability
	alumni    func(context.Context, []int64) (map[int64]bool, error)
	names     func(context.Context, []int64) (map[int64]ports.PersonName, error)
}

func (r timetableStudentEnrollmentRepository) Create(ctx context.Context, enrollment *activityContract.StudentEnrollment) error {
	if enrollment == nil {
		return errors.New("student enrollment cannot be nil")
	}
	if err := enrollment.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreateStudentEnrollment(ctx, publicStudentEnrollmentInput(enrollment))
	if err != nil {
		return legacyDatabaseError("create", err)
	}
	*enrollment = *legacyStudentEnrollment(created)
	return nil
}

func (r timetableStudentEnrollmentRepository) FindByID(ctx context.Context, id int64) (*activityContract.StudentEnrollment, error) {
	enrollmentID, ok := LegacyGroupID(id)
	if !ok {
		return nil, legacyDatabaseError("find by id", fmt.Errorf("invalid student enrollment id %T", id))
	}
	value, err := r.timetable.FindStudentEnrollment(ctx, enrollmentID)
	if err != nil {
		return nil, legacyStudentEnrollmentError("find by id", err)
	}
	return legacyStudentEnrollment(value), nil
}

func (r timetableStudentEnrollmentRepository) Update(ctx context.Context, enrollment *activityContract.StudentEnrollment) error {
	if enrollment == nil {
		return errors.New("student enrollment cannot be nil")
	}
	if err := enrollment.Validate(); err != nil {
		return err
	}
	updated, err := r.timetable.UpdateStudentEnrollment(ctx, enrollment.ID, publicStudentEnrollmentInput(enrollment))
	if err != nil {
		if errors.Is(err, timetable.ErrStudentEnrollmentNotFound) {
			return legacyDatabaseError("update student_enrollment", errors.New("expected 1 rows affected, got 0"))
		}
		return legacyStudentEnrollmentError("update", err)
	}
	*enrollment = *legacyStudentEnrollment(updated)
	return nil
}

func (r timetableStudentEnrollmentRepository) Delete(ctx context.Context, id int64) error {
	enrollmentID, ok := LegacyGroupID(id)
	if !ok {
		return legacyDatabaseError("delete", fmt.Errorf("invalid student enrollment id %T", id))
	}
	if err := r.timetable.DeleteStudentEnrollment(ctx, enrollmentID); err != nil {
		return legacyDatabaseError("delete", err)
	}
	return nil
}

func (r timetableStudentEnrollmentRepository) FindByStudentIDs(ctx context.Context, studentIDs []int64) ([]*activityContract.StudentEnrollment, error) {
	values, err := r.timetable.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{StudentIDs: studentIDs})
	if err != nil {
		return nil, legacyStudentEnrollmentError("list with options", err)
	}
	return legacyStudentEnrollments(values), nil
}

func (r timetableStudentEnrollmentRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*activityContract.StudentEnrollment, error) {
	return r.list(ctx, timetable.StudentEnrollmentFilter{StudentIDs: []int64{studentID}, OrderByValidFrom: true}, "find by student ID")
}

func (r timetableStudentEnrollmentRepository) FindActiveByStudentIDs(ctx context.Context, studentIDs []int64, onDate activityContract.Date) ([]*activityContract.StudentEnrollment, error) {
	if len(studentIDs) == 0 {
		return []*activityContract.StudentEnrollment{}, nil
	}
	date := onDate.String()
	rows, err := r.list(ctx, timetable.StudentEnrollmentFilter{StudentIDs: studentIDs, ActiveOn: &date, OrderByGroupName: true}, "find active enrollments by student IDs")
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	groupIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		groupIDs = append(groupIDs, row.ActivityGroupID)
	}
	groups, err := r.timetable.ListGroups(ctx, timetable.GroupFilter{IDs: groupIDs})
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

func (r timetableStudentEnrollmentRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activityContract.StudentEnrollment, error) {
	rows, err := r.list(ctx, timetable.StudentEnrollmentFilter{ActivityGroupIDs: []int64{groupID}, OrderByValidFrom: true}, "find by group ID")
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	if r.alumni == nil || r.names == nil {
		return nil, errors.New("student enrollment repository: people directory is required")
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.StudentID)
	}
	alumni, err := r.alumni(ctx, ids)
	if err != nil {
		return nil, err
	}
	namesByID, err := r.names(ctx, ids)
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

func (r timetableStudentEnrollmentRepository) list(ctx context.Context, filter timetable.StudentEnrollmentFilter, operation string) ([]*activityContract.StudentEnrollment, error) {
	values, err := r.timetable.ListStudentEnrollments(ctx, filter)
	if err != nil {
		return nil, legacyStudentEnrollmentError(operation, err)
	}
	return legacyStudentEnrollments(values), nil
}

func (r timetableStudentEnrollmentRepository) CapActiveByGroup(ctx context.Context, groupID int64, validUntil activityContract.Date) (int64, error) {
	rows, err := r.timetable.CapActiveStudentEnrollments(ctx, groupID, validUntil.String())
	return rows, legacyStudentEnrollmentError("cap active enrollments by group", err)
}

func (r timetableStudentEnrollmentRepository) SetValidUntilByID(ctx context.Context, id int64, validUntil activityContract.Date) error {
	return legacyStudentEnrollmentError("set enrollment valid_until", r.timetable.SetStudentEnrollmentValidUntil(ctx, id, validUntil.String()))
}

func (r timetableStudentEnrollmentRepository) CloseOpenByGroupAndPeriod(ctx context.Context, groupID int64, periodID *int64, validUntil activityContract.Date) error {
	return legacyStudentEnrollmentError("close open enrollments by group and period", r.timetable.CloseOpenStudentEnrollments(ctx, groupID, periodID, validUntil.String()))
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
		AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday}
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
