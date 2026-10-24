package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/uptrace/bun"
)

// selectStudentRecords is the one shape every whole-row read of the child table
// takes: the owned columns, tenant-scoped, ordered by id so two pages of one
// selection never overlap.
func (s *studentDirectoryReads) selectStudentRecords(
	ctx context.Context,
	operation string,
	narrow func(*bun.SelectQuery) *bun.SelectQuery,
) ([]directory.StudentRecord, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	rows := []directoryRecordRow{}
	query := withStudentTenant(s.studentRows(db, scope).
		ColumnExpr(studentRecordColumns), scope)
	query = narrow(query).OrderExpr(`"student".id ASC`)

	stats := directoryStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: %s: %w", operation, err)
	}
	stats.Rows = int64(len(rows))
	result := make([]directory.StudentRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, stats, nil
}

// inScope applies the roster guard unless the caller asked for every child: a
// graduate's row survives only so a grade transition can be reverted, never to
// be written to or counted by a staff-facing read.
func inScope(query *bun.SelectQuery, scope string) *bun.SelectQuery {
	if scope == string(directory.StudentScopeAll) {
		return query
	}
	return query.Where(`"student".status <> ?`, directory.StudentStatusAlumnus)
}

// ListRecordsByPersonIDs resolves the children of the given identities, alumni
// included, which is how the data import finds the row a matched person owns.
func (s *studentDirectoryReads) ListRecordsByPersonIDs(
	ctx context.Context,
	personIDs []int64,
) ([]directory.StudentRecord, directoryStats, error) {
	return s.selectStudentRecords(ctx, "list student records by person", func(query *bun.SelectQuery) *bun.SelectQuery {
		return query.Where(`"student".person_id IN (?)`, bun.List(personIDs))
	})
}

func (s *studentDirectoryReads) ListRecordsByGroups(
	ctx context.Context,
	groupIDs []int64,
	scope string,
) ([]directory.StudentRecord, directoryStats, error) {
	return s.selectStudentRecords(ctx, "list student records by group", func(query *bun.SelectQuery) *bun.SelectQuery {
		return inScope(query.Where(`"student".group_id IN (?)`, bun.List(groupIDs)), scope)
	})
}

// ListRecords returns every child of the tenant in scope.
func (s *studentDirectoryReads) ListRecords(
	ctx context.Context,
	scope string,
) ([]directory.StudentRecord, directoryStats, error) {
	return s.selectStudentRecords(ctx, "list student records", func(query *bun.SelectQuery) *bun.SelectQuery {
		return inScope(query, scope)
	})
}

func (s *studentDirectoryReads) ListRecordsByClasses(
	ctx context.Context,
	classes []string,
	scope string,
) ([]directory.StudentRecord, directoryStats, error) {
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
func (s *studentDirectoryReads) ListRecordsDueForStatus(
	ctx context.Context,
	status string,
	boundColumn string,
	asOf string,
) ([]directory.StudentRecord, directoryStats, error) {
	return s.selectStudentRecords(ctx, "list student records due for status", func(query *bun.SelectQuery) *bun.SelectQuery {
		query = query.Where(`"student".status = ?`, status)
		// Spelled out per bound rather than built from a column name: the
		// predicate stays readable, and a static reader can still see which
		// column it touches.
		if boundColumn == directory.StudentBoundCareEnd {
			return query.
				Where(`"student".enrolled_until IS NOT NULL`).
				Where(`"student".enrolled_until <= ?`, directoryDateParam(asOf))
		}
		return query.
			Where(`"student".enrolled_from IS NOT NULL`).
			Where(`"student".enrolled_from <= ?`, directoryDateParam(asOf))
	})
}

// LockRecordsByIDs reads and locks the given rows in one statement, ascending
// by id — the project-wide student lock order, so two batch writers serialize
// on their first shared row instead of deadlocking.
func (s *studentDirectoryReads) LockRecordsByIDs(
	ctx context.Context,
	ids []int64,
) ([]directory.StudentRecord, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	rows := []directoryRecordRow{}
	query := withStudentTenant(s.studentRows(db, scope).
		ColumnExpr(studentRecordColumns).
		Where(`"student".id IN (?)`, bun.List(ids)), scope).
		OrderExpr(`"student".id ASC`).
		For("UPDATE")

	stats := directoryStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: lock student records: %w", err)
	}
	stats.Rows = int64(len(rows))
	result := make([]directory.StudentRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, stats, nil
}

// CountByGroups counts the non-alumni children of each group.
func (s *studentDirectoryReads) CountByGroups(
	ctx context.Context,
	groupIDs []int64,
) (map[int64]int, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	var rows []struct {
		GroupID int64 `bun:"group_id"`
		Count   int   `bun:"count"`
	}
	query := withStudentTenant(s.studentRows(db, scope).
		ColumnExpr(`"student".group_id`).
		ColumnExpr(`COUNT(*) AS count`).
		Where(`"student".group_id IN (?)`, bun.List(groupIDs)).
		Where(`"student".status <> ?`, directory.StudentStatusAlumnus).
		GroupExpr(`"student".group_id`), scope)

	stats := directoryStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: count students by group: %w", err)
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
func (s *studentDirectoryReads) ListEnrolledIDsByNameAndBirthday(
	ctx context.Context,
	tenantID int64,
	firstName, lastName, birthday string,
) ([]int64, directoryStats, error) {
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	ids := []int64{}
	query := s.rows.StudentRows(db, tenantID).
		ColumnExpr(`"student".id`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Where(`"student".tenant_id = ?`, tenantID).
		Where(`"student".status IN (?)`, bun.List([]string{directory.StudentStatusActive, directory.StudentStatusPending})).
		Where(`LOWER(TRIM("person".first_name)) = LOWER(TRIM(?))`, strings.TrimSpace(firstName)).
		Where(`LOWER(TRIM("person".last_name)) = LOWER(TRIM(?))`, strings.TrimSpace(lastName)).
		Where(`"person".birthday = ?`, directoryDateParam(birthday)).
		Where(`"person".deleted_at IS NULL`).
		OrderExpr(`"student".id ASC`)

	stats := directoryStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &ids)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: list enrolled students by name: %w", err)
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
func (s *studentDirectoryReads) ListAllIDs(ctx context.Context) ([]int64, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	ids := []int64{}
	query := withStudentTenant(s.studentRows(db, scope).
		ColumnExpr(`"student".id`), scope).
		OrderExpr(`"student".id ASC`)

	stats := directoryStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &ids)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: list all student ids: %w", err)
	}
	stats.Rows = int64(len(ids))
	return ids, stats, nil
}
