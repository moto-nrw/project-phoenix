package compose

import (
	"context"

	applicationBindings "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// The mail seams the legacy composition binds for the invitation, password
// reset and operator link flows join the flow's tenant transaction.
// What they do with that transaction is decided here, where the tenant
// runtime is reachable (#2747).

// NewSavepointGuardianEnrollments serves the acceptance's enrollment claim
// over backfill. The claim runs inside a savepoint: it is best-effort, and a
// failed statement must not leave the acceptance's transaction aborted. A
// savepoint that could not be controlled is the exception — the owner aborts
// the acceptance rather than commit on top of a transaction whose state
// nobody knows.
func NewSavepointGuardianEnrollments(backfill identityPorts.GuardianEnrollmentBackfill) identityPorts.GuardianEnrollments {
	return applicationBindings.SavepointGuardianEnrollments{Backfill: backfill}
}

func RunMailStepInSavepoint(ctx context.Context, step func(context.Context) error) error {
	return applicationBindings.RunMailStepInSavepoint(ctx, step)
}

func RunMailStep(ctx context.Context, step func(context.Context) error) error {
	return applicationBindings.RunMailStep(ctx, step)
}
