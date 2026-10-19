package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func NewParentGuardianPickupExceptions(source careplan.GuardianPickupExceptions) parentportal.GuardianPickupExceptions {
	if source == nil {
		return nil
	}
	return parentportal.NewGuardianPickupExceptions(parentportal.GuardianPickupExceptionsCallbacks{ApplyGuardianPickupException: func(ctx context.Context, change parentportal.GuardianPickupChangeSnapshot) error {
		return source.ApplyGuardianPickupException(ctx, parentCarePlanToOwnerGuardianPickupChange(change))
	},
		WithdrawGuardianPickupException: func(ctx context.Context, change parentportal.PickupExceptionSnapshot) error {
			return source.WithdrawGuardianPickupException(ctx, parentCarePlanToOwnerPickupException(change))
		},
	})
}
