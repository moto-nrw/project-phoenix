package owners

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/devicefleet"
	facilitiesModule "github.com/moto-ogs/project-phoenix/modules/facilities"
	"github.com/moto-ogs/project-phoenix/modules/schoolstructure"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	timetableCompose "github.com/moto-ogs/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
)

// maxSchoolGroups is the School Structure listing cap; a school has far
// fewer OGS groups.
const maxSchoolGroups = 1000

// NewRoomReferences binds the Facilities room removal's reference count
// (#6845): Timetable counts its activities and blocks, School Structure the
// groups, Device Fleet the devices, and Student Presence says whether
// sessions name the room. Each owner answers with one bounded read.
func NewRoomReferences(
	db transactionruntime.Database,
	sessions interface {
		QueryLiveGroups(ctx context.Context, filter studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error)
	},
	groups schoolstructure.GroupListing,
	devices interface {
		ListDevices(ctx context.Context, filter devicefleet.DeviceFilter) ([]devicefleet.Device, error)
	},
) (func(context.Context, int64) (facilitiesModule.RoomReferences, error), error) {
	planning, err := timetableCompose.NewRoomReferenceQueries(db.Pool(), observeTimetableActivities)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, roomID int64) (facilitiesModule.RoomReferences, error) {
		counts, err := planning.CountRoomReferences(ctx, roomID)
		if err != nil {
			return facilitiesModule.RoomReferences{}, err
		}
		schoolGroups, err := groups.ListGroups(ctx, maxSchoolGroups)
		if err != nil {
			return facilitiesModule.RoomReferences{}, err
		}
		roomDevices, err := devices.ListDevices(ctx, devicefleet.DeviceFilter{RoomID: &roomID})
		if err != nil {
			return facilitiesModule.RoomReferences{}, err
		}
		roomSessions, err := sessions.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{RoomID: &roomID, Limit: 1})
		if err != nil {
			return facilitiesModule.RoomReferences{}, err
		}
		return facilitiesModule.RoomReferences{
			ActivityGroups: counts.ActivityGroups, UpcomingBlocks: counts.UpcomingBlocks,
			EducationGroups: countGroupsInRoom(schoolGroups, roomID), Devices: len(roomDevices),
			PastBlocks: counts.PastBlocks, ArchivedActivityGroups: counts.ArchivedActivityGroups,
			HasSessionHistory: len(roomSessions) > 0,
		}, nil
	}, nil
}

func countGroupsInRoom(groups []schoolstructure.Group, roomID int64) int {
	count := 0
	for _, group := range groups {
		if group.RoomID != nil && *group.RoomID == roomID {
			count++
		}
	}
	return count
}
