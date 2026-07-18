package students

import (
	"context"
	"fmt"
	"strings"
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
	DayPlanningStatusAll              = "all"
	DayPlanningStatusComesToday       = "comes_today"
	DayPlanningStatusNotComingToday   = "not_coming_today"
	dayPlanningReasonSick             = "sick"
	dayPlanningReasonExcused          = "excused"
	dayPlanningReasonClassTrip        = "class_trip"
	dayPlanningReasonArrivalException = "arrival_exception"
	dayPlanningReasonPickupException  = "pickup_exception"
	dayPlanningReasonArrivalSchedule  = "arrival_schedule"
	dayPlanningReasonPickupSchedule   = "pickup_schedule"
	dayPlanningReasonTimetable        = "timetable"
	dayPlanningReasonUnplanned        = "unplanned_attendance"
	dayPlanningReasonNoPlan           = "no_plan"
)

// resolvePlanningDate turns the optional `date` query/filter value into the
// calendar day the day-planning pipeline is evaluated for. Empty means the
// school-local today (derived from now, i.e. Berlin wall clock, so the day
// switches at local midnight, not at UTC midnight).
func resolvePlanningDate(raw string, now time.Time) (timezone.Date, bool, error) {
	today := timezone.DateFromTime(now)
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return today, true, nil
	}
	date, err := timezone.ParseDate(trimmed)
	if err != nil {
		return timezone.Date{}, false, fmt.Errorf("invalid date %q, expected YYYY-MM-DD", raw)
	}
	return date, date == today, nil
}

func (rs *Resource) enrichWithDayPlanning(ctx context.Context, responses []StudentResponse, planningDate timezone.Date, isToday bool, attendances map[int64]*activeService.AttendanceStatus) error {
	fullAccessIDs := collectFullAccessStudentIDs(responses)
	if len(fullAccessIDs) == 0 {
		return nil
	}

	arrivals := map[int64]*scheduleService.EffectiveArrivalTime{}
	if rs.ArrivalScheduleService != nil {
		var err error
		arrivals, err = rs.ArrivalScheduleService.GetBulkEffectiveArrivalTimesForDate(ctx, fullAccessIDs, planningDate)
		if err != nil {
			return err
		}
	}

	pickups := map[int64]*scheduleService.EffectivePickupTime{}
	if rs.PickupScheduleService != nil {
		var err error
		pickups, err = rs.PickupScheduleService.GetBulkEffectivePickupTimesForDate(ctx, fullAccessIDs, planningDate)
		if err != nil {
			return err
		}
	}

	timetableIDs := map[int64]struct{}{}
	if rs.InstanceService != nil {
		plannedIDs, err := rs.InstanceService.GetPlannedStudentIDsByDate(ctx, fullAccessIDs, planningDate)
		if err != nil {
			return err
		}
		for _, id := range plannedIDs {
			timetableIDs[id] = struct{}{}
		}
	}

	// Pending parent excused-absence requests covering today (#1845): shown as an
	// informational badge with the parent's note, without changing the planning
	// status — the child stays "expected" until staff decide the request.
	//
	// The badge exposes the parent's note and belongs to the users:update-gated
	// review queue (Änderungsanfragen). This enrichment also runs inside the
	// users:read list/detail/export handlers, so a read-only group supervisor
	// would otherwise see the note and approval marker without any right to the
	// queue or to decide the request. Only enrich when the caller actually holds
	// the review permission that gates the queue itself.
	pendingExcused := map[int64]*activeModels.ExcusedAbsenceRequest{}
	if rs.ExcusedRequestService != nil &&
		authorize.HasPermission(permissions.UsersUpdate, jwt.PermissionsFromCtx(ctx)) {
		var err error
		pendingExcused, err = rs.ExcusedRequestService.PendingByStudentForDate(ctx, planningDate)
		if err != nil {
			return err
		}
	}

	for i := range responses {
		if !responses[i].HasFullAccess {
			continue
		}
		status, reason, label := resolveDayPlanningForDate(responses[i], arrivals[responses[i].ID], pickups[responses[i].ID], attendances[responses[i].ID], timetableIDs, isToday)
		responses[i].DayPlanningStatus = status
		responses[i].DayPlanningReason = reason
		responses[i].DayPlanningLabel = label
		if req, ok := pendingExcused[responses[i].ID]; ok {
			note := req.Note
			responses[i].PendingExcusedNote = &note
		}
	}

	return nil
}

// resolveDayPlanning keeps the original today-bound shape; it exists so the
// single computation in resolveDayPlanningForDate stays the only logic while
// today-callers (and existing tests) keep their signature.
func resolveDayPlanning(
	student StudentResponse,
	arrival *scheduleService.EffectiveArrivalTime,
	pickup *scheduleService.EffectivePickupTime,
	attendance *activeService.AttendanceStatus,
	timetableIDs map[int64]struct{},
) (string, string, string) {
	return resolveDayPlanningForDate(student, arrival, pickup, attendance, timetableIDs, true)
}

