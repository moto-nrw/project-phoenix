package ports

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

type RFIDCardStore interface {
	LookupRFIDCard(context.Context, string, int64) (domain.RFIDCard, bool, domain.OperationStats, error)
	RegisterRFIDCard(context.Context, domain.RFIDCard, int64) (domain.OperationStats, error)
}
