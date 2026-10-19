package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type DocumentCleanup struct {
	store       ports.DocumentCleanupStore
	transaction ports.Transaction
	now         func() time.Time
}

func NewDocumentCleanup(store ports.DocumentCleanupStore, transaction ports.Transaction, now func() time.Time) *DocumentCleanup {
	return &DocumentCleanup{store: store, transaction: transaction, now: now}
}

func (s *DocumentCleanup) Enqueue(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return errors.New("document cleanup: staff ID is required")
	}
	return s.transaction.RunWrite(ctx, func(txCtx context.Context) error { return s.store.EnqueueDocumentCleanup(txCtx, staffID, s.now()) })
}

func (s *DocumentCleanup) Claim(ctx context.Context, limit int) (result []domain.DocumentCleanupClaim, err error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("document cleanup: limit must be between 1 and 100")
	}
	err = s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
		result, err = s.store.ClaimDocumentCleanup(txCtx, limit, s.now())
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *DocumentCleanup) Finish(ctx context.Context, claim domain.DocumentCleanupClaim, success bool) (finished bool, err error) {
	if claim.StaffID <= 0 || claim.Token == "" {
		return false, errors.New("document cleanup: claim is required")
	}
	err = s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
		finished, err = s.store.FinishDocumentCleanup(txCtx, claim, success, s.now())
		return err
	})
	return finished, err
}

func (s *DocumentCleanup) Backlog(ctx context.Context) (result domain.DocumentCleanupBacklog, err error) {
	err = s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
		result, err = s.store.DocumentCleanupBacklog(txCtx, s.now())
		return err
	})
	return result, err
}

// ClaimPublic claims due cleanups for the public queue. The claim must commit
// on its own, so a caller inside a tenant transaction is refused.
func (s *DocumentCleanup) ClaimPublic(ctx context.Context, limit int) ([]workforce.DocumentCleanupClaim, error) {
	if _, active := tenant.TransactionFromContext(ctx); active {
		return nil, errors.New("document cleanup: claim must commit independently")
	}
	values, err := s.Claim(ctx, limit)
	if err != nil {
		return nil, err
	}
	result := make([]workforce.DocumentCleanupClaim, 0, len(values))
	for _, value := range values {
		result = append(result, workforce.DocumentCleanupClaim(value))
	}
	return result, nil
}

// FinishPublic finishes a claim of the public queue.
func (s *DocumentCleanup) FinishPublic(ctx context.Context, claim workforce.DocumentCleanupClaim, success bool) (bool, error) {
	return s.Finish(ctx, domain.DocumentCleanupClaim(claim), success)
}

// BacklogPublic reports the backlog of the public queue.
func (s *DocumentCleanup) BacklogPublic(ctx context.Context) (workforce.DocumentCleanupBacklog, error) {
	result, err := s.Backlog(ctx)
	return workforce.DocumentCleanupBacklog(result), err
}
