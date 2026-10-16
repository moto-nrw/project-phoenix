package compose

import (
	"context"
	"fmt"
	"time"

	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/uptrace/bun"
)

type ActivityRecoveryRepository struct {
	store       *postgres.Store
	assignments application.AssignmentRecovery
	presence    application.PresenceRecovery
}

func NewActivityRecoveryRepository(db *bun.DB, assignments application.AssignmentRecovery, presence application.PresenceRecovery) *ActivityRecoveryRepository {
	if assignments == nil || presence == nil {
		panic("activity recovery: timetable assignments and presence are required")
	}
	return &ActivityRecoveryRepository{store: postgres.New(databaseRuntime(db)), assignments: assignments, presence: presence}
}

func (r *ActivityRecoveryRepository) CompletionTimestamp(ctx context.Context) (time.Time, error) {
	return r.store.TransactionTimestamp(ctx)
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

// Restore brings back the live group, its visits and supervisors, the
// attendance rows and the session exactly as the completion snapshot left
// them. Every step is an owner command inside the caller's transaction.
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
	if err := r.presence.RestoreAttendance(ctx, snapshot.Attendance); err != nil {
		return err
	}
	if err := r.presence.ReopenSession(ctx, instanceID, snapshot.ActiveGroupID); err != nil {
		return fmt.Errorf("restore instance: %w", err)
	}
	return nil
}

// NewRetainedActivityRecoveryRepository composes the completion and reopen
// support of a block over the database from the participant adapter's two
// owners (#2762).
func NewRetainedActivityRecoveryRepository(db *bun.DB, assignments scheduleModel.InstanceStudentRepository) scheduleModel.ActivityRecoveryRepository {
	recoveryAssignments, recoveryPresence := application.RetainedActivityRecovery(assignments)
	return NewActivityRecoveryRepository(db, recoveryAssignments, recoveryPresence)
}
