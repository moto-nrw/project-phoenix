package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// readRecords is the shared shape of every whole-row read: one statement in a
// read transaction, observed under its own operation name.
func (s *StudentService) readRecords(
	ctx context.Context,
	operation string,
	read func(context.Context) ([]domain.StudentRecord, domain.OperationStats, error),
) (result []domain.StudentRecord, err error) {
	err = s.run(ctx, operation, s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = read(txCtx)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *StudentService) ListRecordsByPersonIDs(ctx context.Context, personIDs []int64) ([]domain.StudentRecord, error) {
	return s.readRecords(ctx, "list_student_records_by_person", func(txCtx context.Context) ([]domain.StudentRecord, domain.OperationStats, error) {
		return s.store.ListRecordsByPersonIDs(txCtx, personIDs)
	})
}

func (s *StudentService) ListRecordsByGroups(ctx context.Context, groupIDs []int64, scope string) ([]domain.StudentRecord, error) {
	return s.readRecords(ctx, "list_student_records_by_group", func(txCtx context.Context) ([]domain.StudentRecord, domain.OperationStats, error) {
		return s.store.ListRecordsByGroups(txCtx, groupIDs, scope)
	})
}

func (s *StudentService) ListRecords(ctx context.Context, scope string) ([]domain.StudentRecord, error) {
	return s.readRecords(ctx, "list_student_records", func(txCtx context.Context) ([]domain.StudentRecord, domain.OperationStats, error) {
		return s.store.ListRecords(txCtx, scope)
	})
}

func (s *StudentService) ListRecordsByClasses(ctx context.Context, classes []string, scope string) ([]domain.StudentRecord, error) {
	return s.readRecords(ctx, "list_student_records_by_class", func(txCtx context.Context) ([]domain.StudentRecord, domain.OperationStats, error) {
		return s.store.ListRecordsByClasses(txCtx, classes, scope)
	})
}

func (s *StudentService) ListRecordsDueForStatus(ctx context.Context, status, bound, asOf string) ([]domain.StudentRecord, error) {
	return s.readRecords(ctx, "list_student_records_due_for_status", func(txCtx context.Context) ([]domain.StudentRecord, domain.OperationStats, error) {
		return s.store.ListRecordsDueForStatus(txCtx, status, bound, asOf)
	})
}

// LockRecordsByIDs takes the rows a batch writer is about to change, so it runs
// in a write transaction: the locks have to outlive this call.
func (s *StudentService) LockRecordsByIDs(ctx context.Context, ids []int64) (result []domain.StudentRecord, err error) {
	err = s.run(ctx, "lock_student_records_by_id", func(ctx context.Context, fn func(context.Context) error) error {
		if err := s.tx.RequireTransaction(ctx); err != nil {
			return err
		}
		return fn(ctx)
	}, func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, err := s.store.LockEnrollmentClassWrites(txCtx)
		stats.Add(gateStats)
		if err != nil {
			return err
		}
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.LockRecordsByIDs(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *StudentService) CountByGroups(ctx context.Context, groupIDs []int64) (counts map[int64]int, err error) {
	err = s.run(ctx, "count_students_by_group", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		counts, queryStats, err = s.store.CountByGroups(txCtx, groupIDs)
		stats.Add(queryStats)
		return err
	})
	return counts, err
}

// ListEnrolledIDsByNameAndBirthday resolves eligibility in the school identified
// by the parent submission bootstrap. A conflicting caller school is rejected.
func (s *StudentService) ListEnrolledIDsByNameAndBirthday(
	ctx context.Context,
	tenantID int64,
	firstName, lastName, birthday string,
) (ids []int64, err error) {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		return nil, err
	}
	if current, currentErr := tenant.TenantFromContext(ctx); currentErr == nil && current != id {
		return nil, fmt.Errorf("people directory: enrollment lookup tenant does not match caller")
	}
	ctx = tenant.WithTenant(ctx, id)
	err = s.run(ctx, "list_enrolled_students_by_name", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		ids, queryStats, err = s.store.ListEnrolledIDsByNameAndBirthday(txCtx, tenantID, firstName, lastName, birthday)
		stats.Add(queryStats)
		return err
	})
	return ids, err
}

// ListAllIDs returns every child of the tenant, alumni included.
func (s *StudentService) ListAllIDs(ctx context.Context) (ids []int64, err error) {
	err = s.run(ctx, "list_all_student_ids", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		ids, queryStats, err = s.store.ListAllIDs(txCtx)
		stats.Add(queryStats)
		return err
	})
	return ids, err
}

func (s *StudentService) readRoster(
	ctx context.Context,
	operation string,
	read func(context.Context) ([]domain.StudentRosterEntry, domain.OperationStats, error),
) (result []domain.StudentRosterEntry, err error) {
	err = s.run(ctx, operation, s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = read(txCtx)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *StudentService) ListRosterByGroups(ctx context.Context, groupIDs []int64, today string) ([]domain.StudentRosterEntry, error) {
	return s.readRoster(ctx, "list_student_roster_by_group", func(txCtx context.Context) ([]domain.StudentRosterEntry, domain.OperationStats, error) {
		return s.store.ListRosterByGroups(txCtx, groupIDs, today)
	})
}

func (s *StudentService) ListRoster(ctx context.Context, today string) ([]domain.StudentRosterEntry, error) {
	return s.readRoster(ctx, "list_student_roster", func(txCtx context.Context) ([]domain.StudentRosterEntry, domain.OperationStats, error) {
		return s.store.ListRoster(txCtx, today)
	})
}

func (s *StudentService) ListRosterOverlapping(ctx context.Context, from, to, today string) ([]domain.StudentRosterEntry, error) {
	return s.readRoster(ctx, "list_student_roster_overlapping", func(txCtx context.Context) ([]domain.StudentRosterEntry, domain.OperationStats, error) {
		return s.store.ListRosterOverlapping(txCtx, from, to, today)
	})
}
