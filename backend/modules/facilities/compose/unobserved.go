package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	applicationBindings "github.com/moto-nrw/project-phoenix/modules/facilities/internal/application"
	"github.com/uptrace/bun"
)

// NewUnobserved composes the room owner for graphs that record no
// observations: CLI roots and test compositions. Production roots compose the
// module with New so runtime evidence is kept and deletions are validated.
func NewUnobserved(db *bun.DB) (facilities.Capability, error) {
	return New(Dependencies{
		DB:           db,
		DeletionLock: func(context.Context) error { return nil },
		// The legacy repository accepted deletes without service-level
		// guards; graphs without the API root keep that path usable.
		Usage:   applicationBindings.UnusedRooms{},
		Observe: func(Observation) {},
	})
}
