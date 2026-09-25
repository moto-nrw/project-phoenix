package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// enrollmentRequestDeliveryType is the related-entity type of a request's
// delivery intents.
const enrollmentRequestDeliveryType = "enrollment_request"

// DeletionChildren are the owner writes and reads of a deletion.
type DeletionChildren interface {
	DeleteRequestTree(context.Context, int64) error
	DeleteRequestChildTree(context.Context, int64, int64) error
	ChildrenForRequest(ctx context.Context, requestID int64, forUpdate bool) ([]*enrollment.RequestChild, error)
	ChildByID(context.Context, int64) (*enrollment.RequestChild, error)
}

// DeletionRequests locks the request a deletion removes.
type DeletionRequests interface {
	RequestByID(context.Context, int64, bool) (*enrollment.Request, error)
}

// EnrollmentDeletionDelivery counts and cancels the delivery intents of a
// request.
type EnrollmentDeletionDelivery interface {
	CountRelatedEmails(context.Context, string, int64) (int, error)
	CancelRelatedEmails(context.Context, string, int64, string) (int64, error)
}

// DeletionActor tells an admin deletion from the retention worker's.
type DeletionActor int

const (
	DeletionActorAdmin DeletionActor = iota + 1
	DeletionActorSystem
)

// DeletionScope tells a request deletion from a child deletion.
type DeletionScope int

const (
	DeletionScopeRequest DeletionScope = iota + 1
	DeletionScopeChild
)

// EnrollmentDeletionEvent is the append-only audit row of a deletion.
type EnrollmentDeletionEvent struct {
	RequestID      int64
	ChildID        *int64
	ActorAccountID *int64
	Actor          DeletionActor
	Scope          DeletionScope
	Reason         string
	Counts         enrollment.DeletionCounts
	DeletedAt      time.Time
}

// EnrollmentDeletionAudit appends the deletion audit rows.
type EnrollmentDeletionAudit interface {
	RecordEnrollmentDeletion(context.Context, EnrollmentDeletionEvent) error
}

// DeletionDependencies bind the admin deletion to its owners.
type DeletionDependencies struct {
	Requests DeletionRequests
	Children DeletionChildren
	Preview  *DeletionPreview
	Audit    EnrollmentDeletionAudit
	Delivery EnrollmentDeletionDelivery
	Runtime  Runtime
	Logger   *slog.Logger
}

// Deletions previews and deletes an enrollment request or one of its
// children. A deletion cancels the request's pending mails and appends the
// audit row in the same tenant transaction.
type Deletions struct {
	deps DeletionDependencies
}

var _ enrollment.EnrollmentDeletions = (*Deletions)(nil)

// NewDeletions composes the admin deletion.
func NewDeletions(deps DeletionDependencies) *Deletions {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &Deletions{deps: deps}
}

// PreviewRequest previews the deletion of a whole request.
func (s *Deletions) PreviewRequest(ctx context.Context, requestID int64) (*enrollment.DeletionImpact, error) {
	if err := s.validateConfigured(); err != nil {
		return nil, err
	}
	impact, err := s.deps.Preview.PreviewRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if impact == nil || impact.Counts.Requests != 1 {
		return nil, enrollment.ErrEnrollmentDeletionNotFound
	}
	if err := s.addDeliveryCount(ctx, impact); err != nil {
		return nil, err
	}
	return impact, nil
}

// PreviewChild previews the deletion of one child.
func (s *Deletions) PreviewChild(ctx context.Context, requestID, childID int64) (*enrollment.DeletionImpact, error) {
	if err := s.validateConfigured(); err != nil {
		return nil, err
	}
	impact, err := s.deps.Preview.PreviewChild(ctx, requestID, childID)
	if err != nil {
		return nil, err
	}
	if impact == nil || impact.Counts.RequestChildren != 1 {
		return nil, enrollment.ErrEnrollmentDeletionNotFound
	}
	child, err := s.deps.Children.ChildByID(ctx, childID)
	if err != nil {
		return nil, fmt.Errorf("load enrollment child for deletion preview: %w", err)
	}
	if child == nil || child.RequestID != requestID {
		return nil, enrollment.ErrEnrollmentDeletionNotFound
	}
	if impact.DeletesRequest {
		if err := s.addDeliveryCount(ctx, impact); err != nil {
			return nil, err
		}
	}
	if err := validateDeletableEnrollmentChild(child); err != nil {
		return nil, err
	}
	return impact, nil
}

