package compose

import (
	"context"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesService "github.com/moto-nrw/project-phoenix/modules/facilities/internal/application/roomservice"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

func NewAttendanceRooms(records facilitiesService.AttendanceRoomRecords) interface {
	FindByID(context.Context, any) (*studentpresence.SessionRoom, error)
	FindByIDForUpdate(context.Context, int64) (*studentpresence.SessionRoom, error)
	FindByIDs(context.Context, []int64) ([]*studentpresence.SessionRoom, error)
	List(context.Context, map[string]any) ([]*studentpresence.SessionRoom, error)
} {
	return facilitiesService.NewAttendanceRooms(records)
}
func OccupancyProjection(groups facilitiesModule.RoomOccupancyQuery, activities facilitiesModule.ActivityGroupNames, membership schoolmembership.Query, persons peopledirectory.Query) facilitiesService.OccupancyProjection {
	return facilitiesService.NewOccupancyProjection(groups, activities, membership, persons)
}
func HistoryProjection(groups facilitiesModule.RoomHistoryQuery, activities facilitiesModule.ActivityGroupNames, membership schoolmembership.Query, persons peopledirectory.Query) facilitiesService.RoomHistoryProjection {
	return facilitiesService.NewHistoryProjection(groups, activities, membership, persons)
}
func SettingHandlers(schulhof facilitiesModule.SchulhofProvisioner, wc facilitiesModule.WCProvisioner) (facilitiesModule.SettingHandler, facilitiesModule.SettingHandler) {
	return facilitiesService.SettingHandlers(schulhof, wc)
}
func NewOpenGroupCatalog(supervisors func(context.Context, []int64) ([]facilitiesService.OpenGroupSupervisor, error), rooms func(context.Context, int64) ([]facilitiesService.OpenGroup, error), visits func(context.Context, int64) ([]facilitiesService.OpenGroupVisit, error)) facilitiesService.OpenGroupCatalog {
	return facilitiesService.NewOpenGroupCatalog(supervisors, rooms, visits)
}
