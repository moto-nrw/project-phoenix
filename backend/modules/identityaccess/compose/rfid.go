package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	internalports "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// canonicalTagStore hands the lifecycle flows the same canonical tag lookup:
// a transponder submitted as "04:a1:b2:c3" at registration finds the card
// stored as "04A1B2C3", as it did through the retained card repository.
type canonicalTagStore struct{ internalports.Store }

func (s canonicalTagStore) FindRFIDCard(ctx context.Context, tag string, tenantID int64) (string, bool, domain.OperationStats, error) {
	return s.Store.FindRFIDCard(ctx, domain.NormalizeTagID(tag), tenantID)
}
