package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// RunMailStepInSavepoint runs a best-effort step of a welcome mail in its own
// savepoint of the flow's transaction, so a failed statement cannot abort
// it. The step reports its own failure; only a savepoint that could not be
// controlled, or a missing transaction, is returned.
func RunMailStepInSavepoint(ctx context.Context, step func(context.Context) error) error {
	if err := tenant.WithSavepoint(ctx, step); errors.Is(err, tenant.ErrSavepointControl) {
		return err
	}
	return nil
}

// RunMailStep is RunMailStepInSavepoint for a step that may also run
// without a transaction: then there is nothing to protect and the step runs
// directly.
func RunMailStep(ctx context.Context, step func(context.Context) error) error {
	if _, inTransaction := tenant.TransactionFromContext(ctx); !inTransaction {
		_ = step(ctx)
		return nil
	}
	return RunMailStepInSavepoint(ctx, step)
}

// RunInSavepoint runs fn in a savepoint of the transaction in ctx and
// returns its error after rolling back its writes. A savepoint that could
// not be controlled leaves the transaction unusable.
func RunInSavepoint(ctx context.Context, fn func(context.Context) error) error {
	err := tenant.WithSavepoint(ctx, fn)
	if errors.Is(err, tenant.ErrSavepointControl) {
		return fmt.Errorf("%w: %w", domain.ErrTransactionUnusable, err)
	}
	return err
}

type SavepointGuardianEnrollments struct {
	Backfill identityPorts.GuardianEnrollmentBackfill
}

func (e SavepointGuardianEnrollments) ClaimGuardianEnrollments(ctx context.Context, accountID int64, email string) (int, error) {
	claimed := 0
	err := tenant.WithSavepoint(ctx, func(savepointCtx context.Context) error {
		var claimErr error
		claimed, claimErr = e.Backfill.BackfillGuardianAccountID(savepointCtx, accountID, email)
		return claimErr
	})
	if errors.Is(err, tenant.ErrSavepointControl) {
		return 0, fmt.Errorf("%w: %w", identityaccess.ErrTransactionUnusable, err)
	}
	if err != nil {
		return 0, err
	}
	return claimed, nil
}