// DeleteRequest deletes a whole request.
func (s *Deletions) DeleteRequest(ctx context.Context, requestID, actorAccountID int64, reason string) (*enrollment.DeletionImpact, error) {
	reason, err := validateEnrollmentDeletionInput(actorAccountID, reason)
	if err != nil {
		return nil, err
	}
	if err = s.validateConfigured(); err != nil {
		return nil, err
	}
	var impact *enrollment.DeletionImpact
	err = s.deps.Runtime.Transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var txErr error
		impact, txErr = s.deleteRequest(txCtx, requestID, actorAccountID, reason)
		return txErr
	})
	if err != nil {
		return nil, err
	}
	s.deps.Logger.InfoContext(ctx, "enrollment request deleted",
		slog.Int64("request_id", requestID),
		slog.Int64("actor_account_id", actorAccountID),
		slog.Int("records_affected", impact.Counts.Total()))
	return impact, nil
}

func (s *Deletions) deleteRequest(ctx context.Context, requestID, actorAccountID int64, reason string) (*enrollment.DeletionImpact, error) {
	if err := s.lockRequestForDeletion(ctx, requestID); err != nil {
		return nil, err
	}
	if _, err := s.deps.Children.ChildrenForRequest(ctx, requestID, true); err != nil {
		return nil, fmt.Errorf("lock enrollment request children for deletion: %w", err)
	}
	impact, err := s.deps.Preview.PreviewRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if impact == nil || impact.Counts.Requests != 1 {
		return nil, enrollment.ErrEnrollmentDeletionNotFound
	}
	if len(impact.BlockingStudentIDs) > 0 {
		return nil, enrollment.ErrEnrollmentDeletionStudentExists
	}
	if err := s.deleteRequestTree(ctx, impact, requestID); err != nil {
		return nil, err
	}
	return impact, s.auditDeletion(ctx, impact, actorAccountID, reason, DeletionScopeRequest)
}

// deleteRequestTree counts and cancels the request's delivery intents and
// deletes the request with everything under it.
func (s *Deletions) deleteRequestTree(ctx context.Context, impact *enrollment.DeletionImpact, requestID int64) error {
	if err := s.addDeliveryCount(ctx, impact); err != nil {
		return err
	}
	if _, err := s.deps.Delivery.CancelRelatedEmails(ctx, enrollmentRequestDeliveryType, requestID, "enrollment request deleted"); err != nil {
		return fmt.Errorf("cancel enrollment delivery intents: %w", err)
	}
	return s.deps.Children.DeleteRequestTree(ctx, requestID)
}

// DeleteChild deletes one child; deleting the request's only child deletes
// the request.
func (s *Deletions) DeleteChild(ctx context.Context, requestID, childID, actorAccountID int64, reason string) (*enrollment.DeletionImpact, error) {
	reason, err := validateEnrollmentDeletionInput(actorAccountID, reason)
	if err != nil {
		return nil, err
	}
	if err = s.validateConfigured(); err != nil {
		return nil, err
	}
	var impact *enrollment.DeletionImpact
	err = s.deps.Runtime.Transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var txErr error
		impact, txErr = s.deleteChild(txCtx, requestID, childID, actorAccountID, reason)
		return txErr
	})
	if err != nil {
		return nil, err
	}
	s.deps.Logger.InfoContext(ctx, "enrollment child deleted",
		slog.Int64("request_id", requestID),
		slog.Int64("child_id", childID),
		slog.Int64("actor_account_id", actorAccountID),
		slog.Bool("request_deleted", impact.DeletesRequest),
		slog.Int("records_affected", impact.Counts.Total()))
	return impact, nil
}

