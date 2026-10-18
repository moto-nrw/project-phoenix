package facilities

import (
	"context"
)

type openGroupCatalogAdapter struct {
	supervisors func(context.Context, []int64) ([]OpenGroupSupervisor, error)
	rooms       func(context.Context, int64) ([]OpenGroup, error)
	visits      func(context.Context, int64) ([]OpenGroupVisit, error)
}

func NewOpenGroupCatalog(supervisors func(context.Context, []int64) ([]OpenGroupSupervisor, error), rooms func(context.Context, int64) ([]OpenGroup, error), visits func(context.Context, int64) ([]OpenGroupVisit, error)) OpenGroupCatalog {
	return openGroupCatalogAdapter{supervisors: supervisors, rooms: rooms, visits: visits}
}

func (a openGroupCatalogAdapter) ListByRoom(ctx context.Context, roomID int64) ([]OpenGroup, error) {
	return a.rooms(ctx, roomID)
}

func (a openGroupCatalogAdapter) ListSupervisors(ctx context.Context, groupIDs []int64) ([]OpenGroupSupervisor, error) {
	return a.supervisors(ctx, groupIDs)
}

func (a openGroupCatalogAdapter) ListVisits(ctx context.Context, groupID int64) ([]OpenGroupVisit, error) {
	return a.visits(ctx, groupID)
}
