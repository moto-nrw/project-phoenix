package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// lateCareCancellationPublisher forwards the instance lifecycle's cancellation
// notices (#2601) to the announcement service, which the factory constructs
// after the instance service. resolve is read on every call, so the binding
// the factory completes before returning is what requests see.
type lateCareCancellationPublisher struct {
	resolve func() communication.CareCancellationPublisher
}

func (p lateCareCancellationPublisher) PublishCareCancellation(ctx context.Context, in communication.CareCancellationInput) (*communication.CareCancellationResult, error) {
	return p.resolve().PublishCareCancellation(ctx, in)
}

func (p lateCareCancellationPublisher) CareCancellationReachFor(ctx context.Context, studentIDs []int64) (*communication.CareCancellationReach, error) {
	return p.resolve().CareCancellationReachFor(ctx, studentIDs)
}
