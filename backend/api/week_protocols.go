package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	projectionStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// mountWeekProtocolRoutes serves the weekly protocols of the groups (#4769)
// under /api/week-protocols: Timetable's protocols over its own tables, the
// caller's groups from Identity & Access, the Audit Platform's data access log
// and the People Directory names of the closing person.
func mountWeekProtocolRoutes(tenant chi.Router, protocols timetableModule.WeekProtocols, logger *slog.Logger, caller *owners.CallerRows, accessLog owners.DataAccessAppender, persons peopledirectory.Query, appEnv string, seed schedulerPorts.WeekProtocolEvenings) {
	var seedLifecycle func(context.Context) error
	var isLocalSeed func(*http.Request) bool
	if seed != nil {
		seedLifecycle = func(ctx context.Context) error { return seedWeekProtocols(ctx, time.Now(), seed) }
		isLocalSeed = func(r *http.Request) bool { return common.IsLocalSeedRequest(r, appEnv) }
	}
	resource := timetableAPI.NewWeekProtocolResource(timetableAPI.WeekProtocolResourceConfig{
		Service:     protocols,
		Caller:      caller,
		AccessLog:   weekProtocolAccessLog{Trail: accessLog},
		Renderer:    newDeclarationReports(),
		Names:       staffNoticeNames(persons),
		Logger:      logger.With("handler", "week_protocols"),
		Seed:        seedLifecycle,
		IsLocalSeed: isLocalSeed,
	})
	tenant.Mount("/week-protocols", resource.Router(ProtectedTenantRoutes))
}

// seedWeekProtocols first captures today's live day after the evening hour,
// then advances to its Sunday without another capture so the protocols are
// editable drafts. The seed client closes one of them through the ordinary
// endpoint, which stores the PDF through the normal lifecycle as well.
func seedWeekProtocols(ctx context.Context, now time.Time, seed schedulerPorts.WeekProtocolEvenings) error {
	day := calendar.DateFromTime(now)
	evening := day.BerlinMidnight().Add(21 * time.Hour)
	if err := seed(ctx, evening); err != nil {
		return err
	}
	sunday := day.StartOfISOWeek().AddDays(6).BerlinMidnight().Add(12 * time.Hour)
	return seed(ctx, sunday)
}

// WeekProtocolDayLog builds the weekly protocols' port onto the presence
// projection's day log (#4769) for the Worker: the projection's resource
// over the given sources, read without a request.
func WeekProtocolDayLog(
	logger *slog.Logger, persons peopledirectory.Capability, users peoplePorts.PersonDirectory,
	history studentpresence.StudentHistory, careDays carePlanModule.CareDayQuery,
	arrivals carePlanModule.ArrivalScheduleService, statusDays studentpresence.StatusDayReads,
) timetableModule.WeekProtocolAttendance {
	return weekProtocolAttendance{dayLog: projectionStudentsHTTP.NewResource(projectionStudentsHTTP.ResourceConfig{
		Logger:                  logger,
		PeopleDirectory:         persons,
		Persons:                 peopleCompose.NewStudentRoutePersons(users),
		StudentHistoryService:   history,
		CareDayService:          careDays,
		ArrivalScheduleService:  arrivals,
		StudentStatusDayService: statusDays,
	})}
}

// weekProtocolAttendance names each missing child by first and last name.
type weekProtocolAttendance struct {
	dayLog *projectionStudentsHTTP.Resource
}

func (a weekProtocolAttendance) GroupDayAttendance(ctx context.Context, day calendar.Date, groupIDs []int64) ([]timetableModule.WeekProtocolGroupDay, error) {
	logs, err := a.dayLog.GroupDayLogs(ctx, day, groupIDs)
	if err != nil {
		return nil, err
	}
	result := make([]timetableModule.WeekProtocolGroupDay, 0, len(logs))
	for _, log := range logs {
		entry := timetableModule.WeekProtocolGroupDay{
			GroupID: log.GroupID, StudentIDs: log.StudentIDs, Present: log.Present, Sick: log.Sick,
			Excused: log.Excused, ClassTrip: log.ClassTrip, Absent: log.Absent,
			Missing: make([]timetableModule.WeekProtocolMissingChild, 0, len(log.MissingChild)),
		}
		for _, child := range log.MissingChild {
			entry.Missing = append(entry.Missing, timetableModule.WeekProtocolMissingChild{
				StudentID: child.StudentID, Name: child.FirstName + " " + child.LastName, Reason: child.Status, Label: child.Label,
			})
		}
		result = append(result, entry)
	}
	return result, nil
}

// weekProtocolAccessTrail appends to the Audit Platform's data access log.
type weekProtocolAccessTrail interface {
	Create(context.Context, *auditModels.DataAccessLog) error
}

// weekProtocolAccessLog binds the weekly protocols' data access entries
// (#4769) to the Audit Platform: one entry per disclosure of a protocol or
// its PDF, over the week, with the group as scope.
type weekProtocolAccessLog struct{ Trail weekProtocolAccessTrail }

func (l weekProtocolAccessLog) RecordWeekProtocolAccess(ctx context.Context, access timetableAPI.WeekProtocolAccess) error {
	groupIDs := []int64{}
	if access.GroupID != nil {
		groupIDs = []int64{*access.GroupID}
	}
	return l.Trail.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: access.ActorAccountID,
		ActorRole:      access.ActorRole,
		ResourceType:   auditModels.ResourceTypeAttendanceWeekProtocol,
		RangeStart:     access.WeekStart.BerlinMidnight(),
		RangeEnd:       access.WeekStart.AddDays(6).EndOfDay(),
		AccessedAt:     access.At,
		Metadata:       map[string]interface{}{"protocol_id": access.ProtocolID, "group_ids": groupIDs},
	})
}
