package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/schoolmembership"
	"github.com/moto-ogs/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

func (s *Store) EnrollStudent(ctx context.Context, input schoolmembership.StudentEnrollment) (int64, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err = requireStudentWriteTenant(scope.TenantID); err != nil {
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
	gate, err := StudentClassWriteGateQuery(db, scope.TenantID)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	firstCare := bun.SafeQuery(firstCareFromOnInsert, input.Status, from, scope.TenantID, input.StudentID)
	err = db.NewRaw(`WITH class_write_gate AS MATERIALIZED (?)
 INSERT INTO users.student_school_memberships
  (tenant_id, student_profile_id, school_class, status, enrolled_from, enrolled_until, group_id, first_care_from)
  SELECT ?, ?, ?, ?, ?, ?, ?, ? FROM class_write_gate RETURNING id`,
		gate, scope.TenantID, input.StudentID, input.SchoolClass, input.Status, from, until, input.GroupID, firstCare).Scan(ctx, &id)
	stats.StatementDuration = time.Since(started)
	if err == nil {
		stats.Rows = 1
	}
	return id, stats, err
}

func (s *Store) RenewStudentEnrollment(ctx context.Context, input schoolmembership.StudentEnrollment, today string) (int64, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err = requireStudentWriteTenant(scope.TenantID); err != nil {
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
	day, err := currentDay(today)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	var id int64
	restart := bun.SafeQuery(careRestart, day, until)
	err = db.NewUpdate().TableExpr("users.student_school_memberships AS student").
		Set("school_class = ?", input.SchoolClass).Set("status = ?", input.Status).
		Set("enrolled_from = ?", from).Set("enrolled_until = ?", until).Set("updated_at = NOW()").
		Set(firstCareFromNext, day, input.Status, from).
		Set(carePausedAfterNext, restart).Set(careResumedFromNext, day, input.Status, from, restart).
		Set("group_id = ?", input.GroupID).
		Where("student.tenant_id = ?", scope.TenantID).Where("student.student_profile_id = ?", input.StudentID).
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
	db, scope, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if err = requireStudentWriteTenant(scope.TenantID); err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withStudentTenant(studentUpdate(db), scope).Set("group_id = ?", groupID).
		Where("student.student_profile_id = ?", studentID).
		Where("student.status <> 'alumnus' OR student.group_id = ?::bigint OR (student.group_id IS NULL AND ?::bigint IS NULL)", groupID, groupID)
	changed, stats, err := execStudents(ctx, query, "assign student group")
	return changed == 1, stats, err
}

// LockStudentWindow reads the child's class, status, window and group under a
// row lock, for a write that decides on them (#6505).
func (s *Store) LockStudentWindow(ctx context.Context, id int64) (schoolmembership.StudentEnrollment, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.StudentEnrollment{}, false, domain.OperationStats{}, err
	}
	if err = requireStudentWriteTenant(scope.TenantID); err != nil {
		return schoolmembership.StudentEnrollment{}, false, domain.OperationStats{}, err
	}
	var row struct {
		SchoolClass   string         `bun:"school_class"`
		Status        string         `bun:"status"`
		EnrolledFrom  *calendar.Date `bun:"enrolled_from"`
		EnrolledUntil *calendar.Date `bun:"enrolled_until"`
		GroupID       *int64         `bun:"group_id"`
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`SELECT school_class, status, enrolled_from, enrolled_until, group_id
 FROM users.student_school_memberships
 WHERE tenant_id = ? AND student_profile_id = ? AND deleted_at IS NULL
 FOR UPDATE`, scope.TenantID, id).Scan(ctx, &row)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return schoolmembership.StudentEnrollment{}, false, stats, nil
	}
	if err != nil {
		return schoolmembership.StudentEnrollment{}, false, stats, err
	}
	stats.Rows = 1
	window := schoolmembership.StudentEnrollment{StudentID: id, GroupID: row.GroupID, SchoolClass: row.SchoolClass, Status: row.Status}
	if row.EnrolledFrom != nil {
		window.EnrolledFrom = row.EnrolledFrom.String()
	}
	if row.EnrolledUntil != nil {
		window.EnrolledUntil = row.EnrolledUntil.String()
	}
	return window, true, stats, nil
}
