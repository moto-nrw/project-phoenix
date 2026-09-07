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

type AssignmentRecovery interface {
	LockAttendance(context.Context, int64) error
	RestoreAttendance(context.Context, int64, []scheduleModel.CompletionAttendanceSnapshot) error
}

// PresenceRecovery is the Student Presence capability this repository needs:
// the live group, its supervisors, and its visits are owned there.
type PresenceRecovery interface {
	LockOpenVisits(context.Context, int64) error
	RestoreVisits(context.Context, []int64) error
	LockOpenSupervisors(context.Context, int64) error
	LockSupervisors(context.Context, []int64) error
	RestoreGroup(context.Context, int64, time.Time) error
	RestoreSupervisors(context.Context, []int64) error
}

type ActivityRecoveryRepository struct {
	db          *bun.DB
	assignments AssignmentRecovery
	presence    PresenceRecovery
}

func NewActivityRecoveryRepository(db *bun.DB, assignments AssignmentRecovery, presence PresenceRecovery) *ActivityRecoveryRepository {
	if assignments == nil || presence == nil {
		panic("activity recovery: timetable assignments and presence are required")
	}
	return &ActivityRecoveryRepository{db: db, assignments: assignments, presence: presence}
}

func (r *ActivityRecoveryRepository) LockOpenSupervisors(ctx context.Context, activeGroupID int64) error {
	return r.presence.LockOpenSupervisors(ctx, activeGroupID)
}

func (r *ActivityRecoveryRepository) LockSupervisors(ctx context.Context, supervisorIDs []int64) error {
	return r.presence.LockSupervisors(ctx, supervisorIDs)
}

func (r *ActivityRecoveryRepository) LockOpenVisits(ctx context.Context, activeGroupID int64) error {
	return r.presence.LockOpenVisits(ctx, activeGroupID)
}

func (r *ActivityRecoveryRepository) LockAttendance(ctx context.Context, instanceID int64) error {
	if err := r.assignments.LockAttendance(ctx, instanceID); err != nil {
		return fmt.Errorf("lock attendance: %w", err)
	}
	return nil
}

func (r *ActivityRecoveryRepository) Restore(ctx context.Context, instanceID int64, snapshot scheduleModel.ActivityCompletionSnapshot, now time.Time) error {
	if err := r.presence.RestoreGroup(ctx, snapshot.ActiveGroupID, now); err != nil {
		return err
	}
	if err := r.presence.RestoreVisits(ctx, snapshot.VisitIDs); err != nil {
		return err
	}
	if err := r.presence.RestoreSupervisors(ctx, snapshot.SupervisorIDs); err != nil {
		return err
	}
	if err := r.assignments.RestoreAttendance(ctx, instanceID, snapshot.Attendance); err != nil {
		return err
	}
	db := base.GetDB(ctx, r.db)
	result, err := db.NewUpdate().Table("schedule.activity_instances").Set("status = 'active'").Set("active_group_id = ?", snapshot.ActiveGroupID).Set("completed_at = NULL").Set("completed_by = NULL").Set("reopen_until = NULL").Set("completion_snapshot = NULL").Where("id = ? AND status = 'completed'", instanceID).Exec(ctx)
	if err := expectRestoredRows(result, err, 1, "completed instance"); err != nil {
		return fmt.Errorf("restore instance: %w", err)
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
