package enrollmenthttp

import (
	"context"
	"time"

	capability "github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The rollover and the deletions as these routes speak them.

// Values of the rollover and the deletions the owner's contract carries
// unchanged.
type (
	DeadlineWorkerSummary        = capability.DeadlineWorkerSummary
	CreatePhaseFromSourceRequest = capability.CreatePhaseFromSourceRequest
	RolloverResult               = capability.RolloverResult
	RolloverPreview              = capability.RolloverPreview
	DecideReviewRequest          = capability.DecideReviewRequest
	EnrollmentDeletionService    = capability.EnrollmentDeletions
	// ReviewQueueItem is one row in the admin review UI.
	ReviewQueueItem = capability.RolloverReviewItem
)

// RolloverService creates a new phase from a source phase and resolves its
// renewal rows, over the owner's rollover.
type RolloverService interface {
	CreatePhaseFromSource(ctx context.Context, req CreatePhaseFromSourceRequest) (*RolloverResult, error)
	PreviewPhaseFromSource(ctx context.Context, sourcePhaseID int64, bumpsGrade bool) (*RolloverPreview, error)
	ListReviewQueue(ctx context.Context, phaseID int64) ([]*ReviewQueueItem, error)
	DecideReview(ctx context.Context, req DecideReviewRequest) error
	RunDeadlineWorker(ctx context.Context, asOf time.Time) (*DeadlineWorkerSummary, error)
}

// NewRolloverService serves the owner's rollover to these routes. A nil
// owner yields nil.
func NewRolloverService(owner capability.Rollovers) RolloverService {
	if owner == nil {
		return nil
	}
	return rolloverService{owner: owner, rolloverReviewQueue: owner}
}

// The review queue serves the owner's values since ADR 0173 removed the
// request codec; promote it instead of forwarding.
type rolloverReviewQueue interface {
	ListReviewQueue(ctx context.Context, phaseID int64) ([]*ReviewQueueItem, error)
}

type rolloverService struct {
	owner capability.Rollovers
	rolloverReviewQueue
}

func (c rolloverService) CreatePhaseFromSource(ctx context.Context, req CreatePhaseFromSourceRequest) (*RolloverResult, error) {
	return c.owner.CreatePhaseFromSource(ctx, req)
}

func (c rolloverService) PreviewPhaseFromSource(ctx context.Context, sourcePhaseID int64, bumpsGrade bool) (*RolloverPreview, error) {
	return c.owner.PreviewPhaseFromSource(ctx, sourcePhaseID, bumpsGrade)
}

func (c rolloverService) DecideReview(ctx context.Context, req DecideReviewRequest) error {
	return c.owner.DecideReview(ctx, req)
}

func (c rolloverService) RunDeadlineWorker(ctx context.Context, asOf time.Time) (*DeadlineWorkerSummary, error) {
	return c.owner.RunDeadlineWorker(ctx, asOf)
}
