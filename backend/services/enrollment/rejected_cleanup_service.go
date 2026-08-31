package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type RejectedEnrollmentCleanupResult struct {
	DeletedRequests    int
	DeletedLateInvites int64
	DeletedOutboxRows  int64
}

// RejectedEnrollmentCleaner removes rejected enrollment data after its
// tenant-configured retention window.
type RejectedEnrollmentCleaner interface {
	CleanupRejectedEnrollments(ctx context.Context) (RejectedEnrollmentCleanupResult, error)
}

type relatedOutboxCleaner interface {
	DeleteByRelatedEntity(ctx context.Context, relatedType string, relatedID int64) (int64, error)
}

type rejectedRequestCleaner interface {
	ListFullyRejectedBefore(ctx context.Context, cutoff time.Time) ([]int64, error)
	FindByIDForUpdate(ctx context.Context, requestID int64) (*enrollmentModels.Request, error)
	DeleteByID(ctx context.Context, requestID int64) error
}

type rejectedRequestChildLocker interface {
	ListByRequestIDForUpdate(ctx context.Context, requestID int64) ([]*enrollmentModels.RequestChild, error)
}

type usedLateInviteCleaner interface {
	DeleteByUsedRequestID(ctx context.Context, requestID int64) (int64, error)
}

type rejectedEnrollmentCleanupService struct {
	requests    rejectedRequestCleaner
	children    rejectedRequestChildLocker
	lateInvites usedLateInviteCleaner
	outbox      relatedOutboxCleaner
	settings    RequestSettingsResolver
	deletion    enrollmentModels.DeletionRepository
	audit       auditModels.EnrollmentDeletionRepository
	runInTx     func(context.Context, func(context.Context) error) error
	logger      *slog.Logger
}

// RejectedEnrollmentCleanupAuditDependencies connects the retention worker to
// the same impact calculation and append-only audit trail as manual deletion.
// It is optional at the constructor boundary for backwards-compatible unit
// stubs; the production factory always supplies both repositories.
type RejectedEnrollmentCleanupAuditDependencies struct {
	Deletion enrollmentModels.DeletionRepository
	Audit    auditModels.EnrollmentDeletionRepository
}

func NewRejectedEnrollmentCleanupService(
	requests enrollmentModels.RequestRepository,
	children enrollmentModels.RequestChildRepository,
	lateInvites enrollmentModels.LateInviteRepository,
	outbox relatedOutboxCleaner,
	settings RequestSettingsResolver,
	db *bun.DB,
	logger *slog.Logger,
	auditDependencies ...RejectedEnrollmentCleanupAuditDependencies,
) RejectedEnrollmentCleaner {
	if logger == nil {
		logger = slog.Default()
	}
	service := &rejectedEnrollmentCleanupService{
		requests:    requests,
		children:    children,
		lateInvites: lateInvites,
		outbox:      outbox,
		settings:    settings,
		runInTx:     newRejectedEnrollmentCleanupTxRunner(db),
		logger:      logger,
	}
	if len(auditDependencies) > 0 {
		service.deletion = auditDependencies[0].Deletion
		service.audit = auditDependencies[0].Audit
	}
	return service
}

func newRejectedEnrollmentCleanupTxRunner(db *bun.DB) func(context.Context, func(context.Context) error) error {
	return func(ctx context.Context, fn func(context.Context) error) error {
		if _, ok := tenant.TransactionFromContext(ctx); ok {
			return tenant.WithSavepoint(ctx, fn)
		}
		return tenant.WithinCurrentTenant(ctx, fn)
	}
}

