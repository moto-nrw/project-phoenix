package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func (s *Service) ListStudentDirectory(ctx context.Context, filter peopledirectory.StudentDirectoryFilter) ([]peopledirectory.StudentRecord, error) {
	filter, err := peopledirectory.NormalizeStudentDirectoryFilter(filter)
	if err != nil {
		return nil, err
	}

	values, err := s.students.ListDirectory(ctx, toDomainStudentDirectoryFilter(filter))
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]peopledirectory.StudentRecord, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentRecord(value))
	}
	return result, nil
}

func (s *Service) CountStudentDirectory(ctx context.Context, filter peopledirectory.StudentDirectoryFilter) (int, error) {
	filter, err := peopledirectory.NormalizeStudentDirectoryFilter(filter)
	if err != nil {
		return 0, err
	}
	// The total names the selection, never the page.
	filter.Page, filter.PageSize = 0, 0

	total, err := s.students.CountDirectory(ctx, toDomainStudentDirectoryFilter(filter))
	return total, MapError(err)
}

func (s *Service) ListStudentDirectoryIDs(ctx context.Context) ([]int64, error) {
	ids, err := s.students.ListDirectoryIDs(ctx)
	return ids, MapError(err)
}

func (s *Service) FindStudentRecordForMutation(ctx context.Context, studentID int64) (peopledirectory.StudentRecord, error) {
	if studentID <= 0 {
		return peopledirectory.StudentRecord{}, invalidStudent("student ID is required")
	}

	record, err := s.students.FindRecord(ctx, studentID, "UPDATE")
	return peopledirectory.StudentRecord(record), MapError(err)
}

func (s *Service) ListStudentRecordsByID(ctx context.Context, ids []int64) ([]peopledirectory.StudentRecord, error) {
	ids = peopledirectory.PositiveIDs(ids)
	if len(ids) == 0 {
		return []peopledirectory.StudentRecord{}, nil
	}

	values, err := s.students.ListRecordsByIDs(ctx, ids)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]peopledirectory.StudentRecord, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentRecord(value))
	}
	return result, nil
}

func (s *Service) LockStudentPhotoFeature(ctx context.Context) error {
	return mapPhotoError(s.studentPhotos.LockFeature(ctx))
}

func (s *Service) FindStudentRecord(ctx context.Context, studentID int64) (peopledirectory.StudentRecord, error) {
	if studentID <= 0 {
		return peopledirectory.StudentRecord{}, invalidStudent("student ID is required")
	}

	record, err := s.students.FindRecord(ctx, studentID, "")
	return peopledirectory.StudentRecord(record), MapError(err)
}

func (s *Service) ListStudentRecordsByPerson(ctx context.Context, personIDs []int64) ([]peopledirectory.StudentRecord, error) {
	personIDs = peopledirectory.PositiveIDs(personIDs)
	if len(personIDs) == 0 {
		return []peopledirectory.StudentRecord{}, nil
	}

	values, err := s.students.ListRecordsByPersonIDs(ctx, personIDs)
	return toPublicStudentRecords(values), MapError(err)
}

func (s *Service) ListStudentRecordsByGroup(
	ctx context.Context,
	groupIDs []int64,
	scope peopledirectory.StudentScope,
) ([]peopledirectory.StudentRecord, error) {
	groupIDs = peopledirectory.PositiveIDs(groupIDs)
	if len(groupIDs) == 0 {
		return []peopledirectory.StudentRecord{}, nil
	}

	values, err := s.students.ListRecordsByGroups(ctx, groupIDs, string(scope))
	return toPublicStudentRecords(values), MapError(err)
}

func (s *Service) ListStudentRecords(ctx context.Context, scope peopledirectory.StudentScope) ([]peopledirectory.StudentRecord, error) {
	values, err := s.students.ListRecords(ctx, string(scope))
	return toPublicStudentRecords(values), MapError(err)
}

func (s *Service) ListStudentRecordsByClass(
	ctx context.Context,
	classes []string,
	scope peopledirectory.StudentScope,
) ([]peopledirectory.StudentRecord, error) {
	classes = peopledirectory.SchoolClasses(classes)
	if len(classes) == 0 {
		return []peopledirectory.StudentRecord{}, nil
	}

	values, err := s.students.ListRecordsByClasses(ctx, classes, string(scope))
	return toPublicStudentRecords(values), MapError(err)
}

