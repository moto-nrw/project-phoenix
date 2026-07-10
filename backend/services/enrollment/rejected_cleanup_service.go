package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
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
	runInTx     func(context.Context, func(context.Context) error) error
	logger      *slog.Logger
}

func NewRejectedEnrollmentCleanupService(
	requests enrollmentModels.RequestRepository,
	children enrollmentModels.RequestChildRepository,
	lateInvites enrollmentModels.LateInviteRepository,
	outbox relatedOutboxCleaner,
	settings RequestSettingsResolver,
	db *bun.DB,
	logger *slog.Logger,
) RejectedEnrollmentCleaner {
	if logger == nil {
		logger = slog.Default()
	}
	return &rejectedEnrollmentCleanupService{
		requests:    requests,
		children:    children,
		lateInvites: lateInvites,
		outbox:      outbox,
		settings:    settings,
		runInTx:     newRejectedEnrollmentCleanupTxRunner(db),
		logger:      logger,
	}
}

func newRejectedEnrollmentCleanupTxRunner(db *bun.DB) func(context.Context, func(context.Context) error) error {
	tx := modelBase.NewTxHandler(db)
	return func(ctx context.Context, fn func(context.Context) error) error {
		if ambient, ok := modelBase.TxFromContext(ctx); ok {
			return runRejectedEnrollmentCleanupSavepoint(ctx, ambient, fn)
		}
		return tx.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
	}
}

func runRejectedEnrollmentCleanupSavepoint(ctx context.Context, ambient *bun.Tx, fn func(context.Context) error) error {
	if _, err := ambient.ExecContext(ctx, "SAVEPOINT enrollment_rejected_cleanup"); err != nil {
		return fmt.Errorf("create rejected enrollment cleanup savepoint: %w", err)
	}
	if err := fn(ctx); err != nil {
		if _, rollbackErr := ambient.ExecContext(ctx, "ROLLBACK TO SAVEPOINT enrollment_rejected_cleanup"); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback rejected enrollment cleanup savepoint: %w", rollbackErr))
		}
		if _, releaseErr := ambient.ExecContext(ctx, "RELEASE SAVEPOINT enrollment_rejected_cleanup"); releaseErr != nil {
			return errors.Join(err, fmt.Errorf("release rejected enrollment cleanup savepoint: %w", releaseErr))
		}
		return err
	}
	if _, err := ambient.ExecContext(ctx, "RELEASE SAVEPOINT enrollment_rejected_cleanup"); err != nil {
		return fmt.Errorf("release rejected enrollment cleanup savepoint: %w", err)
	}
	return nil
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
			result.DeletedRequests++
			result.DeletedLateInvites += deletedLateInvites
			result.DeletedOutboxRows += deletedOutbox
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
