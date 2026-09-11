package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) UpdateAttendanceFields(ctx context.Context, id int64, patch domain.AttendanceFieldPatch) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewUpdate().TableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".tenant_id = ?`, tenantID).Where(`"instance_student".id = ?`, id)
	query = setAttendanceStatusFields(query, patch)
	query = setAttendanceNoteFields(query, patch).Set(`updated_at = NOW()`)
	return execMeasuredWrite(ctx, query, "update attendance fields")
}

func setAttendanceStatusFields(query *bun.UpdateQuery, patch domain.AttendanceFieldPatch) *bun.UpdateQuery {
	clearProvenance, manual := false, false
	if patch.Status != nil {
		query = query.Set(`status = ?`, *patch.Status).Set(`not_scheduled = FALSE`)
		clearProvenance, manual = true, true
	}
	if patch.SubstatusClear {
		query = query.Set(`substatus = NULL`)
		clearProvenance, manual = true, true
	} else if patch.Substatus != nil {
		query = query.Set(`substatus = ?`, *patch.Substatus)
		clearProvenance, manual = true, true
	}
	if clearProvenance {
		query = query.Set(`student_status_day_id = NULL`).Set(`pickup_exception_id = NULL`)
	}
	if manual {
		query = query.Set(`manual_status_at = NOW()`)
	}
	return query
}

func setAttendanceNoteFields(query *bun.UpdateQuery, patch domain.AttendanceFieldPatch) *bun.UpdateQuery {
	if patch.NoteClear {
		return query.Set(`note = NULL`)
	}
	if patch.Note != nil {
		return query.Set(`note = ?`, *patch.Note)
	}
	return query
}

func (s *Store) BulkUpdateStatus(ctx context.Context, instanceID int64, fromStatus, toStatus string, excludedStudentIDs []int64) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewUpdate().TableExpr(`schedule.instance_students AS "instance_student"`).
		Set(`status = ?`, toStatus).Set(`updated_at = NOW()`).Where(`"instance_student".tenant_id = ?`, tenantID).
		Where(`"instance_student".instance_id = ?`, instanceID).Where(`"instance_student".status = ?`, fromStatus)
	if len(excludedStudentIDs) > 0 {
		query = query.Where(`"instance_student".student_id NOT IN (?)`, bun.List(excludedStudentIDs))
	}
	stats, err := execMeasuredWrite(ctx, query, "bulk update attendance status")
	return stats.Rows, stats, err
}

func (s *Store) MarkNotScheduled(ctx context.Context, refs []domain.StudentInstanceRef) (domain.OperationStats, error) {
	if len(refs) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := markNotScheduledUpdate(db, tenantID)
	query = query.WhereGroup(" AND ", func(group *bun.UpdateQuery) *bun.UpdateQuery {
		for _, ref := range refs {
			group = group.WhereOr(`("instance_student".instance_id = ? AND "instance_student".student_id = ?)`, ref.InstanceID, ref.StudentID)
		}
		return group
	})
	return execMeasuredWrite(ctx, query, "mark attendance rows not scheduled")
}

func markNotScheduledUpdate(db bun.IDB, tenantID int64) *bun.UpdateQuery {
	return db.NewUpdate().TableExpr(`schedule.instance_students AS "instance_student"`).
		Set(`not_scheduled = TRUE`).Set(`status = 'expected'`).
		Set(`substatus = CASE WHEN "instance_student".student_status_day_id IS NOT NULL OR "instance_student".pickup_exception_id IS NOT NULL THEN NULL ELSE "instance_student".substatus END`).
		Set(`student_status_day_id = NULL`).Set(`pickup_exception_id = NULL`).Set(`updated_at = NOW()`).
		Where(`"instance_student".tenant_id = ?`, tenantID).Where(`"instance_student".manual_status_at IS NULL`).
		Where(`NOT EXISTS (SELECT 1 FROM schedule.activity_instances AS "instance" WHERE "instance".tenant_id = ? AND "instance".id = "instance_student".instance_id AND "instance".status IN ('completed', 'cancelled'))`, tenantID).
		WhereGroup(" AND ", func(group *bun.UpdateQuery) *bun.UpdateQuery {
			return group.WhereOr(`"instance_student".status = 'expected'`).
				WhereOr(`("instance_student".status = 'absent' AND "instance_student".student_status_day_id IS NOT NULL)`).
				WhereOr(`("instance_student".status = 'absent' AND "instance_student".pickup_exception_id IS NOT NULL)`)
		})
}

func (s *Store) MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, updatedAt time.Time, exclusions []domain.StudentInstanceRef) (domain.OperationStats, error) {
	if len(activeGroupIDs) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewUpdate().TableExpr(`schedule.instance_students AS "instance_student"`).
		Set(`status = 'absent'`).Set(`updated_at = ?`, updatedAt).Where(`"instance_student".tenant_id = ?`, tenantID).
		Where(`"instance_student".status = 'expected'`).
		Where(`"instance_student".instance_id IN (SELECT "instance".id FROM schedule.activity_instances AS "instance" WHERE "instance".tenant_id = ? AND "instance".status = 'active' AND "instance".active_group_id IN (?))`, tenantID, bun.List(activeGroupIDs))
	for _, exclusion := range exclusions {
		query = query.Where(`NOT ("instance_student".instance_id = ? AND "instance_student".student_id = ?)`, exclusion.InstanceID, exclusion.StudentID)
	}
	return execMeasuredWrite(ctx, query, "mark expected absent by active group ids")
}

func (s *Store) CloseOpenCheckoutsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, checkedOutAt time.Time) (int64, domain.OperationStats, error) {
	if len(activeGroupIDs) == 0 {
		return 0, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewUpdate().TableExpr(`schedule.instance_students AS "instance_student"`).
		Set(`checked_out_at = ?`, checkedOutAt).Set(`updated_at = NOW()`).Where(`"instance_student".tenant_id = ?`, tenantID).
		Where(`"instance_student".status = 'present'`).Where(`"instance_student".checked_in_at IS NOT NULL`).
		Where(`"instance_student".checked_in_at <= ?`, checkedOutAt).Where(`"instance_student".checked_out_at IS NULL`).
		Where(`"instance_student".instance_id IN (SELECT "instance".id FROM schedule.activity_instances AS "instance" WHERE "instance".tenant_id = ? AND "instance".active_group_id IN (?))`, tenantID, bun.List(activeGroupIDs))
	stats, err := execMeasuredWrite(ctx, query, "close open checkouts by active group ids")
	return stats.Rows, stats, err
}

func (s *Store) ListStudentInstanceRefsBefore(ctx context.Context, cutoff string) ([]domain.StudentInstanceRef, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []domain.StudentInstanceRef{}
	query := db.NewSelect().TableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`"instance_student".student_id, "instance_student".instance_id`).
		Join(`INNER JOIN schedule.activity_instances AS "instance" ON "instance".id = "instance_student".instance_id AND "instance".tenant_id = "instance_student".tenant_id`).
		Where(`"instance_student".tenant_id = ?`, tenantID).Where(`"instance".date < ?::date`, cutoff).
		OrderExpr(`"instance_student".student_id ASC, "instance_student".instance_id ASC`)
	stats, err := scanAllInto(ctx, query, &rows, "list student instance refs before")
	stats.Rows = int64(len(rows))
	return rows, stats, err
}