func (s *Service) ListStudentRecordsDueForStatus(ctx context.Context, status, bound, asOf string) ([]peopledirectory.StudentRecord, error) {
	if status == "" {
		return nil, invalidStudent("status is required")
	}
	switch bound {
	case peopledirectory.StudentBoundCareStart, peopledirectory.StudentBoundCareEnd:
	default:
		return nil, invalidStudent("enrolment bound is invalid")
	}
	if _, err := time.Parse(peopledirectory.BirthdayLayout, asOf); err != nil {
		return nil, invalidStudent("the day must be a calendar date in YYYY-MM-DD format")
	}

	values, err := s.students.ListRecordsDueForStatus(ctx, status, bound, asOf)
	return toPublicStudentRecords(values), MapError(err)
}

func (s *Service) LockStudentRecordsByID(ctx context.Context, ids []int64) ([]peopledirectory.StudentRecord, error) {
	ids = peopledirectory.PositiveIDs(ids)
	if len(ids) == 0 {
		return []peopledirectory.StudentRecord{}, nil
	}

	values, err := s.students.LockRecordsByIDs(ctx, ids)
	return toPublicStudentRecords(values), MapError(err)
}

func (s *Service) CountStudentsByGroup(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	groupIDs = peopledirectory.PositiveIDs(groupIDs)
	if len(groupIDs) == 0 {
		return map[int64]int{}, nil
	}

	counts, err := s.students.CountByGroups(ctx, groupIDs)
	return counts, MapError(err)
}

func (s *Service) ListEnrolledStudentIDsByNameAndBirthday(
	ctx context.Context,
	tenantID int64,
	firstName, lastName, birthday string,
) ([]int64, error) {
	if err := peopledirectory.ValidateStudentMatch(tenantID, firstName, lastName, birthday); err != nil {
		return nil, err
	}

	ids, err := s.students.ListEnrolledIDsByNameAndBirthday(ctx, tenantID, firstName, lastName, birthday)
	return ids, MapError(err)
}

func (s *Service) ListAllStudentIDs(ctx context.Context) ([]int64, error) {
	ids, err := s.students.ListAllIDs(ctx)
	return ids, MapError(err)
}

func (s *Service) FindStudentRecordForMutationNoWait(ctx context.Context, studentID int64) (peopledirectory.StudentRecord, error) {
	if studentID <= 0 {
		return peopledirectory.StudentRecord{}, invalidStudent("student ID is required")
	}

	record, err := s.students.FindRecord(ctx, studentID, "UPDATE NOWAIT")
	return peopledirectory.StudentRecord(record), MapError(err)
}

func (s *Service) ListStudentRoster(ctx context.Context, today string) ([]peopledirectory.StudentRosterEntry, error) {
	if err := peopledirectory.ValidateRosterDay(today); err != nil {
		return nil, err
	}

	values, err := s.students.ListRoster(ctx, today)
	return toPublicRoster(values), MapError(err)
}

func (s *Service) ListStudentRosterByGroup(ctx context.Context, groupIDs []int64, today string) ([]peopledirectory.StudentRosterEntry, error) {
	groupIDs = peopledirectory.PositiveIDs(groupIDs)
	if len(groupIDs) == 0 {
		return []peopledirectory.StudentRosterEntry{}, nil
	}
	if err := peopledirectory.ValidateRosterDay(today); err != nil {
		return nil, err
	}

	values, err := s.students.ListRosterByGroups(ctx, groupIDs, today)
	return toPublicRoster(values), MapError(err)
}

func (s *Service) ListStudentRosterOverlapping(ctx context.Context, from, to, today string) ([]peopledirectory.StudentRosterEntry, error) {
	for _, day := range []string{from, to, today} {
		if err := peopledirectory.ValidateRosterDay(day); err != nil {
			return nil, err
		}
	}

	values, err := s.students.ListRosterOverlapping(ctx, from, to, today)
	return toPublicRoster(values), MapError(err)
}
