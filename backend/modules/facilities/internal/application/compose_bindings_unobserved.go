package application

import (
	context "context"

	"github.com/moto-ogs/project-phoenix/modules/facilities"
)

// UnusedRooms answers that no session and no care offering needs a room.
type UnusedRooms struct{}

func (UnusedRooms) HasActiveSessions(context.Context, int64) (bool, error)      { return false, nil }
func (UnusedRooms) RequiredByCareOffering(context.Context, int64) (bool, error) { return false, nil }

// NoRoomReferences answers that no other owner references a room. The
// database still refuses to delete a referenced room, which is then
// archived instead.
func NoRoomReferences(context.Context, int64) (facilities.RoomReferences, error) {
	return facilities.RoomReferences{}, nil
}
