package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type AccountRoleQueryStore interface {
	ListSchoolAccountRoleNames(context.Context, int64, int64) ([]string, domain.OperationStats, error)
	FindSystemRoleID(context.Context, string) (int64, bool, domain.OperationStats, error)
}
