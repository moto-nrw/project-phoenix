package students

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

const (
	DayPlanningStatusAll            = "all"
	DayPlanningStatusComesToday     = "comes_today"
	DayPlanningStatusNotComingToday = "not_coming_today"

	// Reason codes are owned by the schedule service (the day-planning
	// precedence lives there); these aliases keep the wire strings and the
	// export label switches in one place in this package.
	dayPlanningReasonSick             = scheduleService.DayPlanningReasonSick
	dayPlanningReasonExcused          = scheduleService.DayPlanningReasonExcused
	dayPlanningReasonClassTrip        = scheduleService.DayPlanningReasonClassTrip
	dayPlanningReasonArrivalException = scheduleService.DayPlanningReasonArrivalException
	dayPlanningReasonPickupException  = scheduleService.DayPlanningReasonPickupException
	dayPlanningReasonArrivalSchedule  = scheduleService.DayPlanningReasonArrivalSchedule
	dayPlanningReasonPickupSchedule   = scheduleService.DayPlanningReasonPickupSchedule
	dayPlanningReasonTimetable        = scheduleService.DayPlanningReasonTimetable
	dayPlanningReasonUnplanned        = scheduleService.DayPlanningReasonUnplanned
	dayPlanningReasonNoPlan           = scheduleService.DayPlanningReasonNoPlan
)

func (rs *Resource) enrichWithDayPlanning(ctx context.Context, responses []StudentResponse, now time.Time, attendances map[int64]*activeService.AttendanceStatus) error {
	fullAccessIDs := collectFullAccessStudentIDs(responses)
	if len(fullAccessIDs) == 0 {
		return nil
	}
	date := timezone.DateFromTime(now)

	arrivals, err := rs.loadDayPlanningArrivals(ctx, fullAccessIDs, date)
	if err != nil {
		return err
	}
	pickups, err := rs.loadDayPlanningPickups(ctx, fullAccessIDs, date)
	if err != nil {
		return err
	}
	timetableIDs, err := rs.loadDayPlanningTimetableIDs(ctx, fullAccessIDs, date)
	if err != nil {
		return err
	}
	pendingExcused, err := rs.loadPendingExcusedForDayPlanning(ctx, date)
	if err != nil {
		return err
	}

	applyDayPlanning(responses, arrivals, pickups, attendances, timetableIDs, pendingExcused)
	return nil
}

func (rs *Resource) loadDayPlanningArrivals(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*scheduleService.EffectiveArrivalTime, error) {
	if rs.ArrivalScheduleService == nil {
		return map[int64]*scheduleService.EffectiveArrivalTime{}, nil
	}
	return rs.ArrivalScheduleService.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, date)
}

func (rs *Resource) loadDayPlanningPickups(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*scheduleService.EffectivePickupTime, error) {
	if rs.PickupScheduleService == nil {
		return map[int64]*scheduleService.EffectivePickupTime{}, nil
	}
	return rs.PickupScheduleService.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, date)
}

func (rs *Resource) loadDayPlanningTimetableIDs(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]struct{}, error) {
	timetableIDs := map[int64]struct{}{}
	if rs.InstanceService == nil {
		return timetableIDs, nil
	}
	plannedIDs, err := rs.InstanceService.GetPlannedStudentIDsByDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	for _, id := range plannedIDs {
		timetableIDs[id] = struct{}{}
	}
	return timetableIDs, nil
}

// loadPendingExcusedForDayPlanning returns the pending parent excused-absence
// requests covering the date (#1845), shown as an informational badge with the
// parent's note without changing the planning status — the child stays
// "expected" until staff decide the request.
//
// The badge exposes the parent's note and belongs to the users:update-gated
// review queue (Änderungsanfragen). This enrichment also runs inside the
// users:read list/detail/export handlers, so a read-only group supervisor
// would otherwise see the note and approval marker without any right to the
// queue or to decide the request. Only enrich when the caller actually holds
// the review permission that gates the queue itself.
func (rs *Resource) loadPendingExcusedForDayPlanning(ctx context.Context, date timezone.Date) (map[int64]*activeModels.ExcusedAbsenceRequest, error) {
	if rs.ExcusedRequestService == nil ||
		!authorize.HasPermission(permissions.UsersUpdate, jwt.PermissionsFromCtx(ctx)) {
		return map[int64]*activeModels.ExcusedAbsenceRequest{}, nil
	}
	return rs.ExcusedRequestService.PendingByStudentForDate(ctx, date)
}

