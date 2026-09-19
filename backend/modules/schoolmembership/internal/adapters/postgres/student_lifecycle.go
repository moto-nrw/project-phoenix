package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) TransitionStudentStatus(ctx context.Context, id int64, expected, next string) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withStudentTenant(studentUpdate(db), tenantID).
		Set("status = ?", next).Where("student.student_profile_id = ?", id).
		Where("student.status <> 'alumnus'")
	if expected != "" {
		query = query.Where("student.status = ?", expected)
	}
	count, stats, err := execStudents(ctx, query, "transition student status")
	return count == 1, stats, err
}

func (s *Store) GraduateStudents(ctx context.Context, ids []int64) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withStudentTenant(studentUpdate(db).
		Set(`status = ?`, "alumnus").
		Where(`"student".student_profile_id IN (?)`, bun.List(ids)).
		Where(`"student".status <> ?`, "alumnus"), tenantID)
	return execStudents(ctx, query, "graduate students by id")
}

func (s *Store) ReactivateStudents(ctx context.Context, ids []int64, status string) ([]int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return nil, domain.OperationStats{}, err
	}
	type idRow struct {
		ID int64 `bun:"id"`
	}
	var rows []idRow
	query := withStudentTenant(studentUpdate(db).
		Set(`status = ?`, status).
		Where(`"student".student_profile_id IN (?)`, bun.List(ids)).
		Where(`"student".status = ?`, "alumnus"), tenantID).
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
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	day, err := optionalDate(until, "care end")
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withStudentTenant(studentUpdate(db), tenantID).
		Set("enrolled_until = ?", day).
		Where("student.student_profile_id IN (?)", bun.List(ids)).
		Where("student.status <> 'alumnus'")
	return execStudents(ctx, query, "end student care")
}

func (s *Store) ResumeStudentCare(ctx context.Context, id int64, from, status, on string) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
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
	query := withStudentTenant(studentUpdate(db), tenantID).
		Set("enrolled_from = ?", start).Set("enrolled_until = NULL").Set("status = ?", status).
		Where("student.student_profile_id = ?", id).
		Where("student.status <> 'alumnus'").
		Where("student.enrolled_until < ?", today)
	changed, stats, err := execStudents(ctx, query, "resume student care")
	return changed == 1, stats, err
}
func withStudentTenant(query *bun.UpdateQuery, tenantID int64) *bun.UpdateQuery {
	return query.Where("student.tenant_id = ?", tenantID)
}
func requireStudentWriteTenant(tenantID int64) error {
	if tenantID <= 0 {
		return errors.New("school membership: tenant is required")
	}
	return nil
}
