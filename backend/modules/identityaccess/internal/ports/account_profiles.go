package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type AccountProfileStore interface {
	FindAccountMetadata(context.Context, int64) (domain.AccountMetadata, bool, domain.OperationStats, error)
	SetAccountUsername(context.Context, int64, string) (bool, domain.OperationStats, error)
	SetAccountAvatar(context.Context, int64, string) (bool, domain.OperationStats, error)
	FindAccountProfile(context.Context, int64, int64) (domain.AccountProfile, bool, domain.OperationStats, error)
	SetAccountBio(context.Context, int64, int64, string) (domain.OperationStats, error)
}
