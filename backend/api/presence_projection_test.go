package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
)

// SessionRooms returns the room rows the composed session records project onto
// the given sessions. The public contract answers with a room summary, so a
// test that asserts the full projected row reads it here instead of naming the
// module's composition itself.
func sessionRooms(ctx context.Context, records studentpresence.SessionRecords, ids []int64) (map[int64]*presenceCompose.SessionRoom, error) {
	sessions, _ := presenceCompose.SessionRepositories(records)
	groups, err := sessions.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	rooms := make(map[int64]*presenceCompose.SessionRoom, len(groups))
	for id, group := range groups {
		rooms[id] = group.Room
	}
	return rooms, nil
}

// SupervisionRowStaff returns the staff projection the composed supervision
// records attach to the stored rows of an active group, in row order. The
// public contract carries only the staff member's display name.
func supervisionRowStaff(ctx context.Context, records studentpresence.SessionRecords, activeGroupID int64) ([]*presenceCompose.SessionStaff, error) {
	_, supervisors := presenceCompose.SessionRepositories(records)
	rows, err := supervisors.FindByActiveGroupID(ctx, activeGroupID, true)
	if err != nil {
		return nil, err
	}
	staff := make([]*presenceCompose.SessionStaff, len(rows))
	for i, row := range rows {
		staff[i] = row.Staff
	}
	return staff, nil
}
