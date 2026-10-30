package owners

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	settingsRecords "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// WeekProtocolDayLogBuilder builds Timetable's port onto the presence
// projection's day log (#4769) over the owners the Worker root built: the
// children and their persons, the attendance, today's care days and
// arrivals, and the reported status days. The API root supplies it: the
// projection's day log lives in its HTTP package, which only the API root
// composes.
type WeekProtocolDayLogBuilder func(
	logger *slog.Logger, persons peopledirectory.Capability, users peoplePorts.PersonDirectory,
	history studentpresence.StudentHistory, careDays carePlanModule.CareDayQuery,
	arrivals carePlanModule.ArrivalScheduleService, statusDays studentpresence.StatusDayReads,
) timetable.WeekProtocolAttendance

// weekProtocolGroupLimit bounds the OGS groups of one school the evening run
// keeps protocols for; School Structure refuses larger listings.
const weekProtocolGroupLimit = 1000

// newWeekProtocolEvenings builds the Worker's evening run of the weekly
// protocols (#4769) over the owners the Worker root built: Timetable's own
// blocks and staff notices, the School Calendar, School Structure's groups,
// the presence projection's day log and the group mode setting.
func newWeekProtocolEvenings(
	db transactionruntime.Database, logger *slog.Logger, settings settingsCompose.BatchSettingsService,
	persons peopledirectory.Capability, services PeopleServices, reports PresenceReports,
	schedules CarePlanSchedules, carePlan carePlanModule.Owner, timetableRepos timetableCompose.Repositories,
	dayLog WeekProtocolDayLogBuilder,
) schedulerPorts.WeekProtocolEvenings {
	// Like the planned staff exits, a composition failure is a startup
	// failure of the Worker: the job panics while the Worker is wired.
	if dayLog == nil {
		panic("compose week protocols: the day log builder is required")
	}
	calendar, err := BuildSchoolCalendarRecords(db, ObserveSchoolCalendar, settings)
	if err != nil {
		panic(fmt.Errorf("compose week protocol calendar: %w", err))
	}
	groups, err := BuildServeSchoolStructure(db)
	if err != nil {
		panic(fmt.Errorf("compose week protocol groups: %w", err))
	}
	notices := timetableCompose.NewStaffNotices(timetableCompose.StaffNoticeDependencies{
		DB: db.Pool(), Logger: logger.With("service", "staffnotice"),
		Periods: timetableCompose.NewCalendarPeriodRepository(calendar,
			timetableCompose.NewPlanningCalendarPeriodUsage(enrollmentCompose.New(), timetableRepos.Timetable)),
	})
	attendance := dayLog(logger.With("service", "week_protocol_day_log"), persons, services.Users,
		reports.StudentHistory, schedules.CareDay, schedules.Arrivals, NewStudentStatusDayRepository(carePlan))
	sources := timetable.WeekProtocolSources{
		Notices: notices, Calendar: calendar, Attendance: attendance,
		Groups: weekProtocolGroupsOf(groups), SchoolMode: weekProtocolSchoolModeOf(settings),
	}
	owner := timetableRepos.Timetable
	jobLogger := logger.With("service", "week_protocols")
	return func(ctx context.Context, now time.Time) error {
		result, err := owner.RunWeekProtocolEvening(ctx, now, sources)
		if err != nil {
			return err
		}
		if result.DaysCaptured > 0 || result.Drafted > 0 || result.AutoClosed > 0 {
			jobLogger.Info("week protocols advanced",
				slog.Int("days_captured", result.DaysCaptured),
				slog.Int("drafted", result.Drafted),
				slog.Int("auto_closed", result.AutoClosed),
			)
		}
		return nil
	}
}

// weekProtocolGroupsOf binds School Structure's OGS groups.
func weekProtocolGroupsOf(groups schoolstructure.GroupListing) func(context.Context) ([]timetable.WeekProtocolGroup, error) {
	return func(ctx context.Context) ([]timetable.WeekProtocolGroup, error) {
		listed, err := groups.ListGroups(ctx, weekProtocolGroupLimit)
		if err != nil {
			return nil, err
		}
		result := make([]timetable.WeekProtocolGroup, 0, len(listed))
		for _, group := range listed {
			result = append(result, timetable.WeekProtocolGroup{ID: group.ID, Name: group.Name})
		}
		return result, nil
	}
}

// weekProtocolSchoolModeOf reads operations.group_mode: a school without
// fixed groups keeps one protocol for the whole school.
func weekProtocolSchoolModeOf(settings settingsCompose.BatchSettingsService) func(context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		mode, err := settings.ResolveString(ctx, settingsRecords.KeyGroupMode)
		if err != nil {
			return false, err
		}
		return settingsRecords.GroupModeOf(mode) == settingsRecords.GroupModeOpenCare, nil
	}
}
