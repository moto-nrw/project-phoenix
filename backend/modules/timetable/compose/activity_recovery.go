package compose

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/uptrace/bun"
)

type ActivityRecoveryRepository struct {
	completionClock
	assignments application.AssignmentRecovery
	application.PresenceRecovery
}

func NewActivityRecoveryRepository(db *bun.DB, assignments application.AssignmentRecovery, presence application.PresenceRecovery) *ActivityRecoveryRepository {
	if assignments == nil || presence == nil {
		panic("activity recovery: timetable assignments and presence are required")
	}
	return &ActivityRecoveryRepository{completionClock: postgres.New(databaseRuntime(db)), assignments: assignments, PresenceRecovery: presence}
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
	if err := r.RestoreGroup(ctx, snapshot.ActiveGroupID, now); err != nil {
		return err
	}
	if err := r.RestoreVisits(ctx, snapshot.VisitIDs); err != nil {
		return err
	}
	if err := r.RestoreSupervisors(ctx, snapshot.SupervisorIDs); err != nil {
		return err
	}
	if err := r.RestoreAttendance(ctx, snapshot.Attendance); err != nil {
		return err
	}
	if err := r.ReopenSession(ctx, instanceID, snapshot.ActiveGroupID); err != nil {
		return fmt.Errorf("restore instance: %w", err)
	}
	return nil
}

// NewRetainedActivityRecoveryRepository composes the completion and reopen
// support of a block over the database from the participant adapter's two
// owners (#2762).
func NewRetainedActivityRecoveryRepository(db *bun.DB, assignments ports.InstanceStudentRecords) ports.ActivityRecoveryRecords {
	recoveryAssignments, recoveryPresence := application.RetainedActivityRecovery(assignments)
	return NewActivityRecoveryRepository(db, recoveryAssignments, recoveryPresence)
}

type completionClock interface {
	CompletionTimestamp(context.Context) (time.Time, error)
}
