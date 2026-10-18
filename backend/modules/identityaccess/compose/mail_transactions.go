package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The mail seams the legacy composition binds for the invitation, password
// reset and operator link flows join the flow's tenant transaction.
// What they do with that transaction is decided here, where the tenant
// runtime is reachable (#2747).

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

// NewSavepointGuardianEnrollments serves the acceptance's enrollment claim
// over backfill. The claim runs inside a savepoint: it is best-effort, and a
// failed statement must not leave the acceptance's transaction aborted. A
// savepoint that could not be controlled is the exception — the owner aborts
// the acceptance rather than commit on top of a transaction whose state
// nobody knows.
func NewSavepointGuardianEnrollments(backfill identityPorts.GuardianEnrollmentBackfill) identityPorts.GuardianEnrollments {
	return savepointGuardianEnrollments{backfill: backfill}
}

type savepointGuardianEnrollments struct {
	backfill identityPorts.GuardianEnrollmentBackfill
}

func (e savepointGuardianEnrollments) ClaimGuardianEnrollments(ctx context.Context, accountID int64, email string) (int, error) {
	claimed := 0
	err := tenant.WithSavepoint(ctx, func(savepointCtx context.Context) error {
		var claimErr error
		claimed, claimErr = e.backfill.BackfillGuardianAccountID(savepointCtx, accountID, email)
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
