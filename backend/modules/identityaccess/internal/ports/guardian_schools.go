package ports

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

type GuardianSchoolStore interface {
	ListGuardianSchoolIDs(context.Context, int64) ([]int64, domain.OperationStats, error)
}
