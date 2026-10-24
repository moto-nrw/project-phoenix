package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type GuardianFinancialDataRepository interface {
	Create(context.Context, *peopledirectory.GuardianPayment) error
	Update(context.Context, *peopledirectory.GuardianPayment) error
	FindByGuardianProfileID(context.Context, int64) (*peopledirectory.GuardianPayment, error)
	ListByGuardianProfileIDs(context.Context, []int64) (map[int64]*peopledirectory.GuardianPayment, error)
}
