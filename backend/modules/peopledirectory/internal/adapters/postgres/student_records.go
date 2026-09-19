package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
)

// selectStudentRecords is the one shape every whole-row read of the child table
// takes: the owned columns, tenant-scoped, ordered by id so two pages of one
// selection never overlap.
func (s *StudentStore) selectStudentRecords(
	ctx context.Context,
	operation string,
	narrow func(*bun.SelectQuery) *bun.SelectQuery,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []studentRecordRow{}
	query := withStudentTenant(db.NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(studentRecordColumns), tenantID)
	query = narrow(query).OrderExpr(`"student".id ASC`)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: %s: %w", operation, err)
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.StudentRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, stats, nil
}

// inScope applies the roster guard unless the caller asked for every child: a
// graduate's row survives only so a grade transition can be reverted, never to
// be written to or counted by a staff-facing read.
func inScope(query *bun.SelectQuery, scope string) *bun.SelectQuery {
	if scope == domain.StudentScopeAll {
		return query
	}
	return query.Where(`"student".status <> ?`, domain.StudentStatusAlumnus)
}

// ListRecordsByPersonIDs resolves the children of the given identities, alumni
// included, which is how the data import finds the row a matched person owns.
func (s *StudentStore) ListRecordsByPersonIDs(
	ctx context.Context,
	personIDs []int64,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	return s.selectStudentRecords(ctx, "list student records by person", func(query *bun.SelectQuery) *bun.SelectQuery {
		return query.Where(`"student".person_id IN (?)`, bun.List(personIDs))
	})
}

func (s *StudentStore) ListRecordsByGroups(
	ctx context.Context,
	groupIDs []int64,
	scope string,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	return s.selectStudentRecords(ctx, "list student records by group", func(query *bun.SelectQuery) *bun.SelectQuery {
		return inScope(query.Where(`"student".group_id IN (?)`, bun.List(groupIDs)), scope)
	})
}

// ListRecords returns every child of the tenant in scope.
func (s *StudentStore) ListRecords(
	ctx context.Context,
	scope string,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	return s.selectStudentRecords(ctx, "list student records", func(query *bun.SelectQuery) *bun.SelectQuery {
		return inScope(query, scope)
	})
}

func (s *StudentStore) ListRecordsByClasses(
	ctx context.Context,
	classes []string,
	scope string,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	return s.selectStudentRecords(ctx, "list student records by class", func(query *bun.SelectQuery) *bun.SelectQuery {
		// Trimmed and case-insensitive, like the directory page: the column is
		// free text a school typed, so " 2a " and "2A" are the same class.
		return inScope(query.Where(
			`LOWER(TRIM("student".school_class)) IN (?)`, bun.List(lowerTrimmed(classes))), scope)
	})
}

// ListRecordsDueForStatus drives the two halves of the activate-students tick:
// the children whose start day has arrived, and those whose last care day has
// passed.
func (s *StudentStore) ListRecordsDueForStatus(
	ctx context.Context,
	status string,
	boundColumn string,
	asOf string,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	return s.selectStudentRecords(ctx, "list student records due for status", func(query *bun.SelectQuery) *bun.SelectQuery {
		query = query.Where(`"student".status = ?`, status)
		// Spelled out per bound rather than built from a column name: the
		// predicate stays readable, and a static reader can still see which
		// column it touches.
		if boundColumn == domain.StudentBoundCareEnd {
			return query.
				Where(`"student".enrolled_until IS NOT NULL`).
				Where(`"student".enrolled_until <= ?`, studentDateParam(asOf))
		}
		return query.
			Where(`"student".enrolled_from IS NOT NULL`).
			Where(`"student".enrolled_from <= ?`, studentDateParam(asOf))
	})
}

// LockRecordsByIDs reads and locks the given rows in one statement, ascending
// by id — the project-wide student lock order, so two batch writers serialize
// on their first shared row instead of deadlocking.
func (s *StudentStore) LockRecordsByIDs(
	ctx context.Context,
	ids []int64,
) ([]domain.StudentRecord, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []studentRecordRow{}
	query := withStudentTenant(db.NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(studentRecordColumns).
		Where(`"student".id IN (?)`, bun.List(ids)), tenantID).
		OrderExpr(`"student".id ASC`).
		For("UPDATE")

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: lock student records: %w", err)
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.StudentRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, stats, nil
}

// CountByGroups counts the non-alumni children of each group.
func (s *StudentStore) CountByGroups(
	ctx context.Context,
	groupIDs []int64,
) (map[int64]int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		GroupID int64 `bun:"group_id"`
		Count   int   `bun:"count"`
	}
	query := withStudentTenant(db.NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".group_id`).
		ColumnExpr(`COUNT(*) AS count`).
		Where(`"student".group_id IN (?)`, bun.List(groupIDs)).
		Where(`"student".status <> ?`, domain.StudentStatusAlumnus).
		GroupExpr(`"student".group_id`), tenantID)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: count students by group: %w", err)
	}
	stats.Rows = int64(len(rows))
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.GroupID] = row.Count
	}
	return counts, stats, nil
}

// ListEnrolledIDsByNameAndBirthday resolves the children already enrolled under
// a name and birthday, matched case- and whitespace-insensitively.
//
// The tenant is explicit rather than taken from the transaction: the parent
// submit path runs in an admin transaction, where row-level security would not
// scope this for it.
func (s *StudentStore) ListEnrolledIDsByNameAndBirthday(
	ctx context.Context,
	tenantID int64,
	firstName, lastName, birthday string,
) ([]int64, domain.OperationStats, error) {
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	ids := []int64{}
	query := db.NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Where(`"student".tenant_id = ?`, tenantID).
		Where(`"student".status IN (?)`, bun.List([]string{domain.StudentStatusActive, domain.StudentStatusPending})).
		Where(`LOWER(TRIM("person".first_name)) = LOWER(TRIM(?))`, strings.TrimSpace(firstName)).
		Where(`LOWER(TRIM("person".last_name)) = LOWER(TRIM(?))`, strings.TrimSpace(lastName)).
		Where(`"person".birthday = ?`, studentDateParam(birthday)).
		Where(`"person".deleted_at IS NULL`).
		OrderExpr(`"student".id ASC`)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &ids)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: list enrolled students by name: %w", err)
	}
	stats.Rows = int64(len(ids))
	return ids, stats, nil
}

func lowerTrimmed(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strings.ToLower(strings.TrimSpace(value)))
	}
	return out
}

// ListAllIDs returns every child of the tenant, alumni included. A graduate
// who is actually present today still has to be reachable, so this deliberately
// applies no lifecycle filter — unlike the staff directory's own id list.
func (s *StudentStore) ListAllIDs(ctx context.Context) ([]int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	ids := []int64{}
	query := withStudentTenant(db.NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id`), tenantID).
		OrderExpr(`"student".id ASC`)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &ids)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: list all student ids: %w", err)
	}
	stats.Rows = int64(len(ids))
	return ids, stats, nil
}
