package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// YardRoomColor reads the colour of the system yard for attendance.
func YardRoomColor(rooms facilities.Query) func(context.Context) (*string, error) {
	return func(ctx context.Context) (*string, error) {
		return facilities.SchulhofRoomColor(ctx, rooms)
	}
}
