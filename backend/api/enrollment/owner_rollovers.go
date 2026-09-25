package enrollment

import (
	"context"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
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
)

// ReviewQueueItem is one row in the admin review UI, decoded.
type ReviewQueueItem struct {
	Child       *RequestChild
	Request     *enrollmentModels.Request
	SourceChild *RequestChild
}

// RolloverService creates a new phase from a source phase and resolves its
// renewal rows, over the owner's rollover.
type RolloverService interface {
	CreatePhaseFromSource(ctx context.Context, req CreatePhaseFromSourceRequest) (*RolloverResult, error)
	PreviewPhaseFromSource(ctx context.Context, sourcePhaseID int64, bumpsGrade bool) (*RolloverPreview, error)
	ListReviewQueue(ctx context.Context, phaseID int64) ([]*ReviewQueueItem, error)
	DecideReview(ctx context.Context, req DecideReviewRequest) error
	RunDeadlineWorker(ctx context.Context, asOf time.Time) (*DeadlineWorkerSummary, error)
}

// NewRolloverService decodes the owner's rollover for these routes. A nil
// owner yields nil.
func NewRolloverService(owner capability.Rollovers) RolloverService {
	if owner == nil {
		return nil
	}
	return rolloverService{owner: owner}
}

type rolloverService struct{ owner capability.Rollovers }

func (c rolloverService) CreatePhaseFromSource(ctx context.Context, req CreatePhaseFromSourceRequest) (*RolloverResult, error) {
	return c.owner.CreatePhaseFromSource(ctx, req)
}

func (c rolloverService) PreviewPhaseFromSource(ctx context.Context, sourcePhaseID int64, bumpsGrade bool) (*RolloverPreview, error) {
	return c.owner.PreviewPhaseFromSource(ctx, sourcePhaseID, bumpsGrade)
}

func (c rolloverService) ListReviewQueue(ctx context.Context, phaseID int64) ([]*ReviewQueueItem, error) {
	values, err := c.owner.ListReviewQueue(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	out := make([]*ReviewQueueItem, 0, len(values))
	for _, value := range values {
		item, err := reviewQueueItem(value)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func reviewQueueItem(value *capability.RolloverReviewItem) (*ReviewQueueItem, error) {
	child, err := childValue(value.Child)
	if err != nil {
		return nil, err
	}
	request, err := requestValue(value.Request)
	if err != nil {
		return nil, err
	}
	source, err := childValue(value.SourceChild)
	if err != nil {
		return nil, err
	}
	return &ReviewQueueItem{Child: child, Request: request, SourceChild: source}, nil
}

func (c rolloverService) DecideReview(ctx context.Context, req DecideReviewRequest) error {
	return c.owner.DecideReview(ctx, req)
}

func (c rolloverService) RunDeadlineWorker(ctx context.Context, asOf time.Time) (*DeadlineWorkerSummary, error) {
	return c.owner.RunDeadlineWorker(ctx, asOf)
}
