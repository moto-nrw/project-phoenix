package careplanhttp

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// CareExitRestoration is this HTTP adapter's workflow entry port for the
// care-end commands that put a removed plan back: confirming a changed end
// and cancelling a planned one (#5235). Root binds it to the care-exit
// workflow, which re-plans the restored children in the same unit.
type CareExitRestoration interface {
	Confirm(ctx context.Context, token string, input careplan.CareExitInput, actorAccountID int64) (*careplan.CareExitResult, error)
	ConfirmWithdrawalCareEnd(ctx context.Context, completionID int64, token string, input careplan.CareExitInput, actorAccountID int64) (*careplan.CareExitResult, error)
	Cancel(ctx context.Context, studentIDs []int64, actorAccountID int64) (int, error)
}
