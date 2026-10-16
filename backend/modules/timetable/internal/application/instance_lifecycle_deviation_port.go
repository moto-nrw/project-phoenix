package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// DeviationLifecycle is the lifecycle the deviation saves hand the
// cancellation and the acknowledgement to; its errors pass through
// unchanged.
func (s *InstanceLifecycleService) DeviationLifecycle() DeviationLifecycle {
	return deviationLifecycle{lifecycle: s}
}

type deviationLifecycle struct {
	lifecycle *InstanceLifecycleService
}

func (l deviationLifecycle) CancelBlock(ctx context.Context, in DeviationCancellation) (CancelledBlock, error) {
	cancelled, err := l.lifecycle.CancelWithNotice(ctx, timetable.CancelInstanceInput{
		InstanceID: in.InstanceID, Reason: in.Reason, ActorAccountID: in.ActorAccountID, GuardianNotice: in.GuardianNotice,
	})
	if err != nil {
		return CancelledBlock{}, err
	}
	return CancelledBlock{
		InstanceID: cancelled.Instance.ID, UnderstaffedAck: cancelled.Instance.UnderstaffedAck, GuardianNotice: cancelled.GuardianNotice,
	}, nil
}

func (l deviationLifecycle) SetUnderstaffedAck(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64) error {
	_, err := l.lifecycle.SetUnderstaffedAck(ctx, instanceID, ack, note, actorAccountID)
	return err
}

func (l deviationLifecycle) ClearUnderstaffedAckIfStaffed(ctx context.Context, instanceID int64, actorAccountID *int64) error {
	return l.lifecycle.ClearUnderstaffedAckIfStaffed(ctx, instanceID, actorAccountID)
}
