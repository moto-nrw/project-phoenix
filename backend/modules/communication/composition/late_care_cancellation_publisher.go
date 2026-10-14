package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// LateCareCancellationPublisher forwards the instance lifecycle's
// cancellation notices (#2601) to a publisher that is composed after the
// lifecycle. resolve is read on every call, so the binding a composition
// completes before it serves requests is what the requests see.
func LateCareCancellationPublisher(resolve func() communication.CareCancellationPublisher) communication.CareCancellationPublisher {
	return lateCareCancellationPublisher{resolve: resolve}
}

type lateCareCancellationPublisher struct {
	resolve func() communication.CareCancellationPublisher
}

func (p lateCareCancellationPublisher) PublishCareCancellation(ctx context.Context, in communication.CareCancellationInput) (*communication.CareCancellationResult, error) {
	return p.resolve().PublishCareCancellation(ctx, in)
}

func (p lateCareCancellationPublisher) CareCancellationReachFor(ctx context.Context, studentIDs []int64) (*communication.CareCancellationReach, error) {
	return p.resolve().CareCancellationReachFor(ctx, studentIDs)
}