func (s *Deletions) deleteChild(ctx context.Context, requestID, childID, actorAccountID int64, reason string) (*enrollment.DeletionImpact, error) {
	if err := s.lockRequestForDeletion(ctx, requestID); err != nil {
		return nil, err
	}
	children, err := s.deps.Children.ChildrenForRequest(ctx, requestID, true)
	if err != nil {
		return nil, fmt.Errorf("lock enrollment request children for deletion: %w", err)
	}
	child := deletionRequestChildByID(children, childID)
	if child == nil {
		return nil, enrollment.ErrEnrollmentDeletionNotFound
	}
	if err := validateDeletableEnrollmentChild(child); err != nil {
		return nil, err
	}
	impact, err := s.deps.Preview.PreviewChild(ctx, requestID, childID)
	if err != nil {
		return nil, err
	}
	if impact == nil || impact.Counts.RequestChildren != 1 {
		return nil, enrollment.ErrEnrollmentDeletionNotFound
	}
	if len(impact.BlockingStudentIDs) > 0 {
		return nil, enrollment.ErrEnrollmentDeletionStudentExists
	}
	if impact.DeletesRequest {
		err = s.deleteRequestTree(ctx, impact, requestID)
	} else {
		err = s.deps.Children.DeleteRequestChildTree(ctx, requestID, childID)
	}
	if err != nil {
		return nil, err
	}
	return impact, s.auditDeletion(ctx, impact, actorAccountID, reason, DeletionScopeChild)
}

func (s *Deletions) lockRequestForDeletion(ctx context.Context, requestID int64) error {
	_, err := s.deps.Requests.RequestByID(ctx, requestID, true)
	if err != nil && s.deps.Runtime.NotFound(err) {
		return enrollment.ErrEnrollmentDeletionNotFound
	}
	if err != nil {
		return fmt.Errorf("lock enrollment request for deletion: %w", err)
	}
	return nil
}

func (s *Deletions) auditDeletion(ctx context.Context, impact *enrollment.DeletionImpact, actorAccountID int64, reason string, scope DeletionScope) error {
	if err := s.deps.Audit.RecordEnrollmentDeletion(ctx, EnrollmentDeletionEvent{
		RequestID:      impact.RequestID,
		ChildID:        impact.ChildID,
		ActorAccountID: &actorAccountID,
		Actor:          DeletionActorAdmin,
		Scope:          scope,
		Reason:         reason,
		Counts:         impact.Counts,
		DeletedAt:      time.Now(),
	}); err != nil {
		return fmt.Errorf("audit enrollment deletion: %w", err)
	}
	return nil
}

func (s *Deletions) validateConfigured() error {
	d := s.deps
	if d.Requests == nil || d.Children == nil || d.Preview == nil || d.Audit == nil || d.Runtime.Transactions == nil || d.Delivery == nil {
		return errors.New("enrollment deletion service is not configured")
	}
	return nil
}

func (s *Deletions) addDeliveryCount(ctx context.Context, impact *enrollment.DeletionImpact) error {
	count, err := s.deps.Delivery.CountRelatedEmails(ctx, enrollmentRequestDeliveryType, impact.RequestID)
	if err != nil {
		return fmt.Errorf("count enrollment delivery intents: %w", err)
	}
	impact.Counts.EmailOutbox = count
	return nil
}

func validateEnrollmentDeletionInput(actorAccountID int64, reason string) (string, error) {
	if actorAccountID <= 0 {
		return "", errors.New("actor account id is required for enrollment deletion")
	}
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) < 3 || len([]rune(reason)) > 500 {
		return "", enrollment.ErrEnrollmentDeletionInvalidReason
	}
	return reason, nil
}

func validateDeletableEnrollmentChild(child *enrollment.RequestChild) error {
	if child == nil {
		return enrollment.ErrEnrollmentDeletionNotFound
	}
	if child.CreatedStudentID != nil && *child.CreatedStudentID > 0 {
		return enrollment.ErrEnrollmentDeletionStudentExists
	}
	switch child.Status {
	case enrollmentModels.ChildStatusRejected,
		enrollmentModels.ChildStatusWithdrawn,
		enrollmentModels.ChildStatusApproved:
		return nil
	default:
		return enrollment.ErrEnrollmentDeletionNotAllowed
	}
}

func deletionRequestChildByID(children []*enrollment.RequestChild, childID int64) *enrollment.RequestChild {
	for _, child := range children {
		if child != nil && child.ID == childID {
			return child
		}
	}
	return nil
}