// resolveDayPlanningForDate applies the #1448 precedence to one calendar day:
// explicit absence wins, then a day-specific no-time exception that clears the
// child's arrival/pickup (also an absence signal), then any positive planning
// signal for the day, otherwise the child is not expected. Absence must be
// evaluated before presence: a no-time exception means "not here this day" and
// has to win even when the child also carries an unrelated presence signal
// (a regular pickup time, a timetable placement) for the same date (#1939).
// For non-today dates the actual-attendance shortcut is skipped — a child being
// present right now says nothing about another day (#1939) — and labels avoid
// "heute" wording.
func resolveDayPlanningForDate(
	student StudentResponse,
	arrival *scheduleService.EffectiveArrivalTime,
	pickup *scheduleService.EffectivePickupTime,
	attendance *activeService.AttendanceStatus,
	timetableIDs map[int64]struct{},
	isToday bool,
) (string, string, string) {
	if isToday && hasActualAttendanceToday(attendance) {
		return DayPlanningStatusComesToday, dayPlanningReasonUnplanned, "ungeplant anwesend"
	}
	if status, reason, label, ok := scheduledAbsencePlanning(student); ok {
		return status, reason, label
	}
	if status, reason, label, ok := plannedAbsencePlanning(arrival, pickup); ok {
		return status, reason, label
	}
	if status, reason, label, ok := plannedPresencePlanning(student, arrival, pickup, timetableIDs, isToday); ok {
		return status, reason, label
	}
	return DayPlanningStatusNotComingToday, dayPlanningReasonNoPlan, dayLabel("kein Plan für heute", "kein Plan für diesen Tag", isToday)
}

// scheduledAbsencePlanning covers the explicit-absence statuses recorded for the
// day (sick / class trip / excused). They win over any presence signal.
func scheduledAbsencePlanning(student StudentResponse) (string, string, string, bool) {
	switch {
	case student.Sick:
		return DayPlanningStatusNotComingToday, dayPlanningReasonSick, "krank gemeldet", true
	case student.ClassTrip:
		return DayPlanningStatusNotComingToday, dayPlanningReasonClassTrip, "Klassenfahrt", true
	case student.Excused:
		return DayPlanningStatusNotComingToday, dayPlanningReasonExcused, "entschuldigt", true
	}
	return "", "", "", false
}

// plannedPresencePlanning reports a "comes today" status when a positive plan
// exists for the day: a scheduled/exception arrival time, a scheduled/exception
// pickup time, or a timetable placement.
func plannedPresencePlanning(
	student StudentResponse,
	arrival *scheduleService.EffectiveArrivalTime,
	pickup *scheduleService.EffectivePickupTime,
	timetableIDs map[int64]struct{},
	isToday bool,
) (string, string, string, bool) {
	if arrival != nil && arrival.ArrivalTime != nil {
		if arrival.IsException {
			return DayPlanningStatusComesToday, dayPlanningReasonArrivalException, dayLabel("geplante Ankunft heute", "geplante Ankunft", isToday), true
		}
		return DayPlanningStatusComesToday, dayPlanningReasonArrivalSchedule, dayLabel("Ankunftsplan heute", "Ankunftsplan", isToday), true
	}
	if pickup != nil && pickup.PickupTime != nil {
		if pickup.IsException {
			return DayPlanningStatusComesToday, dayPlanningReasonPickupException, dayLabel("geplante Abholung heute", "geplante Abholung", isToday), true
		}
		return DayPlanningStatusComesToday, dayPlanningReasonPickupSchedule, dayLabel("Abholplan heute", "Abholplan", isToday), true
	}
	if _, ok := timetableIDs[student.ID]; ok {
		return DayPlanningStatusComesToday, dayPlanningReasonTimetable, dayLabel("Betreuungsplan heute", "Betreuungsplan", isToday), true
	}
	return "", "", "", false
}

// plannedAbsencePlanning reports a "not coming" status for a day-specific
// exception that explicitly clears the child's arrival or pickup — a "not here
// today" exception carrying no time.
func plannedAbsencePlanning(
	arrival *scheduleService.EffectiveArrivalTime,
	pickup *scheduleService.EffectivePickupTime,
) (string, string, string, bool) {
	if arrival != nil && arrival.IsException && arrival.ArrivalTime == nil {
		return DayPlanningStatusNotComingToday, dayPlanningReasonArrivalException, dayPlanningExceptionLabel(arrival.Notes), true
	}
	if pickup != nil && pickup.IsException && pickup.PickupTime == nil {
		return DayPlanningStatusNotComingToday, dayPlanningReasonPickupException, dayPlanningExceptionLabel(pickup.Notes), true
	}
	return "", "", "", false
}

func dayLabel(todayLabel, otherDayLabel string, isToday bool) string {
	if isToday {
		return todayLabel
	}
	return otherDayLabel
}

// resetScheduledStatusFlags clears the Sick/ClassTrip/Excused flags that
// buildStudentResponses seeds from the student row. Those columns mirror
// TODAY's state; when the list is evaluated for another date they must not
// leak into it — applyStatusDaysForDate then overlays the rows recorded for
// the requested date.
func resetScheduledStatusFlags(responses []StudentResponse) {
	for i := range responses {
		responses[i].Sick = false
		responses[i].SickSince = nil
		responses[i].ClassTrip = false
		responses[i].ClassTripSince = nil
		responses[i].Excused = false
		responses[i].ExcusedSince = nil
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
