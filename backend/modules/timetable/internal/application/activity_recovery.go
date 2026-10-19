package application

import (
	"context"
	"fmt"
	"time"

	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

type ActivityRecoveryRepository struct {
	completionClock
	assignments AssignmentRecovery
	PresenceRecovery
}

func NewActivityRecoveryRepository(clock completionClock, assignments AssignmentRecovery, presence PresenceRecovery) *ActivityRecoveryRepository {
	if assignments == nil || presence == nil {
		panic("activity recovery: timetable assignments and presence are required")
	}
	return &ActivityRecoveryRepository{completionClock: clock, assignments: assignments, PresenceRecovery: presence}
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

type completionClock interface {
	CompletionTimestamp(context.Context) (time.Time, error)
}
