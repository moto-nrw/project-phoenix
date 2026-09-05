package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) UpdateAttendanceFromCheckin(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats, err := execMeasuredWrite(ctx, attendanceCheckinUpdate(db, tenantID, checkedInAt).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".student_id = ?`, studentID), "update attendance from checkin")
	return stats.Rows > 0, stats, err
}

func (s *Store) UpdateAttendanceFromCheckinBatch(ctx context.Context, keys []domain.InstanceStudentKey, checkedInAt time.Time) (domain.OperationStats, error) {
	if len(keys) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := attendanceCheckinUpdate(db, tenantID, checkedInAt).
		Where(`("instance_student".instance_id, "instance_student".student_id) IN (?)`, bun.List(instanceStudentKeyTuples(keys)))
	return execMeasuredWrite(ctx, query, "update attendance from checkin batch")
}

func attendanceCheckinUpdate(db bun.IDB, tenantID int64, checkedInAt time.Time) *bun.UpdateQuery {
	return db.NewUpdate().TableExpr(`schedule.instance_students AS "instance_student"`).
		Set(`status = 'present'`).
		Set(`substatus = CASE WHEN "instance_student".student_status_day_id IS NOT NULL OR "instance_student".pickup_exception_id IS NOT NULL THEN NULL ELSE "instance_student".substatus END`).
		Set(`student_status_day_id = NULL`).Set(`pickup_exception_id = NULL`).
		Set(`checked_in_at = CASE WHEN "instance_student".checked_out_at IS NOT NULL THEN ? ELSE COALESCE("instance_student".checked_in_at, ?) END`, checkedInAt, checkedInAt).
		Set(`checked_out_at = NULL`).Set(`updated_at = NOW()`).Where(`"instance_student".tenant_id = ?`, tenantID).
		Where(`("instance_student".status = 'expected' OR "instance_student".student_status_day_id IS NOT NULL OR "instance_student".pickup_exception_id IS NOT NULL OR ("instance_student".status = 'present' AND "instance_student".checked_out_at IS NOT NULL))`)
}

func (s *Store) UpdateAttendanceCheckout(ctx context.Context, instanceID, studentID int64, checkedOutAt time.Time) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := attendanceCheckoutUpdate(db, tenantID, checkedOutAt).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".student_id = ?`, studentID)
	return execMeasuredWrite(ctx, query, "update slot attendance checkout")
}

func (s *Store) UpdateAttendanceCheckoutBatch(ctx context.Context, keys []domain.InstanceStudentKey, checkedOutAt time.Time) (domain.OperationStats, error) {
	if len(keys) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := attendanceCheckoutUpdate(db, tenantID, checkedOutAt).
		Where(`("instance_student".instance_id, "instance_student".student_id) IN (?)`, bun.List(instanceStudentKeyTuples(keys)))
	return execMeasuredWrite(ctx, query, "update slot attendance checkout batch")
}

func attendanceCheckoutUpdate(db bun.IDB, tenantID int64, checkedOutAt time.Time) *bun.UpdateQuery {
	return db.NewUpdate().TableExpr(`schedule.instance_students AS "instance_student"`).
		Set(`checked_out_at = CASE WHEN "instance_student".checked_out_at IS NULL OR "instance_student".checked_out_at < ? THEN ? ELSE "instance_student".checked_out_at END`, checkedOutAt, checkedOutAt).
		Set(`updated_at = NOW()`).Where(`"instance_student".tenant_id = ?`, tenantID).
		Where(`"instance_student".status = 'present'`).Where(`"instance_student".checked_in_at IS NOT NULL`).
		Where(`"instance_student".checked_in_at <= ?`, checkedOutAt)
}

func (s *Store) CreateUnplannedPresentIfAbsent(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (domain.InstanceStudent, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.InstanceStudent{}, domain.OperationStats{}, err
	}
	row := instanceStudentRow{TenantID: tenantID, InstanceID: instanceID, StudentID: studentID,
		Status: "present", CheckedInAt: &checkedInAt, IsUnplanned: true}
	stats, err := execMeasuredWrite(ctx, db.NewInsert().Model(&row).
		ModelTableExpr(`schedule.instance_students AS "attendance"`).On(unplannedPresentConflict), "create unplanned slot attendance")
	if err != nil {
		return domain.InstanceStudent{}, stats, err
	}
	row = instanceStudentRow{}
	found, findStats, err := scanOne(ctx, instanceStudentSelect(db, &row, tenantID).
		Where(`"instance_student".instance_id = ?`, instanceID).Where(`"instance_student".student_id = ?`, studentID), "find unplanned slot attendance")
	stats.Add(findStats)
	if err != nil {
		return domain.InstanceStudent{}, stats, err
	}
	if !found {
		return domain.InstanceStudent{}, stats, fmt.Errorf("timetable postgres: create unplanned slot attendance: %w", domain.ErrInstanceStudentNotFound)
	}
	return instanceStudentToDomain(row), stats, nil
}

const unplannedPresentConflict = `CONFLICT (instance_id, student_id) DO UPDATE
	SET status = EXCLUDED.status,
		substatus = CASE WHEN attendance.student_status_day_id IS NOT NULL OR attendance.pickup_exception_id IS NOT NULL THEN NULL ELSE attendance.substatus END,
		student_status_day_id = NULL, pickup_exception_id = NULL,
		checked_in_at = CASE WHEN attendance.checked_out_at IS NOT NULL THEN EXCLUDED.checked_in_at ELSE COALESCE(attendance.checked_in_at, EXCLUDED.checked_in_at) END,
		checked_out_at = NULL, updated_at = EXCLUDED.updated_at
	WHERE attendance.status = 'expected' OR attendance.student_status_day_id IS NOT NULL
		OR attendance.pickup_exception_id IS NOT NULL
		OR (attendance.status = 'present' AND attendance.checked_out_at IS NOT NULL)`

func (s *Store) ReconcileAttendanceInterval(ctx context.Context, instanceID, studentID int64, previousCheckIn time.Time, previousCheckOut *time.Time, updatedCheckIn time.Time, updatedCheckOut *time.Time) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := db.NewUpdate().TableExpr(`schedule.instance_students AS "instance_student"`).
		Set(`checked_in_at = ?`, updatedCheckIn).Set(`checked_out_at = ?`, updatedCheckOut).Set(`updated_at = NOW()`).
		Where(`"instance_student".tenant_id = ?`, tenantID).Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".student_id = ?`, studentID).Where(`"instance_student".status = 'present'`).
		Where(`"instance_student".checked_in_at = ?`, previousCheckIn).
		Where(`("instance_student".checked_out_at IS NOT DISTINCT FROM ? OR (? AND "instance_student".checked_out_at IS NULL))`, previousCheckOut, previousCheckOut != nil)
	stats, err := execMeasuredWrite(ctx, query, "reconcile slot attendance interval")
	return stats.Rows > 0, stats, err
}

func instanceStudentKeyTuples(keys []domain.InstanceStudentKey) []any {
	tuples := make([]any, 0, len(keys))
	for _, key := range keys {
		tuples = append(tuples, bun.Tuple([]int64{key.InstanceID, key.StudentID}))
	}
	return tuples
}
