package ports

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

type AccountProfileStore interface {
	FindAccountPortalLocale(context.Context, int64) (*string, bool, domain.OperationStats, error)
	SetAccountPortalLocale(context.Context, int64, string) (bool, domain.OperationStats, error)
	FindAccountMetadata(context.Context, int64) (domain.AccountMetadata, bool, domain.OperationStats, error)
	SetAccountUsername(context.Context, int64, string) (bool, domain.OperationStats, error)
	SetAccountAvatar(context.Context, int64, string) (bool, domain.OperationStats, error)
	FindAccountProfile(context.Context, int64, int64) (domain.AccountProfile, bool, domain.OperationStats, error)
	SetAccountBio(context.Context, int64, int64, string) (domain.OperationStats, error)
}
