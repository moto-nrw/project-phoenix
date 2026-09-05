package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func (s *Store) LockOpenStudentAssignments(ctx context.Context, studentIDs []int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	var ids []int64
	return scanAllInto(ctx, db.NewSelect().Table("schedule.instance_students").Column("id").
		Where("tenant_id = ?", tenantID).Where("student_id IN (?)", bun.List(studentIDs)).
		Where("checked_in_at IS NOT NULL").Where("checked_out_at IS NULL").OrderExpr("id").For("UPDATE"),
		&ids, "lock open student assignments")
}

func (s *Store) LockPlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	var ids []int64
	return scanAllInto(ctx, db.NewSelect().TableExpr("schedule.instance_students AS s").ColumnExpr("s.id").
		Join("JOIN schedule.activity_instances AS ai ON ai.id = s.instance_id AND ai.tenant_id = s.tenant_id").
		Where("s.student_id IN (?)", bun.List(studentIDs)).
		Where("s.checked_in_at IS NULL AND s.checked_out_at IS NULL").
		Where("ai.date > ?::date", after).Where("ai.status NOT IN ('completed', 'cancelled')").
		Where("s.tenant_id = ?", tenantID).OrderExpr("s.id").For("UPDATE OF s"),
		&ids, "lock planned student assignments after care end")
}

func (s *Store) ReconnectCareExitAssignmentPickupExceptions(ctx context.Context, studentIDs, pickupExceptionIDs []int64, removals []domain.InstanceStudent) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	snapshot, err := encodeCareExitRoster(removals)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewRaw(`
 UPDATE schedule.instance_students AS live SET pickup_exception_id = rm.pickup_exception_id
 FROM `+careExitRosterRecordset+`
 WHERE rm.tenant_id = ? AND rm.student_id IN (?)
 AND rm.pickup_exception_id = ANY(?::BIGINT[])
 AND live.tenant_id = rm.tenant_id AND live.instance_id = rm.instance_id AND live.student_id = rm.student_id
 `, snapshot, tenantID, bun.List(studentIDs), pgdialect.Array(pickupExceptionIDs)),
		"reconnect restored roster pickup exception")
}
