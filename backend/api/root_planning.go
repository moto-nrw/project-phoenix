package api

import (
	"context"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// FacilitiesYard are the yard and WC views of Facilities over the room
// service and the activities.
type FacilitiesYard struct {
	Schulhof facilitiesCompose.SchulhofService
	WC       facilitiesCompose.WCService
}

// buildActivities binds the activities service of the Timetable owner.
func buildActivities(owner TimetableOwner, timetableOwner timetable.Capability, sessions *presenceCompose.SessionRecords, people PeopleRows) (arrivalTimetable.ActivityService, error) {
	repos := owner.Repositories
	return arrivalTimetable.NewService(
		timetableOwner, repos.ActivityGroup, repos.ActivitySchedule, repos.ActivitySupervisor, repos.StudentEnrollment,
		sessions, NewTimetableStaff(people.Staff), NewTimetableRosterStudents(people.Students),
	)
}

// buildFacilitiesYard binds the yard and WC views over the retained room
// service (#4010); Facilities owns the room deletion decision behind its
// room usage port.
func buildFacilitiesYard(env RootEnvironment, rooms FacilitiesRooms, activities arrivalTimetable.ActivityService) FacilitiesYard {
	logger := env.Logger.With("service", "facilities")
	return FacilitiesYard{
		Schulhof: facilitiesCompose.NewSchulhofService(rooms.RoomService, FacilitiesActivityCatalog(activities), rooms.OpenGroups, logger),
		WC:       facilitiesCompose.NewWCService(rooms.RoomService, FacilitiesActivityCatalog(activities), logger),
	}
}

// buildShiftPlanning binds the Dienstplan side of Workforce (#3418):
// shifts, series, shift types, the self-service assignments and the week
// overview over the owner's native rows. The calendar period comes from the
// School Calendar; the timetable blocks, their staffing and the Angebote
// arrive in the Timetable owner's public vocabulary (#3424); the shift_moved
// Änderungsprotokoll entry (#1884) and the SSE invalidation are bound here.
func buildShiftPlanning(
	db transactionruntime.Database, env RootEnvironment, transport MailTransport, workTime workforceModule.Capability,
	people PeopleRows, calendar schoolcalendar.Days, audit AuditOwner, owner TimetableOwner, rooms facilitiesModule.Catalog,
	activities arrivalTimetable.ActivityService,
) (*workforceCompose.ShiftPlanning, error) {
	repos := owner.Repositories
	return workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce: workTime, Staff: NewPlanStaffDirectory(people.Staff), CalendarPeriods: calendar, ShiftMoveAudit: NewShiftMoveAudit(audit.Persistence.DeviationEvent),
		Instances:      arrivalTimetable.NewTimetableInstanceReads(repos.ActivityInstance),
		InstanceStaff:  arrivalTimetable.NewTimetableInstanceStaffReads(repos.InstanceStaff),
		Rooms:          facilitiesCompose.NewRoomNameDirectory(facilitiesCompose.NewRoomRecords(rooms)),
		ActivityGroups: arrivalTimetable.NewTimetableGroupReads(repos.ActivityGroup),
		WorkSchedules:  workTime,
		WorkModels:     workTime,
		Holidays:       schoolCalendarCompose.NonWorkingDays{Calendar: calendar}, NonWorkingDays: calendar,
		CategoryLinker: arrivalTimetable.ShiftTypeCategoryLinker(activities.SetCategoryShiftTypeLinks),
		DB:             db.Pool(), Broadcaster: transport.Hub, Logger: env.Logger.With("service", "shift_planning"),
		Today: env.Today,
	})
}

// payrollStatus is the one payroll-status instance: the /payroll page and
// the DATEV writers must judge completeness identically.
func payrollStatus(settings settingsCompose.BatchSettingsService, people PeopleRows) settingsCompose.PayrollStatusGetter {
	staff := people.Staff
	return settingsCompose.NewPayrollStatusService(settings, func(ctx context.Context) (int, int, error) {
		members, err := staff.List(ctx, nil)
		if err != nil {
			return 0, 0, err
		}
		numbers := make([]*string, len(members))
		for i, member := range members {
			numbers[i] = member.PersonnelNumber
		}
		return len(members), workforceModule.MissingPersonnelNumbers(numbers), nil
	})
}
