package owners

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// ssePresence reads the room sessions a live-update client subscribes to.
type ssePresence struct {
	groups studentpresence.Owner
}

func (s ssePresence) GetStaffActiveGroupIDs(ctx context.Context, staffID int64) ([]int64, error) {
	return OpenRoomSessionPresence{Presence: s.groups}.GetStaffActiveGroupIDs(ctx, staffID)
}

func (s ssePresence) ListSSEGroups(ctx context.Context) ([]studentpresence.LiveGroup, error) {
	rows, err := s.groups.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{})
	if err != nil {
		return nil, &studentpresence.OperationError{Op: "ListActiveGroups", Err: fmt.Errorf("list failed: %w", err)}
	}
	return rows, nil
}
