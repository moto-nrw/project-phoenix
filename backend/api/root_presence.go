package api

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// PresenceOwner is Student Presence as the roots compose it (#4010): the
// owner module with its operations logged at debug level and one that logs
// only its failed operations, the session and supervision records over the
// room, activity, device and staff owners, the attendance reads and the
// scheduled day statuses over Care Plan.
type PresenceOwner struct {
	Presence                  *studentpresence.Module
	UnloggedPresence          *studentpresence.Module
	SessionRecords            *presenceCompose.SessionRecords
	AttendanceStaff           presenceservice.AttendanceStaff
	AttendanceEducationGroups presenceservice.AttendanceEducationGroups
	AttendanceActivities      presenceservice.AttendanceActivityGroups
	StatusDays                studentpresence.StatusDays
}

// FacilitiesRooms are the retained room service with its occupancy and
// history over Student Presence, and the running sessions the yard reads.
type FacilitiesRooms struct {
	RoomService facilitiesCompose.RoomService
	OpenGroups  facilitiesCompose.OpenGroupCatalog
}

// buildPresenceOwner builds Student Presence, its session records and the
// attendance reads over the owners (#4010).
func buildPresenceOwner(
	db transactionruntime.Database, logger *slog.Logger, now func() time.Time, devices devicefleet.Capability,
	persons peopledirectory.Capability, rooms facilitiesModule.Query, membership schoolmembership.Capability,
	carePlan careplan.Capability, timetableOwner timetable.Capability, timetableRows TimetableRepositories,
	people PeopleRows, structure StructureRows,
) PresenceOwner {
	return PresenceOwner{
		Presence:                  NewLoggedStudentPresence(db, logger),
		UnloggedPresence:          NewUnloggedStudentPresence(db),
		SessionRecords:            NewSessionRecords(db, now, devices, rooms, timetableOwner, membership, persons),
		AttendanceStaff:           NewAttendanceStaffDirectory(people.Staff),
		AttendanceEducationGroups: NewAttendanceEducationGroups(structure.Groups, people.Students),
		AttendanceActivities:      NewSessionActivities(timetableRows.GroupReads()),
		StatusDays: presenceservice.NewStatusDays(
			NewStudentStatusDayRepository(carePlan), NewManualPartialAbsenceDates(carePlan),
			db.Pool(), carePlan.LockExceptionDay, now,
		),
	}
}

// buildFacilitiesRooms builds the retained room service and the open
// groups of the yard over Student Presence.
func buildFacilitiesRooms(presence *studentpresence.Module, rooms facilitiesModule.Catalog, timetableOwner timetable.Capability, membership schoolmembership.Capability, persons peopledirectory.Capability) FacilitiesRooms {
	return FacilitiesRooms{
		RoomService: facilitiesCompose.NewRoomService(facilitiesCompose.RoomServiceConfig{
			Rooms:     rooms,
			Occupancy: facilitiesCompose.OccupancyProjection(RoomOccupancyPresence{presence}, roomActivityNames{timetableOwner}, membership, persons),
			History:   facilitiesCompose.HistoryProjection(RoomHistoryPresence{presence}, roomActivityNames{timetableOwner}, membership, persons),
		}),
		OpenGroups: facilitiesCompose.NewOpenGroupCatalog(FacilitiesGroupSupervisions(presence), FacilitiesRoomSessions(presence), FacilitiesGroupVisits(presence)),
	}
}