func applyDayPlanning(
	responses []StudentResponse,
	arrivals map[int64]*scheduleService.EffectiveArrivalTime,
	pickups map[int64]*scheduleService.EffectivePickupTime,
	attendances map[int64]*activeService.AttendanceStatus,
	timetableIDs map[int64]struct{},
	pendingExcused map[int64]*activeModels.ExcusedAbsenceRequest,
) {
	for i := range responses {
		if !responses[i].HasFullAccess {
			continue
		}
		id := responses[i].ID
		status, reason, label := resolveDayPlanning(responses[i], arrivals[id], pickups[id], attendances[id], timetableIDs)
		responses[i].DayPlanningStatus = status
		responses[i].DayPlanningReason = reason
		responses[i].DayPlanningLabel = label
		if req, ok := pendingExcused[id]; ok {
			note := req.Note
			responses[i].PendingExcusedNote = &note
		}
	}
}

func resolveDayPlanning(
	student StudentResponse,
	arrival *scheduleService.EffectiveArrivalTime,
	pickup *scheduleService.EffectivePickupTime,
	attendance *activeService.AttendanceStatus,
	timetableIDs map[int64]struct{},
) (string, string, string) {
	_, hasTimetable := timetableIDs[student.ID]
	decision := scheduleService.ResolveDayPlanning(scheduleService.DayPlanningInputs{
		HasActualAttendance: hasActualAttendanceToday(attendance),
		Sick:                student.Sick,
		ClassTrip:           student.ClassTrip,
		Excused:             student.Excused,
		Arrival:             arrival,
		Pickup:              pickup,
		HasTimetable:        hasTimetable,
	})

	status := DayPlanningStatusNotComingToday
	if decision.ComesToday {
		status = DayPlanningStatusComesToday
	}
	return status, decision.Reason, dayPlanningLabel(decision)
}

// dayPlanningLabel renders the German UI label for a resolved day-planning
// decision. The exception reasons carry two labels each: a planned-time variant
// (ComesToday) and a no-time absence variant that surfaces the exception note.
func dayPlanningLabel(decision scheduleService.DayPlanningDecision) string {
	switch decision.Reason {
	case dayPlanningReasonUnplanned:
		return "ungeplant anwesend"
	case dayPlanningReasonSick:
		return "krank gemeldet"
	case dayPlanningReasonClassTrip:
		return "Klassenfahrt"
	case dayPlanningReasonExcused:
		return "entschuldigt"
	case dayPlanningReasonArrivalException:
		if decision.ComesToday {
			return "geplante Ankunft heute"
		}
		return dayPlanningExceptionLabel(decision.ExceptionNotes)
	case dayPlanningReasonArrivalSchedule:
		return "Ankunftsplan heute"
	case dayPlanningReasonPickupException:
		if decision.ComesToday {
			return "geplante Abholung heute"
		}
		return dayPlanningExceptionLabel(decision.ExceptionNotes)
	case dayPlanningReasonPickupSchedule:
		return "Abholplan heute"
	case dayPlanningReasonTimetable:
		return "Betreuungsplan heute"
	default:
		return "kein Plan für heute"
	}
}

func hasActualAttendanceToday(attendance *activeService.AttendanceStatus) bool {
	return attendance != nil && attendance.CheckInTime != nil && attendance.Status != "not_checked_in"
}

func attendanceMapFromSnapshot(snapshot *common.StudentDataSnapshot) map[int64]*activeService.AttendanceStatus {
	if snapshot == nil || snapshot.LocationSnapshot == nil || snapshot.LocationSnapshot.Attendances == nil {
		return map[int64]*activeService.AttendanceStatus{}
	}
	return snapshot.LocationSnapshot.Attendances
}

func dayPlanningExceptionLabel(notes string) string {
	if notes != "" {
		return notes
	}
	return "Tagesausnahme"
}

func applyDayPlanningFilter(responses []StudentResponse, dayStatus string) []StudentResponse {
	if dayStatus == "" || dayStatus == DayPlanningStatusAll {
		return responses
	}
	filtered := make([]StudentResponse, 0, len(responses))
	for _, response := range responses {
		if response.DayPlanningStatus == dayStatus {
			filtered = append(filtered, response)
		}
	}
	return filtered
}
