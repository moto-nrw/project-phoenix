package enrollment

import (
	"context"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The rollover, the admin deletion and the retention cleanup run in the
// Enrollment owner (#3564); the names below keep the retained consumers on
// the owner's values.

// Sentinels of the rollover and the deletions, pointed at the owner values.
var (
	ErrRolloverSourceNotFound      = capability.ErrRolloverSourceNotFound
	ErrRolloverInvalidRequest      = capability.ErrRolloverInvalidRequest
	ErrRolloverReviewNotFound      = capability.ErrRolloverReviewNotFound
	ErrRolloverReviewInvalid       = capability.ErrRolloverReviewInvalid
	ErrRolloverDuplicateName       = capability.ErrRolloverDuplicateName
	ErrRolloverSourceAlreadyRolled = capability.ErrRolloverSourceAlreadyRolled

	ErrEnrollmentDeletionNotFound      = capability.ErrEnrollmentDeletionNotFound
	ErrEnrollmentDeletionInvalidReason = capability.ErrEnrollmentDeletionInvalidReason
	ErrEnrollmentDeletionNotAllowed    = capability.ErrEnrollmentDeletionNotAllowed
	ErrEnrollmentDeletionStudentExists = capability.ErrEnrollmentDeletionStudentExists
)

// Values of the rollover and the deletions the owner's contract carries
// unchanged.
type (
	DeadlineWorkerSummary           = capability.DeadlineWorkerSummary
	CreatePhaseFromSourceRequest    = capability.CreatePhaseFromSourceRequest
	RolloverResult                  = capability.RolloverResult
	RolloverPreview                 = capability.RolloverPreview
	DecideReviewRequest             = capability.DecideReviewRequest
	EnrollmentDeletionService       = capability.EnrollmentDeletions
	RejectedEnrollmentCleaner       = capability.RejectedEnrollmentCleaner
	RejectedEnrollmentCleanupResult = capability.RejectedEnrollmentCleanupResult
)

// ReviewDecision values the admin can pick on a queued review row.
const (
	ReviewDecisionKeep  = capability.ReviewDecisionKeep
	ReviewDecisionDrop  = capability.ReviewDecisionDrop
	ReviewDecisionDefer = capability.ReviewDecisionDefer
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

// NewRolloverService decodes the owner's rollover for the retained
// consumers.
func NewRolloverService(owner capability.Rollovers) RolloverService {
	return rolloverContract{owner: owner}
}

type rolloverContract struct {
	owner capability.Rollovers
}

func (c rolloverContract) CreatePhaseFromSource(ctx context.Context, req CreatePhaseFromSourceRequest) (*RolloverResult, error) {
	return c.owner.CreatePhaseFromSource(ctx, req)
}

func (c rolloverContract) PreviewPhaseFromSource(ctx context.Context, sourcePhaseID int64, bumpsGrade bool) (*RolloverPreview, error) {
	return c.owner.PreviewPhaseFromSource(ctx, sourcePhaseID, bumpsGrade)
}

func (c rolloverContract) ListReviewQueue(ctx context.Context, phaseID int64) ([]*ReviewQueueItem, error) {
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
	child, err := intakeChildValue(value.Child)
	if err != nil {
		return nil, err
	}
	request, err := intakeRequestValue(value.Request)
	if err != nil {
		return nil, err
	}
	source, err := intakeChildValue(value.SourceChild)
	if err != nil {
		return nil, err
	}
	return &ReviewQueueItem{Child: child, Request: request, SourceChild: source}, nil
}

func (c rolloverContract) DecideReview(ctx context.Context, req DecideReviewRequest) error {
	return c.owner.DecideReview(ctx, req)
}

func (c rolloverContract) RunDeadlineWorker(ctx context.Context, asOf time.Time) (*DeadlineWorkerSummary, error) {
	return c.owner.RunDeadlineWorker(ctx, asOf)
}
