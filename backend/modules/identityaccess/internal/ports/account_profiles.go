package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type AccountProfileStore interface {
	FindAccountProfile(context.Context, int64, int64) (domain.AccountProfile, bool, domain.OperationStats, error)
	SetAccountBio(context.Context, int64, int64, string) (domain.OperationStats, error)
}
