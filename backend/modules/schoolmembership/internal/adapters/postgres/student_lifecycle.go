package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// firstCareFromNext is the first-care bound after a write that moves a child's
// status or start (#6041, ADR 0166): the earliest day the child can have been
// cared for at this school, NULL when unknown. ?0 is the application's today,
// ?1 the next status, ?2 the next start (SET reads the row before the write).
//
//   - Unknown stays unknown: a returning child keeps its past.
//   - A bound already reached may have seen care, so it only moves earlier.
//   - A bound still ahead saw no care: it follows the start, or becomes today
//     for an activation before the start or a child without a start.
const firstCareFromNext = `first_care_from = CASE
	WHEN "student".first_care_from IS NULL THEN NULL
	WHEN "student".first_care_from <= ?0::date THEN LEAST("student".first_care_from, ?2::date)
	WHEN ?1::text = 'active' OR ?2::date IS NULL THEN LEAST(?2::date, ?0::date)
	ELSE ?2::date END`

// careRestart is true when a write restarts an ended care with a gap (#6073,
// ADR 0177): the row has an end, and the next start lies more than a day
// after it. ?0 is the next start.
const careRestart = `("student".enrolled_until IS NOT NULL AND ?0::date IS NOT NULL
	AND ?0::date > "student".enrolled_until + 1)`

// carePausedAfterNext records the last care day before a restart; every
// other write keeps it. ?0 is the restart condition.
const carePausedAfterNext = `care_paused_after = CASE WHEN ?0 THEN "student".enrolled_until
	ELSE "student".care_paused_after END`

// careResumedFromNext is the end of the care gap. A restart sets it to the
// earliest day the child can be cared for again: the start, or today for an
// activation before it. Every other write keeps it like firstCareFromNext
// keeps the first-care bound. ?0 to ?2 as there, ?3 the restart condition.
const careResumedFromNext = `care_resumed_from = CASE
	WHEN ?3 THEN CASE WHEN ?1::text = 'active' THEN LEAST(?2::date, ?0::date) ELSE ?2::date END
	WHEN "student".care_resumed_from IS NULL THEN NULL
	WHEN "student".care_resumed_from <= ?0::date THEN LEAST("student".care_resumed_from, ?2::date)
	WHEN ?1::text = 'active' OR ?2::date IS NULL THEN LEAST(?2::date, ?0::date)
	ELSE ?2::date END`

// noCareRestart is the restart condition of writes that keep the start.
const noCareRestart = bun.Safe("FALSE")

// firstCareFromOnInsert is the bound of a new membership: its start, when the
// child is inserted pending with a start and has no other (also soft-deleted)
// membership at this school. ?0 is the status, ?1 the start, ?2 the tenant,
// ?3 the child.
const firstCareFromOnInsert = `CASE WHEN ?0::text = 'pending' AND NOT EXISTS (
	SELECT 1 FROM users.student_school_memberships AS other
	WHERE other.tenant_id = ?2 AND other.student_profile_id = ?3
) THEN ?1::date END`

// currentDay parses the application's today the first-care bound is kept on;
// without it the bound cannot be decided.
func currentDay(today string) (calendar.Date, error) {
	if today == "" {
		return "", errors.New("school membership postgres: current day is required")
	}
	day, err := optionalDate(today, "current day")
	if err != nil {
		return "", err
	}
	return *day, nil
}

func (s *Store) TransitionStudentStatus(ctx context.Context, id int64, expected, next, today string) (bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(scope.TenantID); err != nil {
		return false, domain.OperationStats{}, err
	}
	day, err := currentDay(today)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withStudentTenant(studentUpdate(db), scope).
		Set("status = ?", next).Set(firstCareFromNext, day, next, bun.Safe(`"student".enrolled_from`)).
		Set(careResumedFromNext, day, next, bun.Safe(`"student".enrolled_from`), noCareRestart).
		Where("student.student_profile_id = ?", id).
		Where("student.status <> 'alumnus'")
	if expected != "" {
		query = query.Where("student.status = ?", expected)
	}
	count, stats, err := execStudents(ctx, query, "transition student status")
	return count == 1, stats, err
}

func (s *Store) GraduateStudents(ctx context.Context, ids []int64) (int64, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(scope.TenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withStudentTenant(studentUpdate(db).
		Set(`status = ?`, "alumnus").
		Where(`"student".student_profile_id IN (?)`, bun.List(ids)).
		Where(`"student".status <> ?`, "alumnus"), scope)
	return execStudents(ctx, query, "graduate students by id")
}

func (s *Store) ReactivateStudents(ctx context.Context, ids []int64, status, today string) ([]int64, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(scope.TenantID); err != nil {
		return nil, domain.OperationStats{}, err
	}
	day, err := currentDay(today)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	type idRow struct {
		ID int64 `bun:"id"`
	}
	var rows []idRow
	query := withStudentTenant(studentUpdate(db).
		Set(`status = ?`, status).Set(firstCareFromNext, day, status, bun.Safe(`"student".enrolled_from`)).
		Set(careResumedFromNext, day, status, bun.Safe(`"student".enrolled_from`), noCareRestart).
		Where(`"student".student_profile_id IN (?)`, bun.List(ids)).
		Where(`"student".status = ?`, "alumnus"), scope).
		Returning(`"student".student_profile_id AS id`)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = query.Exec(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("school membership postgres: reactivate students: %w", err)
	}
	stats.Rows = int64(len(rows))
	result := make([]int64, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.ID)
	}
	return result, stats, nil
}

func execStudents(ctx context.Context, query *bun.UpdateQuery, operation string) (int64, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("school membership postgres: %s: %w", operation, err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("school membership postgres: %s: count rows: %w", operation, err)
	}
	return stats.Rows, stats, nil
}

func studentUpdate(db bun.IDB) *bun.UpdateQuery {
	return db.NewUpdate().TableExpr(`users.student_school_memberships AS "student"`).
		Set("updated_at = NOW()").Where("student.deleted_at IS NULL")
}

func (s *Store) EndStudentCare(ctx context.Context, ids []int64, until string) (int64, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(scope.TenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	day, err := optionalDate(until, "care end")
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withStudentTenant(studentUpdate(db), scope).
		Set("enrolled_until = ?", day).
		Where("student.student_profile_id IN (?)", bun.List(ids)).
		Where("student.status <> 'alumnus'")
	return execStudents(ctx, query, "end student care")
}

func (s *Store) ResumeStudentCare(ctx context.Context, id int64, from, status, on string) (bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(scope.TenantID); err != nil {
		return false, domain.OperationStats{}, err
	}
	start, err := optionalDate(from, "care start")
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	today, err := optionalDate(on, "current day")
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	restart := bun.SafeQuery(careRestart, start)
	query := withStudentTenant(studentUpdate(db), scope).
		Set("enrolled_from = ?", start).Set("enrolled_until = NULL").Set("status = ?", status).
		Set(firstCareFromNext, today, status, start).
		Set(carePausedAfterNext, restart).Set(careResumedFromNext, today, status, start, restart).
		Where("student.student_profile_id = ?", id).
		Where("student.status <> 'alumnus'").
		Where("student.enrolled_until < ?", today)
	changed, stats, err := execStudents(ctx, query, "resume student care")
	return changed == 1, stats, err
}
func withStudentTenant(query *bun.UpdateQuery, scope TenantScope) *bun.UpdateQuery {
	return query.Where("student.tenant_id = ?", scope.TenantID)
}
func requireStudentWriteTenant(tenantID int64) error {
	if tenantID <= 0 {
		return errors.New("school membership: tenant is required")
	}
	return nil
}
