package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
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
		Usage:   unusedRooms{},
		Observe: func(Observation) {},
	})
}

// unusedRooms answers that no session and no care offering needs a room.
type unusedRooms struct{}

func (unusedRooms) HasActiveSessions(context.Context, int64) (bool, error)      { return false, nil }
func (unusedRooms) RequiredByCareOffering(context.Context, int64) (bool, error) { return false, nil }
