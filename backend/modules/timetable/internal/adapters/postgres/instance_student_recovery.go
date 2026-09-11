package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Store) LockInstanceStudentAssignments(ctx context.Context, instanceID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	var ids []int64
	return scanAllInto(ctx, db.NewSelect().Table("schedule.instance_students").Column("id").
		Where("tenant_id = ?", tenantID).Where("instance_id = ?", instanceID).OrderExpr("id").For("UPDATE"), &ids, "lock attendance")
}
func (s *Store) RestoreInstanceStudentAttendanceRow(ctx context.Context, instanceID int64, row domain.CompletionAttendance) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats, err := execMeasuredWrite(ctx, db.NewUpdate().Table("schedule.instance_students").
		Set("status = ?", row.Status).Set("substatus = ?", row.Substatus).Set("note = ?", row.Note).
		Set("checked_in_at = ?", row.CheckedInAt).Set("checked_out_at = ?", row.CheckedOutAt).
		Set("not_scheduled = ?", row.NotScheduled).Set("student_status_day_id = ?", row.StudentStatusDayID).
		Set("pickup_exception_id = ?", row.PickupExceptionID).
		Where("tenant_id = ?", tenantID).Where("id = ? AND instance_id = ?", row.RowID, instanceID), "restore attendance")
	if err != nil {
		return stats, fmt.Errorf("restore attendance row %d: %w", row.RowID, err)
	}
	if stats.Rows != 1 {
		return stats, fmt.Errorf("restore attendance row %d: snapshot mismatch for attendance row: expected 1 rows, updated %d", row.RowID, stats.Rows)
	}
	return stats, nil
}
