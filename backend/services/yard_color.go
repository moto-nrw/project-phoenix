package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

func yardRoomColorQuery(rooms facilities.Query) func(context.Context) (*string, error) {
	return func(ctx context.Context) (*string, error) {
		return facilities.SchulhofRoomColor(ctx, rooms)
	}
}
