package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

func (s *Store) EnrollStudent(ctx context.Context, input domain.StudentEnrollment) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err = requireStudentWriteTenant(tenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	from, err := optionalDate(input.EnrolledFrom, "enrollment start")
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	until, err := optionalDate(input.EnrolledUntil, "enrollment end")
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	var id int64
	gate, err := StudentClassWriteGateQuery(db, tenantID)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	err = db.NewRaw(`WITH class_write_gate AS MATERIALIZED (?)
 INSERT INTO users.student_school_memberships
  (tenant_id, student_profile_id, school_class, status, enrolled_from, enrolled_until, group_id)
  SELECT ?, ?, ?, ?, ?, ?, ? FROM class_write_gate RETURNING id`,
		gate, tenantID, input.StudentID, input.SchoolClass, input.Status, from, until, input.GroupID).Scan(ctx, &id)
	stats.StatementDuration = time.Since(started)
	if err == nil {
		stats.Rows = 1
	}
	return id, stats, err
}

func (s *Store) RenewStudentEnrollment(ctx context.Context, input domain.StudentEnrollment) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err = requireStudentWriteTenant(tenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	from, err := optionalDate(input.EnrolledFrom, "enrollment start")
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	until, err := optionalDate(input.EnrolledUntil, "enrollment end")
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	var id int64
	err = db.NewUpdate().TableExpr("users.student_school_memberships AS student").
		Set("school_class = ?", input.SchoolClass).Set("status = ?", input.Status).
		Set("enrolled_from = ?", from).Set("enrolled_until = ?", until).Set("updated_at = NOW()").
		Set("group_id = ?", input.GroupID).
		Where("student.tenant_id = ?", tenantID).Where("student.student_profile_id = ?", input.StudentID).
		Where("student.deleted_at IS NULL").
		Where(`(student.status <> 'alumnus' AND ? <> 'alumnus') OR
   (student.status = ? AND student.school_class = ? AND
    (student.enrolled_from = ?::date OR (student.enrolled_from IS NULL AND ?::date IS NULL)) AND
    (student.enrolled_until = ?::date OR (student.enrolled_until IS NULL AND ?::date IS NULL)) AND
    (student.group_id = ?::bigint OR (student.group_id IS NULL AND ?::bigint IS NULL)))`,
			input.Status, input.Status, input.SchoolClass, from, from, until, until, input.GroupID, input.GroupID).
		Returning("student.id").Scan(ctx, &id)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, stats, nil
	}
	if err == nil {
		stats.Rows = 1
	}
	return id, stats, err
}

func (s *Store) AssignStudentGroup(ctx context.Context, studentID int64, groupID *int64) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if err = requireStudentWriteTenant(tenantID); err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withStudentTenant(studentUpdate(db), tenantID).Set("group_id = ?", groupID).
		Where("student.student_profile_id = ?", studentID).
		Where("student.status <> 'alumnus' OR student.group_id = ?::bigint OR (student.group_id IS NULL AND ?::bigint IS NULL)", groupID, groupID)
	changed, stats, err := execStudents(ctx, query, "assign student group")
	return changed == 1, stats, err
}
