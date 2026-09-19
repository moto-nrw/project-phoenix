package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
)

func (e engine) FindRFIDCard(ctx context.Context, tag string) (string, bool, error) {
	// Reuse the owner's canonical normalization, also used by card writes and
	// IoT readers, rather than introducing another interpretation of tag IDs.
	id, found, err := e.service.FindRFIDCard(ctx, authmodels.NormalizeTagID(tag))
	return id, found, mapError(err)
}

// canonicalTagStore hands the lifecycle flows the same canonical tag lookup:
// a transponder submitted as "04:a1:b2:c3" at registration finds the card
// stored as "04A1B2C3", as it did through the retained card repository.
type canonicalTagStore struct{ ports.Store }

func (s canonicalTagStore) FindRFIDCard(ctx context.Context, tag string, tenantID int64) (string, bool, domain.OperationStats, error) {
	return s.Store.FindRFIDCard(ctx, authmodels.NormalizeTagID(tag), tenantID)
}
