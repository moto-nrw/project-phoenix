package legacy

import (
	"context"

	facilitiesService "github.com/moto-nrw/project-phoenix/modules/facilities/internal/application/roomservice"
)

type openGroupCatalogAdapter struct {
	supervisors func(context.Context, []int64) ([]facilitiesService.OpenGroupSupervisor, error)
	rooms       func(context.Context, int64) ([]facilitiesService.OpenGroup, error)
	visits      func(context.Context, int64) ([]facilitiesService.OpenGroupVisit, error)
}

func OpenGroupCatalog(supervisors func(context.Context, []int64) ([]facilitiesService.OpenGroupSupervisor, error), rooms func(context.Context, int64) ([]facilitiesService.OpenGroup, error), visits func(context.Context, int64) ([]facilitiesService.OpenGroupVisit, error)) facilitiesService.OpenGroupCatalog {
	return openGroupCatalogAdapter{supervisors: supervisors, rooms: rooms, visits: visits}
}

func (a openGroupCatalogAdapter) ListByRoom(ctx context.Context, roomID int64) ([]facilitiesService.OpenGroup, error) {
	return a.rooms(ctx, roomID)
}

func (a openGroupCatalogAdapter) ListSupervisors(ctx context.Context, groupIDs []int64) ([]facilitiesService.OpenGroupSupervisor, error) {
	return a.supervisors(ctx, groupIDs)
}

func (a openGroupCatalogAdapter) ListVisits(ctx context.Context, groupID int64) ([]facilitiesService.OpenGroupVisit, error) {
	return a.visits(ctx, groupID)
}