func (s *rejectedEnrollmentCleanupService) CleanupRejectedEnrollments(ctx context.Context) (RejectedEnrollmentCleanupResult, error) {
	if s.requests == nil || s.children == nil || s.lateInvites == nil || s.outbox == nil || s.settings == nil || s.runInTx == nil {
		return RejectedEnrollmentCleanupResult{}, errors.New("rejected enrollment cleanup is not configured")
	}
	days, err := s.settings.ResolveInt(ctx, configModel.KeyEnrollmentRejectedRetentionDays)
	if err != nil {
		return RejectedEnrollmentCleanupResult{}, fmt.Errorf("resolve rejected enrollment retention: %w", err)
	}
	if days < 0 {
		return RejectedEnrollmentCleanupResult{}, fmt.Errorf("invalid rejected enrollment retention days: %d", days)
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	result := RejectedEnrollmentCleanupResult{}
	err = s.runInTx(ctx, func(txCtx context.Context) error {
		requestIDs, listErr := s.requests.ListFullyRejectedBefore(txCtx, cutoff)
		if listErr != nil {
			return listErr
		}
		for _, requestID := range requestIDs {
			// Match every request-mutation path's parent -> children lock order.
			// Locking children first and then cascading the parent delete can
			// deadlock with an edit that already holds the request row.
			if _, lockErr := s.requests.FindByIDForUpdate(txCtx, requestID); lockErr != nil {
				return fmt.Errorf("lock rejected enrollment request: %w", lockErr)
			}
			children, lockErr := s.children.ListByRequestIDForUpdate(txCtx, requestID)
			if lockErr != nil {
				return fmt.Errorf("lock rejected enrollment request children: %w", lockErr)
			}
			if !childrenRemainFullyRejectedBefore(children, cutoff) {
				continue
			}
			var impact *enrollmentModels.DeletionImpact
			if s.deletion != nil || s.audit != nil {
				if s.deletion == nil || s.audit == nil {
					return errors.New("rejected enrollment cleanup audit dependencies are incomplete")
				}
				var previewErr error
				impact, previewErr = s.deletion.PreviewRequest(txCtx, requestID)
				if previewErr != nil {
					return fmt.Errorf("preview rejected enrollment cleanup: %w", previewErr)
				}
				if impact == nil || impact.Counts.Requests != 1 {
					return fmt.Errorf("preview rejected enrollment cleanup request %d: request not found", requestID)
				}
				if len(impact.BlockingStudentIDs) > 0 {
					s.logger.WarnContext(txCtx, "rejected enrollment cleanup skipped request linked to existing student",
						slog.Int64("request_id", requestID),
						slog.Int("blocking_students", len(impact.BlockingStudentIDs)))
					continue
				}
			}
			if impact != nil {
				// Production uses the same explicit cross-schema deletion as the
				// manual workflow. This is essential for polymorphic rows such as
				// platform.email_outbox, which no foreign key can cascade.
				if deleteErr := s.deletion.DeleteRequest(txCtx, requestID); deleteErr != nil {
					return fmt.Errorf("delete rejected enrollment request dependencies: %w", deleteErr)
				}
				event := &auditModels.EnrollmentDeletion{
					RequestID: requestID,
					ActorType: auditModels.EnrollmentDeletionActorSystem,
					Scope:     auditModels.EnrollmentDeletionScopeRequest,
					Reason:    "Automatische Löschung nach Ablauf der Aufbewahrungsfrist",
					Counts:    impact.Counts,
					DeletedAt: time.Now(),
				}
				if auditErr := s.audit.Create(txCtx, event); auditErr != nil {
					return fmt.Errorf("audit rejected enrollment cleanup: %w", auditErr)
				}
				result.DeletedLateInvites += int64(impact.Counts.LateInvites)
				result.DeletedOutboxRows += int64(impact.Counts.EmailOutbox)
			} else {
				// Compatibility path for focused unit tests that construct the
				// historical service without the production audit dependencies.
				deletedLateInvites, deleteLateInvitesErr := s.lateInvites.DeleteByUsedRequestID(txCtx, requestID)
				if deleteLateInvitesErr != nil {
					return fmt.Errorf("delete used enrollment late invites: %w", deleteLateInvitesErr)
				}
				deletedOutbox, deleteOutboxErr := s.outbox.DeleteByRelatedEntity(txCtx, platformModels.EmailRelatedTypeEnrollmentRequest, requestID)
				if deleteOutboxErr != nil {
					return fmt.Errorf("delete dependent enrollment outbox rows: %w", deleteOutboxErr)
				}
				if deleteRequestErr := s.requests.DeleteByID(txCtx, requestID); deleteRequestErr != nil {
					return fmt.Errorf("delete rejected enrollment request: %w", deleteRequestErr)
				}
				result.DeletedLateInvites += deletedLateInvites
				result.DeletedOutboxRows += deletedOutbox
			}
			result.DeletedRequests++
		}
		return nil
	})
	if err != nil {
		return RejectedEnrollmentCleanupResult{}, err
	}
	s.logger.InfoContext(ctx, "rejected enrollment cleanup completed",
		slog.Int("deleted_requests", result.DeletedRequests),
		slog.Int64("deleted_late_invites", result.DeletedLateInvites),
		slog.Int64("deleted_outbox_rows", result.DeletedOutboxRows))
	return result, nil
}

func childrenRemainFullyRejectedBefore(children []*enrollmentModels.RequestChild, cutoff time.Time) bool {
	if len(children) == 0 {
		return false
	}
	for _, child := range children {
		if child == nil || child.Status != enrollmentModels.ChildStatusRejected || child.ReviewedAt == nil || !child.ReviewedAt.Before(cutoff) {
			return false
		}
	}
	return true
}
