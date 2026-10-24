package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// RejectedRequestCleaner finds and deletes fully rejected requests.
type RejectedRequestCleaner interface {
	DeleteRequestTree(context.Context, int64) error
	FullyRejectedRequestsBefore(ctx context.Context, cutoff time.Time) ([]int64, error)
	SettledRequestsBefore(ctx context.Context, withdrawnCutoff, rejectedCutoff time.Time) ([]int64, error)
	RequestByID(context.Context, int64, bool) (*enrollmentNative.Request, error)
	DeleteRequest(ctx context.Context, requestID int64) error
}

// RejectedChildren locks the children of a request.
type RejectedChildren interface {
	ChildrenForRequest(ctx context.Context, requestID int64, forUpdate bool) ([]*enrollmentNative.RequestChild, error)
}

// UsedLateInviteCleaner deletes the late invites a request used, and the
// invitations settled before a cutoff.
type UsedLateInviteCleaner interface {
	DeleteLateInvitesByUsedRequestID(ctx context.Context, requestID int64) (int64, error)
	DeleteLateInvitesSettledBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// RetentionSettings resolves the tenant's retention of rejected and of
// withdrawn enrollments in days.
type RetentionSettings interface {
	RejectedRetentionDays(ctx context.Context) (int, error)
	WithdrawnRetentionDays(ctx context.Context) (int, error)
}

// RejectedCleanupDependencies bind the retention worker to its owners.
// Preview and Audit connect it to the same impact calculation and
// append-only audit trail as the manual deletion; they are optional for
// focused unit tests of the historical path, the production composition
// always supplies both.
type RejectedCleanupDependencies struct {
	Requests    RejectedRequestCleaner
	Children    RejectedChildren
	LateInvites UsedLateInviteCleaner
	Delivery    EnrollmentDeletionDelivery
	Settings    RetentionSettings
	Preview     *DeletionPreview
	Audit       EnrollmentDeletionAudit
	Runtime     Runtime
	Logger      *slog.Logger
}

// RejectedCleanup removes rejected enrollment data after its
// tenant-configured retention window.
type RejectedCleanup struct {
	deps RejectedCleanupDependencies
}

var _ enrollmentNative.RejectedEnrollmentCleaner = (*RejectedCleanup)(nil)

// NewRejectedCleanup composes the retention worker.
func NewRejectedCleanup(deps RejectedCleanupDependencies) *RejectedCleanup {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &RejectedCleanup{deps: deps}
}

// runInTx joins the ambient transaction through a savepoint, or opens a
// transaction of the tenant in context.
func (s *RejectedCleanup) runInTx(ctx context.Context, fn func(context.Context) error) error {
	if s.deps.Runtime.InTransaction(ctx) {
		return s.deps.Runtime.Savepoint(ctx, fn)
	}
	return s.deps.Runtime.WithinCurrentTenant(ctx, fn)
}

// CleanupRejectedEnrollments deletes every request whose children were all
// rejected before the retention cutoff.
func (s *RejectedCleanup) CleanupRejectedEnrollments(ctx context.Context) (enrollmentNative.RejectedEnrollmentCleanupResult, error) {
	d := s.deps
	if d.Requests == nil || d.Children == nil || d.LateInvites == nil || d.Delivery == nil || d.Settings == nil || d.Runtime.InTransaction == nil {
		return enrollmentNative.RejectedEnrollmentCleanupResult{}, errors.New("rejected enrollment cleanup is not configured")
	}
	days, err := d.Settings.RejectedRetentionDays(ctx)
	if err != nil {
		return enrollmentNative.RejectedEnrollmentCleanupResult{}, fmt.Errorf("resolve rejected enrollment retention: %w", err)
	}
	if days < 0 {
		return enrollmentNative.RejectedEnrollmentCleanupResult{}, fmt.Errorf("invalid rejected enrollment retention days: %d", days)
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	result := enrollmentNative.RejectedEnrollmentCleanupResult{}
	err = s.runInTx(ctx, func(txCtx context.Context) error {
		requestIDs, listErr := d.Requests.FullyRejectedRequestsBefore(txCtx, cutoff)
		if listErr != nil {
			return listErr
		}
		settled := func(children []*enrollmentNative.RequestChild) bool {
			return childrenRemainFullyRejectedBefore(children, cutoff)
		}
		for _, requestID := range requestIDs {
			if err := s.cleanupRequest(txCtx, requestID, settled, &result); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return enrollmentNative.RejectedEnrollmentCleanupResult{}, err
	}
	d.Logger.InfoContext(ctx, "rejected enrollment cleanup completed",
		slog.Int("deleted_requests", result.DeletedRequests),
		slog.Int64("deleted_late_invites", result.DeletedLateInvites),
		slog.Int64("deleted_outbox_rows", result.DeletedOutboxRows))
	return result, nil
}

// cleanupRequest deletes one request whose locked children are still
// settled. It matches every request-mutation path's parent -> children lock
// order: locking children first and then cascading the parent delete can
// deadlock with an edit that already holds the request row.
func (s *RejectedCleanup) cleanupRequest(ctx context.Context, requestID int64, settled func([]*enrollmentNative.RequestChild) bool, result *enrollmentNative.RejectedEnrollmentCleanupResult) error {
	d := s.deps
	if _, err := d.Requests.RequestByID(ctx, requestID, true); err != nil {
		return fmt.Errorf("lock rejected enrollment request: %w", err)
	}
	children, err := d.Children.ChildrenForRequest(ctx, requestID, true)
	if err != nil {
		return fmt.Errorf("lock rejected enrollment request children: %w", err)
	}
	if !settled(children) {
		return nil
	}
	emailCount, err := d.Delivery.CountRelatedEmails(ctx, enrollmentRequestDeliveryType, requestID)
	if err != nil {
		return fmt.Errorf("count rejected enrollment delivery rows: %w", err)
	}
	if d.Preview == nil && d.Audit == nil {
		return s.deleteUnaudited(ctx, requestID, result)
	}
	if d.Preview == nil || d.Audit == nil {
		return errors.New("rejected enrollment cleanup audit dependencies are incomplete")
	}
	impact, err := d.Preview.PreviewRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("preview rejected enrollment cleanup: %w", err)
	}
	if impact == nil || impact.Counts.Requests != 1 {
		return fmt.Errorf("preview rejected enrollment cleanup request %d: request not found", requestID)
	}
	if len(impact.BlockingStudentIDs) > 0 {
		d.Logger.WarnContext(ctx, "rejected enrollment cleanup skipped request linked to existing student",
			slog.Int64("request_id", requestID),
			slog.Int("blocking_students", len(impact.BlockingStudentIDs)))
		return nil
	}
	impact.Counts.EmailOutbox = emailCount
	return s.deleteAudited(ctx, requestID, impact, result)
}

// deleteAudited cancels the request's mails, deletes the request tree and
// appends the system deletion audit row.
func (s *RejectedCleanup) deleteAudited(ctx context.Context, requestID int64, impact *enrollmentNative.DeletionImpact, result *enrollmentNative.RejectedEnrollmentCleanupResult) error {
	d := s.deps
	cancelled, err := d.Delivery.CancelRelatedEmails(ctx, enrollmentRequestDeliveryType, requestID, "related enrollment data deleted")
	if err != nil {
		return fmt.Errorf("cancel rejected enrollment delivery rows: %w", err)
	}
	if err := d.Requests.DeleteRequestTree(ctx, requestID); err != nil {
		return fmt.Errorf("delete rejected enrollment request dependencies: %w", err)
	}
	if err := d.Audit.RecordEnrollmentDeletion(ctx, EnrollmentDeletionEvent{
		RequestID: requestID,
		Actor:     DeletionActorSystem,
		Scope:     DeletionScopeRequest,
		Reason:    "Automatische Löschung nach Ablauf der Aufbewahrungsfrist",
		Counts:    impact.Counts,
		DeletedAt: time.Now(),
	}); err != nil {
		return fmt.Errorf("audit rejected enrollment cleanup: %w", err)
	}
	result.DeletedLateInvites += int64(impact.Counts.LateInvites)
	result.DeletedOutboxRows += cancelled
	result.DeletedRequests++
	return nil
}

// deleteUnaudited is the compatibility path for focused unit tests that
// compose the worker without the production audit dependencies.
func (s *RejectedCleanup) deleteUnaudited(ctx context.Context, requestID int64, result *enrollmentNative.RejectedEnrollmentCleanupResult) error {
	d := s.deps
	deletedLateInvites, err := d.LateInvites.DeleteLateInvitesByUsedRequestID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("delete used enrollment late invites: %w", err)
	}
	cancelled, err := d.Delivery.CancelRelatedEmails(ctx, enrollmentRequestDeliveryType, requestID, "related enrollment data deleted")
	if err != nil {
		return fmt.Errorf("cancel rejected enrollment delivery rows: %w", err)
	}
	if err := d.Requests.DeleteRequest(ctx, requestID); err != nil {
		return fmt.Errorf("delete rejected enrollment request: %w", err)
	}
	result.DeletedLateInvites += deletedLateInvites
	result.DeletedOutboxRows += cancelled
	result.DeletedRequests++
	return nil
}

func childrenRemainFullyRejectedBefore(children []*enrollmentNative.RequestChild, cutoff time.Time) bool {
	if len(children) == 0 {
		return false
	}
	for _, child := range children {
		if child == nil || child.Status != enrollmentNative.ChildStatusRejected || child.ReviewedAt == nil || !child.ReviewedAt.Before(cutoff) {
			return false
		}
	}
	return true
}
