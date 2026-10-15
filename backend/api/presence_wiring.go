package api

import (
	"errors"
	"log/slog"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	devicefleetLegacy "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/legacy"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// PresenceWiringOwners are the owners the root has composed when it builds
// the Student Presence, Facilities, Device Fleet and class-day wiring of the
// legacy service graph (#4010).
type PresenceWiringOwners struct {
	DB         transactionruntime.Database
	Logger     *slog.Logger
	Persons    peopledirectory.Capability
	Rooms      facilitiesModule.Capability
	Membership schoolmembership.Capability
	CarePlan   careplan.Capability
	Calendar   schoolcalendar.Calendar
	Timetable  timetable.Capability
	// TimetableRows are the retained Timetable adapters the attendance and
	// room reads project activities from.
	TimetableRows TimetableRepositories
	// People are the retained People Directory, School Membership and School
	// Structure rows the attendance reads.
	People services.PeopleWiring
	// Now fixes the clock of the session records, the status days and the
	// statistics; nil is the Berlin clock.
	Now func() time.Time
}

// NewPresenceWiring builds Student Presence, its session records, the
// attendance reads, the retained room service and the class-day bindings
// over the owners. The legacy service graph takes the result in place of
// building it itself (ADR 0053 point 8).
func NewPresenceWiring(owners PresenceWiringOwners) (services.PresenceWiring, error) {
	if owners.Persons == nil || owners.Rooms == nil || owners.Membership == nil || owners.CarePlan == nil ||
		owners.Calendar == nil || owners.Timetable == nil || owners.TimetableRows.ActivityGroup == nil || owners.People.Staff == nil {
		return services.PresenceWiring{}, errors.New("presence wiring: the People Directory, Facilities, School Membership, Care Plan, School Calendar and Timetable owners and their retained rows are required")
	}
	logger := owners.Logger
	if logger == nil {
		logger = slog.Default()
	}
	db := owners.DB
	devices, err := services.NewDeviceFleet(db.Pool())
	if err != nil {
		return services.PresenceWiring{}, err
	}
	presence := NewLoggedStudentPresence(db, logger)
	staff := owners.People.Staff
	sessionDeviceRecords := devicefleetRepositoryAdapter.NewDeviceRepository(devices)
	return services.PresenceWiring{
		Presence:         presence,
		UnloggedPresence: NewUnloggedStudentPresence(db),
		SessionRecords: NewSessionRecords(db, SessionRecordOwners{
			Now: owners.Now, Devices: devices, Rooms: owners.Rooms, Timetable: owners.Timetable,
			Membership: owners.Membership, Persons: owners.Persons,
		}),
		PartialAbsencePreview:     NewPartialAbsencePreview(db),
		AttendanceStaff:           NewAttendanceStaffDirectory(staff),
		AttendanceEducationGroups: NewAttendanceEducationGroups(owners.People.Groups, owners.People.Students),
		AttendanceActivities:      NewSessionActivities(owners.TimetableRows.GroupReads()),
		RoomService: facilitiesCompose.NewRoomService(facilitiesCompose.RoomServiceConfig{
			Rooms:     owners.Rooms,
			Occupancy: facilitiesLegacy.OccupancyProjection(RoomOccupancyPresence{presence}, owners.TimetableRows.ActivityGroup, owners.Membership, owners.Persons),
			History:   facilitiesLegacy.HistoryProjection(RoomHistoryPresence{presence}, owners.TimetableRows.ActivityGroup, owners.Membership, owners.Persons),
		}),
		OpenGroups:       facilitiesLegacy.OpenGroupCatalog(FacilitiesGroupSupervisions(presence), FacilitiesRoomSessions(presence), FacilitiesGroupVisits(presence)),
		OpenRoomSessions: OpenRoomSessionPresence{Presence: presence, Activities: owners.Timetable},
		OpenRooms:        OpenRoomDirectory{Rooms: owners.Rooms},
		StatusDays: presenceservice.NewStatusDays(
			NewStudentStatusDayRepository(owners.CarePlan), NewManualPartialAbsenceDates(owners.CarePlan),
			db.Pool(), owners.CarePlan.LockExceptionDay, owners.Now,
		),
		ClassListEntries: NewClassListEntryRosterReader(owners.Membership),
		SessionDevices: func(settings devicefleetLegacy.SettingsResolver) presenceservice.SessionDeviceDirectory {
			return NewSessionDeviceDirectory(sessionDeviceRecords, settings, logger.With("service", "active"))
		},
		BindClassListEntries: func(audit auditModels.Command) error {
			return BindClassListEntryAdministration(owners.Membership, owners.Persons, audit)
		},
		AttendanceStaffNames: func(people *peopleCompose.PersonDirectory) presenceservice.AttendanceStaffNames {
			return NewAttendanceStaffNames(staff, people)
		},
		ClassDay: NewClassDay,
		Statistics: newStatisticsReports(statisticsOwners{
			DB: db, CarePlan: owners.CarePlan, Calendar: owners.Calendar, Students: owners.People.Students,
			Rooms: owners.Rooms, Logger: logger, Now: owners.Now,
		}),
	}, nil
}
