package postgres

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/studentdirectoryview"
)

func (s *StudentStore) ListByIDs(ctx context.Context, ids []int64) ([]domain.Student, domain.OperationStats, error) {
	records, stats, err := s.reads.ListByIDs(ctx, ids)
	result := make([]domain.Student, 0, len(records))
	for _, row := range records {
		result = append(result, domain.Student(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListNamesByIDs(ctx context.Context, ids []int64) ([]domain.StudentName, domain.OperationStats, error) {
	records, stats, err := s.reads.ListNamesByIDs(ctx, ids)
	result := make([]domain.StudentName, 0, len(records))
	for _, row := range records {
		result = append(result, domain.StudentName(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListByClasses(ctx context.Context, classes []string) ([]domain.Student, domain.OperationStats, error) {
	records, stats, err := s.reads.ListByClasses(ctx, classes)
	result := make([]domain.Student, 0, len(records))
	for _, row := range records {
		result = append(result, domain.Student(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListByPersonIDs(ctx context.Context, personIDs []int64) ([]domain.Student, domain.OperationStats, error) {
	records, stats, err := s.reads.ListByPersonIDs(ctx, personIDs)
	result := make([]domain.Student, 0, len(records))
	for _, row := range records {
		result = append(result, domain.Student(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListEnrolled(ctx context.Context) ([]domain.Student, domain.OperationStats, error) {
	records, stats, err := s.reads.ListEnrolled(ctx)
	result := make([]domain.Student, 0, len(records))
	for _, row := range records {
		result = append(result, domain.Student(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListClasses(ctx context.Context) ([]string, domain.OperationStats, error) {
	records, stats, err := s.reads.ListClasses(ctx)
	return records, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListByStatusFlag(ctx context.Context, status string) ([]domain.Student, domain.OperationStats, error) {
	records, stats, err := s.reads.ListByStatusFlag(ctx, status)
	result := make([]domain.Student, 0, len(records))
	for _, row := range records {
		result = append(result, domain.Student(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListDirectory(
	ctx context.Context,
	filter domain.StudentDirectoryFilter,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	records, stats, err := s.reads.ListDirectory(ctx, studentdirectoryview.Filter(filter))
	result := make([]domain.StudentRecord, 0, len(records))
	for _, row := range records {
		result = append(result, domain.StudentRecord(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) CountDirectory(
	ctx context.Context,
	filter domain.StudentDirectoryFilter,
) (int, domain.OperationStats, error) {
	records, stats, err := s.reads.CountDirectory(ctx, studentdirectoryview.Filter(filter))
	return records, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListDirectoryIDs(ctx context.Context) ([]int64, domain.OperationStats, error) {
	records, stats, err := s.reads.ListDirectoryIDs(ctx)
	return records, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) FindRecord(
	ctx context.Context,
	studentID int64,
	lock string,
) (domain.StudentRecord, bool, domain.OperationStats, error) {
	row, found, stats, err := s.reads.FindRecord(ctx, studentID, lock)
	return domain.StudentRecord(row), found, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListRecordsByIDs(
	ctx context.Context,
	ids []int64,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	records, stats, err := s.reads.ListRecordsByIDs(ctx, ids)
	result := make([]domain.StudentRecord, 0, len(records))
	for _, row := range records {
		result = append(result, domain.StudentRecord(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListRecordsByPersonIDs(
	ctx context.Context,
	personIDs []int64,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	records, stats, err := s.reads.ListRecordsByPersonIDs(ctx, personIDs)
	result := make([]domain.StudentRecord, 0, len(records))
	for _, row := range records {
		result = append(result, domain.StudentRecord(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListRecordsByGroups(
	ctx context.Context,
	groupIDs []int64,
	scope string,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	records, stats, err := s.reads.ListRecordsByGroups(ctx, groupIDs, scope)
	result := make([]domain.StudentRecord, 0, len(records))
	for _, row := range records {
		result = append(result, domain.StudentRecord(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListRecords(
	ctx context.Context,
	scope string,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	records, stats, err := s.reads.ListRecords(ctx, scope)
	result := make([]domain.StudentRecord, 0, len(records))
	for _, row := range records {
		result = append(result, domain.StudentRecord(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListRecordsByClasses(
	ctx context.Context,
	classes []string,
	scope string,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	records, stats, err := s.reads.ListRecordsByClasses(ctx, classes, scope)
	result := make([]domain.StudentRecord, 0, len(records))
	for _, row := range records {
		result = append(result, domain.StudentRecord(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListRecordsDueForStatus(
	ctx context.Context,
	status string,
	boundColumn string,
	asOf string,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	records, stats, err := s.reads.ListRecordsDueForStatus(ctx, status, boundColumn, asOf)
	result := make([]domain.StudentRecord, 0, len(records))
	for _, row := range records {
		result = append(result, domain.StudentRecord(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) LockRecordsByIDs(
	ctx context.Context,
	ids []int64,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	records, stats, err := s.reads.LockRecordsByIDs(ctx, ids)
	result := make([]domain.StudentRecord, 0, len(records))
	for _, row := range records {
		result = append(result, domain.StudentRecord(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) CountByGroups(
	ctx context.Context,
	groupIDs []int64,
) (map[int64]int, domain.OperationStats, error) {
	records, stats, err := s.reads.CountByGroups(ctx, groupIDs)
	return records, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListEnrolledIDsByNameAndBirthday(
	ctx context.Context,
	tenantID int64,
	firstName, lastName, birthday string,
) ([]int64, domain.OperationStats, error) {
	records, stats, err := s.reads.ListEnrolledIDsByNameAndBirthday(ctx, tenantID, firstName, lastName, birthday)
	return records, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListAllIDs(ctx context.Context) ([]int64, domain.OperationStats, error) {
	records, stats, err := s.reads.ListAllIDs(ctx)
	return records, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListRosterByGroups(
	ctx context.Context,
	groupIDs []int64,
	today string,
) ([]domain.StudentRosterEntry, domain.OperationStats, error) {
	records, stats, err := s.reads.ListRosterByGroups(ctx, groupIDs, today)
	result := make([]domain.StudentRosterEntry, 0, len(records))
	for _, row := range records {
		result = append(result, domain.StudentRosterEntry{Record: domain.StudentRecord(row.Record), FirstName: row.FirstName, LastName: row.LastName, TagID: row.TagID, AccountID: row.AccountID})
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListRoster(
	ctx context.Context,
	today string,
) ([]domain.StudentRosterEntry, domain.OperationStats, error) {
	records, stats, err := s.reads.ListRoster(ctx, today)
	result := make([]domain.StudentRosterEntry, 0, len(records))
	for _, row := range records {
		result = append(result, domain.StudentRosterEntry{Record: domain.StudentRecord(row.Record), FirstName: row.FirstName, LastName: row.LastName, TagID: row.TagID, AccountID: row.AccountID})
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListRosterOverlapping(
	ctx context.Context,
	from, to, today string,
) ([]domain.StudentRosterEntry, domain.OperationStats, error) {
	records, stats, err := s.reads.ListRosterOverlapping(ctx, from, to, today)
	result := make([]domain.StudentRosterEntry, 0, len(records))
	for _, row := range records {
		result = append(result, domain.StudentRosterEntry{Record: domain.StudentRecord(row.Record), FirstName: row.FirstName, LastName: row.LastName, TagID: row.TagID, AccountID: row.AccountID})
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func projectionError(err error) error {
	if errors.Is(err, studentdirectoryview.ErrStudentLockBusy) {
		return domain.ErrStudentLockBusy
	}
	if errors.Is(err, studentdirectoryview.ErrStudentNotFound) {
		return domain.ErrStudentNotFound
	}
	return err
}
