package schedule

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

type ActivityRecoveryRepository struct{ db *bun.DB }

func NewActivityRecoveryRepository(db *bun.DB) *ActivityRecoveryRepository {
	return &ActivityRecoveryRepository{db: db}
}

func (r *ActivityRecoveryRepository) LockOpenSupervisors(ctx context.Context, activeGroupID int64) error {
	db := base.GetDB(ctx, r.db)
	var supervisorIDs []int64
	query := db.NewSelect().
		TableExpr(`active.group_supervisors AS "group_supervisor"`).
		ColumnExpr(`"group_supervisor".id`).
		Where(`"group_supervisor".group_id = ?`, activeGroupID).
		Where(`"group_supervisor".end_date IS NULL`).
		OrderExpr(`"group_supervisor".id ASC`).
		For("UPDATE")
	query = base.WithTenantFilter(ctx, query, "group_supervisor")
	if err := query.Scan(ctx, &supervisorIDs); err != nil {
		return fmt.Errorf("lock open supervisors: %w", err)
	}
	return nil
}

func (r *ActivityRecoveryRepository) LockSupervisors(ctx context.Context, supervisorIDs []int64) error {
	if len(supervisorIDs) == 0 {
		return nil
	}
	db := base.GetDB(ctx, r.db)
	var locked []int64
	query := db.NewSelect().
		TableExpr(`active.group_supervisors AS "group_supervisor"`).
		ColumnExpr(`"group_supervisor".id`).
		Where(`"group_supervisor".id IN (?)`, bun.In(supervisorIDs)).
		OrderExpr(`"group_supervisor".id ASC`).
		For("UPDATE")
	query = base.WithTenantFilter(ctx, query, "group_supervisor")
	if err := query.Scan(ctx, &locked); err != nil {
		return fmt.Errorf("lock supervisors: %w", err)
	}
	return nil
}

func (r *ActivityRecoveryRepository) LockOpenVisits(ctx context.Context, activeGroupID int64) error {
	db := base.GetDB(ctx, r.db)
	var visitIDs []int64
	query := db.NewSelect().
		TableExpr(`active.visits AS "visit"`).
		ColumnExpr(`"visit".id`).
		Where(`"visit".active_group_id = ?`, activeGroupID).
		Where(`"visit".exit_time IS NULL`).
		OrderExpr(`"visit".id ASC`).
		For("UPDATE")
	query = base.WithTenantFilter(ctx, query, "visit")
	if err := query.Scan(ctx, &visitIDs); err != nil {
		return fmt.Errorf("lock open visits: %w", err)
	}
	return nil
}

func (r *ActivityRecoveryRepository) LockAttendance(ctx context.Context, instanceID int64) error {
	db := base.GetDB(ctx, r.db)
	var rowIDs []int64
	query := db.NewSelect().
		TableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`"instance_student".id`).
		Where(`"instance_student".instance_id = ?`, instanceID).
		OrderExpr(`"instance_student".id ASC`).
		For("UPDATE")
	query = base.WithTenantFilter(ctx, query, "instance_student")
	if err := query.Scan(ctx, &rowIDs); err != nil {
		return fmt.Errorf("lock attendance: %w", err)
	}
	return nil
}

func (r *ActivityRecoveryRepository) Restore(ctx context.Context, instanceID int64, snapshot scheduleModel.ActivityCompletionSnapshot, now time.Time) error {
	db := base.GetDB(ctx, r.db)
	result, execErr := db.NewUpdate().Table("active.groups").Set("end_time = NULL").Set("last_activity = ?", now).Where("id = ? AND end_time IS NOT NULL", snapshot.ActiveGroupID).Exec(ctx)
	if err := expectRestoredRows(result, execErr, 1, "active group"); err != nil {
		return fmt.Errorf("restore active group: %w", err)
	}
	if len(snapshot.VisitIDs) > 0 {
		result, execErr = db.NewUpdate().Table("active.visits").Set("exit_time = NULL").Where("id IN (?) AND exit_time IS NOT NULL", bun.List(snapshot.VisitIDs)).Exec(ctx)
		if err := expectRestoredRows(result, execErr, int64(len(snapshot.VisitIDs)), "visits"); err != nil {
			return fmt.Errorf("restore visits: %w", err)
		}
	}
	if len(snapshot.SupervisorIDs) > 0 {
		result, execErr = db.NewUpdate().Table("active.group_supervisors").Set("end_date = NULL").Where("id IN (?) AND end_date IS NOT NULL", bun.List(snapshot.SupervisorIDs)).Exec(ctx)
		if err := expectRestoredRows(result, execErr, int64(len(snapshot.SupervisorIDs)), "supervisors"); err != nil {
			return fmt.Errorf("restore supervisors: %w", err)
		}
	}
	for _, row := range snapshot.Attendance {
		result, execErr = db.NewUpdate().Table("schedule.instance_students").Set("status = ?", row.Status).Set("substatus = ?", row.Substatus).Set("note = ?", row.Note).Set("checked_in_at = ?", row.CheckedInAt).Set("checked_out_at = ?", row.CheckedOutAt).Set("not_scheduled = ?", row.NotScheduled).Where("id = ? AND instance_id = ?", row.RowID, instanceID).Exec(ctx)
		if err := expectRestoredRows(result, execErr, 1, "attendance row"); err != nil {
			return fmt.Errorf("restore attendance row %d: %w", row.RowID, err)
		}
	}
	result, err := db.NewUpdate().Table("schedule.activity_instances").Set("status = 'active'").Set("active_group_id = ?", snapshot.ActiveGroupID).Set("completed_at = NULL").Set("completed_by = NULL").Set("reopen_until = NULL").Set("completion_snapshot = NULL").Where("id = ? AND status = 'completed'", instanceID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("restore instance: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count restored instances: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("restore instance: expected one completed instance, updated %d", rows)
	}
	return nil
}

func expectRestoredRows(result sql.Result, err error, expected int64, label string) error {
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != expected {
		return fmt.Errorf("snapshot mismatch for %s: expected %d rows, updated %d", label, expected, rows)
	}
	return nil
}
