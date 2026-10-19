package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type GuardianPickupExceptions interface{ care.GuardianPickupExceptions }
type GuardianPickupExceptionsCallbacks struct {
	ApplyGuardianPickupException    func(ctx context.Context, change GuardianPickupChangeSnapshot) error
	WithdrawGuardianPickupException func(ctx context.Context, change PickupExceptionSnapshot) error
}

func NewGuardianPickupExceptions(callbacks GuardianPickupExceptionsCallbacks) GuardianPickupExceptions {
	return guardianPickupExceptions{callbacks}
}

type guardianPickupExceptions struct {
	callbacks GuardianPickupExceptionsCallbacks
}

func (a guardianPickupExceptions) ApplyGuardianPickupException(ctx context.Context, change care.GuardianPickupChange) error {
	return a.callbacks.ApplyGuardianPickupException(ctx, carePlanFromAppGuardianPickupChange(change))
}
func (a guardianPickupExceptions) WithdrawGuardianPickupException(ctx context.Context, change care.PickupException) error {
	return a.callbacks.WithdrawGuardianPickupException(ctx, carePlanFromAppPickupException(change))
}
