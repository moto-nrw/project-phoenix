package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *StudentStore) ListByIDs(ctx context.Context, ids []int64) ([]directory.Student, domain.OperationStats, error) {
	records, stats, err := s.reads.ListByIDs(ctx, ids)
	result := make([]directory.Student, 0, len(records))
	for _, row := range records {
		result = append(result, directory.Student(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListNamesByIDs(ctx context.Context, ids []int64) ([]directory.StudentName, domain.OperationStats, error) {
	records, stats, err := s.reads.ListNamesByIDs(ctx, ids)
	result := make([]directory.StudentName, 0, len(records))
	for _, row := range records {
		result = append(result, directory.StudentName(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListByClasses(ctx context.Context, classes []string) ([]directory.Student, domain.OperationStats, error) {
	records, stats, err := s.reads.ListByClasses(ctx, classes)
	result := make([]directory.Student, 0, len(records))
	for _, row := range records {
		result = append(result, directory.Student(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListByPersonIDs(ctx context.Context, personIDs []int64) ([]directory.Student, domain.OperationStats, error) {
	records, stats, err := s.reads.ListByPersonIDs(ctx, personIDs)
	result := make([]directory.Student, 0, len(records))
	for _, row := range records {
		result = append(result, directory.Student(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListEnrolled(ctx context.Context) ([]directory.Student, domain.OperationStats, error) {
	records, stats, err := s.reads.ListEnrolled(ctx)
	result := make([]directory.Student, 0, len(records))
	for _, row := range records {
		result = append(result, directory.Student(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListClasses(ctx context.Context) ([]string, domain.OperationStats, error) {
	records, stats, err := s.reads.ListClasses(ctx)
	return records, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListByStatusFlag(ctx context.Context, status string) ([]directory.Student, domain.OperationStats, error) {
	records, stats, err := s.reads.ListByStatusFlag(ctx, status)
	result := make([]directory.Student, 0, len(records))
	for _, row := range records {
		result = append(result, directory.Student(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func (s *StudentStore) ListDirectory(
	ctx context.Context,
	filter domain.StudentDirectoryFilter,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	records, stats, err := s.reads.ListDirectory(ctx, directoryFilter(filter))
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
	records, stats, err := s.reads.CountDirectory(ctx, directoryFilter(filter))
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
		result = append(result, toDomainRosterEntry(row))
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
		result = append(result, toDomainRosterEntry(row))
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
		result = append(result, toDomainRosterEntry(row))
	}
	return result, domain.OperationStats(stats), projectionError(err)
}

func projectionError(err error) error {
	if errors.Is(err, directory.ErrStudentLockBusy) {
		return domain.ErrStudentLockBusy
	}
	if errors.Is(err, directory.ErrStudentNotFound) {
		return domain.ErrStudentNotFound
	}
	return err
}

func toDomainRosterEntry(row directory.StudentRosterEntry) domain.StudentRosterEntry {
	// DirectoryRecord is write-oriented; roster reads also retain the stored
	// timestamps and the hydrated departure plan.
	record := domain.StudentRecord(row.Record.DirectoryRecord())
	record.CreatedAt, record.UpdatedAt = row.Record.CreatedAt, row.Record.UpdatedAt
	record.DepartureDays = row.Record.DepartureDays
	record.AllowedDepartureModes = row.Record.AllowedDepartureModes
	record.PickupDays, record.BusDays = row.Record.PickupDays, row.Record.BusDays
	return domain.StudentRosterEntry{Record: record, FirstName: row.FirstName, LastName: row.LastName, TagID: row.TagID, AccountID: row.AccountID}
}

// LockRecordByPersonID keeps the class-writes gate ahead of the row lock,
// using the owning gate query just like enrollment creation does.
func (s *StudentStore) LockRecordByPersonID(ctx context.Context, personID int64) (domain.StudentRecord, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.StudentRecord{}, false, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(scope.TenantID); err != nil {
		return domain.StudentRecord{}, false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	started := time.Now()
	var gate *bun.SelectQuery
	if s.classGateQuery != nil {
		gate, err = s.classGateQuery(ctx)
	} else {
		err = s.classGate(ctx, false)
		stats.Queries++
		gate = db.NewSelect().ColumnExpr("TRUE")
	}
	if err != nil {
		return domain.StudentRecord{}, false, stats, err
	}
	rows := []directoryRecordRow{}
	query := withStudentTenant(s.reads.studentRows(db, scope).ColumnExpr(studentRecordColumns).Where(`"student".person_id = ?`, personID), scope).
		Where("(WITH class_write_gate AS MATERIALIZED (?) SELECT count(*) FROM class_write_gate) = 1", gate).
		For("UPDATE OF student")
	stats.Queries++
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StudentRecord{}, false, stats, fmt.Errorf("people directory postgres: lock student by person: %w", err)
	}
	stats.Rows = int64(len(rows))
	if len(rows) == 0 {
		return domain.StudentRecord{}, false, stats, nil
	}
	return domain.StudentRecord(rows[0].toDomain()), true, stats, nil
}
