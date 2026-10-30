package api

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	projectionStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"
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
func mountWeekProtocolRoutes(tenant chi.Router, protocols timetableModule.WeekProtocols, logger *slog.Logger, caller *owners.CallerRows, accessLog owners.DataAccessAppender, persons peopledirectory.Query) {
	resource := timetableAPI.NewWeekProtocolResource(timetableAPI.WeekProtocolResourceConfig{
		Service:   protocols,
		Caller:    caller,
		AccessLog: weekProtocolAccessLog{Trail: accessLog},
		Renderer:  newDeclarationReports(),
		Names:     staffNoticeNames(persons),
		Logger:    logger.With("handler", "week_protocols"),
	})
	tenant.Mount("/week-protocols", resource.Router(ProtectedTenantRoutes))
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
// (#4769) to the Audit Platform: one entry per opening of a protocol or its
// PDF, over the week, with the group as scope.
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
