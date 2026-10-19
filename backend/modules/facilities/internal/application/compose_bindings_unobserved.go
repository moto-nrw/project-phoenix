package application

import (
	context "context"
)

// UnusedRooms answers that no session and no care offering needs a room.
type UnusedRooms struct{}

func (UnusedRooms) HasActiveSessions(context.Context, int64) (bool, error)      { return false, nil }
func (UnusedRooms) RequiredByCareOffering(context.Context, int64) (bool, error) { return false, nil }
