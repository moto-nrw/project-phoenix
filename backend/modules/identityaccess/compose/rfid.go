package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

func (e engine) FindRFIDCard(ctx context.Context, tag string) (string, bool, error) {
	// Reuse the owner's canonical normalization, also used by card writes and
	// IoT readers, rather than introducing another interpretation of tag IDs.
	id, found, err := e.service.FindRFIDCard(ctx, auth.NormalizeTagID(tag))
	return id, found, mapError(err)
}
