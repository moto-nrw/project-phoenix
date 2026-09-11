package repositories

import (
	"context"
	"errors"
	"fmt"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type timetableStudentEnrollmentRepository struct {
	timetable timetable.Capability
	students  peopledirectory.StudentQuery
}

func (r timetableStudentEnrollmentRepository) Create(ctx context.Context, enrollment *activitiesModels.StudentEnrollment) error {
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

func (r timetableStudentEnrollmentRepository) FindByID(ctx context.Context, id any) (*activitiesModels.StudentEnrollment, error) {
	enrollmentID, ok := legacyGroupID(id)
	if !ok {
		return nil, legacyDatabaseError("find by id", fmt.Errorf("invalid student enrollment id %T", id))
	}
	value, err := r.timetable.FindStudentEnrollment(ctx, enrollmentID)
	if err != nil {
		return nil, legacyStudentEnrollmentError("find by id", err)
	}
	return legacyStudentEnrollment(value), nil
}

func (r timetableStudentEnrollmentRepository) Update(ctx context.Context, enrollment *activitiesModels.StudentEnrollment) error {
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

func (r timetableStudentEnrollmentRepository) Delete(ctx context.Context, id any) error {
	enrollmentID, ok := legacyGroupID(id)
	if !ok {
		return legacyDatabaseError("delete", fmt.Errorf("invalid student enrollment id %T", id))
	}
	if err := r.timetable.DeleteStudentEnrollment(ctx, enrollmentID); err != nil {
		return legacyDatabaseError("delete", err)
	}
	return nil
}

func (r timetableStudentEnrollmentRepository) List(ctx context.Context, options *activitiesModels.StudentEnrollmentQueryOptions) ([]*activitiesModels.StudentEnrollment, error) {
	filter, err := legacyStudentEnrollmentFilter(options)
	if err != nil {
		return nil, legacyDatabaseError("list with options", err)
	}
	values, err := r.timetable.ListStudentEnrollments(ctx, filter)
	if err != nil {
		return nil, legacyStudentEnrollmentError("list with options", err)
	}
	return legacyStudentEnrollments(values), nil
}

func (r timetableStudentEnrollmentRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*activitiesModels.StudentEnrollment, error) {
	return r.list(ctx, timetable.StudentEnrollmentFilter{StudentIDs: []int64{studentID}, OrderByValidFrom: true}, "find by student ID")
}

func (r timetableStudentEnrollmentRepository) FindActiveByStudentIDs(ctx context.Context, studentIDs []int64, onDate activitiesModels.StudentEnrollmentDate) ([]*activitiesModels.StudentEnrollment, error) {
	if len(studentIDs) == 0 {
		return []*activitiesModels.StudentEnrollment{}, nil
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
	byID := make(map[int64]*activitiesModels.Group, len(groups))
	for _, group := range groups {
		byID[group.ID] = legacyGroup(group)
	}
	for _, row := range rows {
		row.ActivityGroup = byID[row.ActivityGroupID]
	}
	return rows, nil
}

func (r timetableStudentEnrollmentRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activitiesModels.StudentEnrollment, error) {
	rows, err := r.list(ctx, timetable.StudentEnrollmentFilter{ActivityGroupIDs: []int64{groupID}, OrderByValidFrom: true}, "find by group ID")
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	if r.students == nil {
		return nil, errors.New("student enrollment repository: people directory is required")
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.StudentID)
	}
	students, err := r.students.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	names, err := r.students.ListStudentNamesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	alumni := make(map[int64]bool, len(students))
	for _, student := range students {
		alumni[student.ID] = student.IsAlumnus()
	}
	namesByID := make(map[int64]peopledirectory.StudentName, len(names))
	for _, name := range names {
		namesByID[name.StudentID] = name
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

func (r timetableStudentEnrollmentRepository) list(ctx context.Context, filter timetable.StudentEnrollmentFilter, operation string) ([]*activitiesModels.StudentEnrollment, error) {
	values, err := r.timetable.ListStudentEnrollments(ctx, filter)
	if err != nil {
		return nil, legacyStudentEnrollmentError(operation, err)
	}
	return legacyStudentEnrollments(values), nil
}

func (r timetableStudentEnrollmentRepository) BackfillEnrollmentRequestChildSource(ctx context.Context, studentID, requestChildID int64, groupIDs []int64) (int64, error) {
	if studentID <= 0 {
		return 0, errors.New("student_id is required")
	}
	if requestChildID <= 0 {
		return 0, errors.New("enrollment_request_child_id is required")
	}
	rows, err := r.timetable.BackfillStudentEnrollmentSource(ctx, studentID, requestChildID, groupIDs)
	return rows, legacyStudentEnrollmentError("backfill enrollment request child source", err)
}

func (r timetableStudentEnrollmentRepository) DeleteByEnrollmentRequestChild(ctx context.Context, studentID, requestChildID int64) (int64, error) {
	if studentID <= 0 {
		return 0, errors.New("student_id is required")
	}
	if requestChildID <= 0 {
		return 0, errors.New("enrollment_request_child_id is required")
	}
	rows, err := r.timetable.DeleteStudentEnrollmentsBySource(ctx, studentID, requestChildID)
	return rows, legacyStudentEnrollmentError("delete by enrollment request child", err)
}

func (r timetableStudentEnrollmentRepository) CapActiveByGroup(ctx context.Context, groupID int64, validUntil activitiesModels.StudentEnrollmentDate) (int64, error) {
	rows, err := r.timetable.CapActiveStudentEnrollments(ctx, groupID, validUntil.String())
	return rows, legacyStudentEnrollmentError("cap active enrollments by group", err)
}

func (r timetableStudentEnrollmentRepository) SetValidUntilByID(ctx context.Context, id int64, validUntil activitiesModels.StudentEnrollmentDate) error {
	return legacyStudentEnrollmentError("set enrollment valid_until", r.timetable.SetStudentEnrollmentValidUntil(ctx, id, validUntil.String()))
}

func (r timetableStudentEnrollmentRepository) CloseOpenByGroupAndPeriod(ctx context.Context, groupID int64, periodID *int64, validUntil activitiesModels.StudentEnrollmentDate) error {
	return legacyStudentEnrollmentError("close open enrollments by group and period", r.timetable.CloseOpenStudentEnrollments(ctx, groupID, periodID, validUntil.String()))
}

func legacyStudentEnrollments(values []timetable.StudentEnrollment) []*activitiesModels.StudentEnrollment {
	result := make([]*activitiesModels.StudentEnrollment, 0, len(values))
	for _, value := range values {
		result = append(result, legacyStudentEnrollment(value))
	}
	return result
}

func legacyStudentEnrollment(value timetable.StudentEnrollment) *activitiesModels.StudentEnrollment {
	result := &activitiesModels.StudentEnrollment{StudentID: value.StudentID, ActivityGroupID: value.ActivityGroupID,
		ValidFrom: activitiesModels.StudentEnrollmentDate(value.ValidFrom), CalendarPeriodID: value.CalendarPeriodID,
		EnrollmentRequestChildID: value.EnrollmentRequestChildID, SelectedWeekdays: value.SelectedWeekdays,
		AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday}
	if value.ValidUntil != nil {
		date := activitiesModels.StudentEnrollmentDate(*value.ValidUntil)
		result.ValidUntil = &date
	}
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	result.SetTenantID(value.TenantID)
	return result
}

func publicStudentEnrollmentInput(value *activitiesModels.StudentEnrollment) timetable.StudentEnrollmentInput {
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

func legacyStudentEnrollmentFilter(options *activitiesModels.StudentEnrollmentQueryOptions) (timetable.StudentEnrollmentFilter, error) {
	if options == nil {
		return timetable.StudentEnrollmentFilter{}, nil
	}
	if options.Limit < 0 || options.Offset < 0 {
		return timetable.StudentEnrollmentFilter{}, errors.New("student enrollment pagination cannot be negative")
	}
	for _, id := range options.StudentIDs {
		if id <= 0 {
			return timetable.StudentEnrollmentFilter{}, errors.New("student enrollment IDs must be positive")
		}
	}
	return timetable.StudentEnrollmentFilter{StudentIDs: options.StudentIDs, Limit: options.Limit, Offset: options.Offset}, nil
}
