package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// DeviationLifecycle is the lifecycle the deviation saves hand the
// cancellation and the acknowledgement to; its errors pass through
// unchanged.
func (s *InstanceLifecycleService) DeviationLifecycle() ports.DeviationLifecycle {
	return deviationLifecycle{lifecycle: s, deviationAckClearer: s}
}

type deviationAckClearer interface {
	ClearUnderstaffedAckIfStaffed(context.Context, int64, *int64) error
}

type deviationLifecycle struct {
	deviationAckClearer
	lifecycle *InstanceLifecycleService
}

func (l deviationLifecycle) CancelBlock(ctx context.Context, in ports.DeviationCancellation) (ports.CancelledBlock, error) {
	cancelled, err := l.lifecycle.CancelWithNotice(ctx, timetable.CancelInstanceInput{
		InstanceID: in.InstanceID, Reason: in.Reason, ActorAccountID: in.ActorAccountID, GuardianNotice: in.GuardianNotice,
	})
	if err != nil {
		return ports.CancelledBlock{}, err
	}
	return ports.CancelledBlock{
		InstanceID: cancelled.Instance.ID, UnderstaffedAck: cancelled.Instance.UnderstaffedAck, GuardianNotice: cancelled.GuardianNotice,
	}, nil
}

func (l deviationLifecycle) SetUnderstaffedAck(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64) error {
	_, err := l.lifecycle.SetUnderstaffedAck(ctx, instanceID, ack, note, actorAccountID)
	return err
}
