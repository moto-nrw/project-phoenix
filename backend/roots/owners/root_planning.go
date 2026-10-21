package owners

import (
	"context"

	realtime "github.com/moto-nrw/project-phoenix/realtime"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

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

// BuildActivities binds the activities service of the Timetable owner.
func BuildActivities(owner TimetableOwner, timetableOwner timetable.Capability, sessions presenceCompose.SessionRecords, people PeopleRows) (activityContract.ActivityService, error) {
	repos := owner.Repositories
	return arrivalTimetable.NewService(
		timetableOwner, repos.ActivitySchedule, repos.Directories,
		sessions, NewTimetableStaff(people.Staff), NewTimetableRosterStudents(people.Students),
	)
}

// BuildShiftPlanning binds the Dienstplan side of Workforce (#3418):
// shifts, series, shift types, the self-service assignments and the week
// overview over the owner's native rows. The calendar period comes from the
// School Calendar; the timetable blocks, their staffing and the Angebote
// arrive in the Timetable owner's public vocabulary (#3424); the shift_moved
// Änderungsprotokoll entry (#1884) and the SSE invalidation are bound here.
func BuildShiftPlanning(
	db transactionruntime.Database, env RootEnvironment, hub *realtime.Hub, workTime workforceCompose.WorkTime,
	people PeopleRows, calendar schoolcalendar.Days, audit AuditOwner, owner TimetableOwner, rooms facilitiesModule.Catalog,
	activities activityContract.ActivityService,
) (*workforceCompose.ShiftPlanning, error) {
	repos := owner.Repositories
	return workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce: workTime, Staff: NewPlanStaffDirectory(people.Staff), CalendarPeriods: calendar, ShiftMoveAudit: NewShiftMoveAudit(audit.Persistence.DeviationEvent),
		Instances:      arrivalTimetable.NewTimetableInstanceReads(repos.ActivityInstance),
		InstanceStaff:  arrivalTimetable.NewTimetableInstanceStaffReads(repos.InstanceStaff),
		Rooms:          facilitiesCompose.NewRoomNameDirectory(facilitiesCompose.NewRoomRecords(rooms)),
		ActivityGroups: arrivalTimetable.NewTimetableGroupReads(repos.Timetable),
		WorkSchedules:  workTime,
		WorkModels:     workTime,
		Holidays:       schoolCalendarCompose.NewNonWorkingDays(calendar), NonWorkingDays: calendar,
		CategoryLinker: arrivalTimetable.ShiftTypeCategoryLinker(activities.SetCategoryShiftTypeLinks),
		DB:             db.Pool(), Broadcaster: hub, Logger: env.Logger.With("service", "shift_planning"),
		Today: env.Today,
	})
}

// PayrollStatus is the one payroll-status instance: the /payroll page and
// the DATEV writers must judge completeness identically.
func PayrollStatus(settings settingsCompose.BatchSettingsService, people PeopleRows) settingsCompose.PayrollStatusGetter {
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
