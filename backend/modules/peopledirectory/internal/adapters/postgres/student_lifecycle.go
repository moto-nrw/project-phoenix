package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres/calendar"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
)

// SetStatus changes the lifecycle status of one child. It reports false when
// the tenant has no such row.
func (s *StudentStore) SetStatus(
	ctx context.Context,
	studentID int64,
	status string,
) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withStudentTenant(db.NewUpdate().
		TableExpr(`users.students AS "student"`).
		Set(`status = ?`, status).
		Set(`updated_at = NOW()`).
		Where(`"student".id = ?`, studentID), tenantID)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: set student status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: set student status: %w", err)
	}
	stats.Rows = affected
	return affected == 1, stats, nil
}

// TransitionStatus changes a child's status only while the stored one still
// matches expected, reporting false when another writer moved it first.
//
// An unconditional update by id would resurrect a child whose status changed in
// the meantime: a grade transition commits "alumnus", the waiting update takes
// the row lock and replaces it with "active", and a departed child is back in
// every staff list past all the alumni read filters. Comparing against the
// status the caller actually saw makes that a no-op instead.
func (s *StudentStore) TransitionStatus(
	ctx context.Context,
	studentID int64,
	expected, next string,
) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withStudentTenant(db.NewUpdate().
		TableExpr(`users.students AS "student"`).
		Set(`status = ?`, next).
		Set(`updated_at = NOW()`).
		Where(`"student".id = ?`, studentID).
		Where(`"student".status = ?`, expected), tenantID)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: transition student status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: transition student status: %w", err)
	}
	stats.Rows = affected
	return affected == 1, stats, nil
}

// SetCareEnd writes the enrolment interval's inclusive upper bound for a batch
// of children in one statement, and returns how many rows it moved.
func (s *StudentStore) SetCareEnd(
	ctx context.Context,
	ids []int64,
	until string,
) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withStudentTenant(db.NewUpdate().
		TableExpr(`users.students AS "student"`).
		Set(`enrolled_until = ?`, studentDateParam(until)).
		Set(`updated_at = NOW()`).
		Where(`"student".id IN (?)`, bun.List(ids)), tenantID)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("people directory postgres: set student care end: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("people directory postgres: set student care end: %w", err)
	}
	stats.Rows = affected
	return affected, stats, nil
}

// ReopenCare gives one child a new start day, clears the end day and writes the
// lifecycle status the caller derived for today.
func (s *StudentStore) ReopenCare(
	ctx context.Context,
	studentID int64,
	from string,
	status string,
) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withStudentTenant(db.NewUpdate().
		TableExpr(`users.students AS "student"`).
		Set(`enrolled_from = ?`, studentDateParam(from)).
		Set(`enrolled_until = NULL`).
		Set(`status = ?`, status).
		Set(`updated_at = NOW()`).
		Where(`"student".id = ?`, studentID), tenantID)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: reopen student care: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: reopen student care: %w", err)
	}
	stats.Rows = affected
	return affected == 1, stats, nil
}

// ListCareEnds projects the enrolment interval's upper bound for the given
// children; a child without one is absent from the result.
func (s *StudentStore) ListCareEnds(
	ctx context.Context,
	ids []int64,
) (map[int64]string, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		ID            int64          `bun:"id"`
		EnrolledUntil *calendar.Date `bun:"enrolled_until"`
	}
	query := withStudentTenant(db.NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id, "student".enrolled_until`).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Where(`"student".enrolled_until IS NOT NULL`), tenantID)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: list student care ends: %w", err)
	}
	stats.Rows = int64(len(rows))
	bounds := make(map[int64]string, len(rows))
	for _, row := range rows {
		if row.EnrolledUntil != nil {
			bounds[row.ID] = row.EnrolledUntil.String()
		}
	}
	return bounds, stats, nil
}
