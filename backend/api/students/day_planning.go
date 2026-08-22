package students

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
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

// maxPlanningDate returns the last supported planning day: the Sunday closing
// the CURRENT calendar week. The timetable signal (GetPlannedStudentIDsByDate)
// reads only concrete, pre-materialized schedule.instance_students rows; past
// the materialized window a timetable-only child would silently classify as
// "Kommt nicht" even though the recurring template says otherwise. Capping the
// selectable range keeps day planning on the same materialized source of truth
// as every other timetable view instead of tracking materialization state or
// projecting recurrence a second time (#1939).
//
// Why the CURRENT week and not the next one: the scheduler materializes one
// Monday–Sunday block per run, and materializationService.ResolveWindow always
// starts at the Monday STRICTLY AFTER the run day — the current partial week is
// never (re-)materialized. So a run on weekday W of week N covers week N+1 only.
// With the default cadence (timetable.materialization_weekday = 5 / Friday,
// timetable.materialization_weeks_ahead = 1) the sole coverage guarantee that
// holds at every moment is therefore the week containing today: on Monday
// through Thursday the following week has not been materialized yet — Friday's
// run creates it. Both settings are tenant-overridable (any weekday, weeks-ahead
// clamped to >= 1), and the current week is the largest horizon that stays
// correct for EVERY combination, which is what lets the frontend mirror this as
// a pure function of today instead of querying materialization state.
//
// Consequence: on a Friday the picker stops at that Sunday even though the
// day's run has usually already materialized the next week. Widening this needs
// a real guarantee, not a wider constant — raise the weeks-ahead default to 2
// AND derive the horizon from the resolved per-tenant settings (exposed to the
// frontend), rather than assuming coverage the scheduler does not promise.
func maxPlanningDate(today timezone.Date) timezone.Date {
	daysUntilSunday := (7 - int(today.Weekday())) % 7
	return today.AddDays(daysUntilSunday)
}

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
	// Planning is future-only. The pipeline projects the current recurring
	// schedule definitions onto the requested day; for a past day that is a
	// misleading historical reconstruction (the definitions describe the plan
	// going forward, not what was actually planned back then), which is why the
	// page treats past dates as unsupported. Reject them at the boundary so a
	// direct or stale client cannot bypass the UI and obtain such a plan (#1939).
	if date.Before(today) {
		return timezone.Date{}, false, fmt.Errorf("date %q is in the past, planning is only available for today and future dates", raw)
	}
	// Mirror of the past guard at the far end: reject days beyond the
	// materialization-backed horizon so a direct or stale client cannot obtain
	// a plan the timetable signal cannot answer for (see maxPlanningDate).
	if maxDate := maxPlanningDate(today); date.After(maxDate) {
		return timezone.Date{}, false, fmt.Errorf("date %q is beyond the supported planning horizon, planning is available through %s", raw, maxDate.String())
	}
	return date, date == today, nil
}

// activeLiveListFilters names the list filters that can only be answered from
// TODAY's live presence state: room_id and location_state resolve through
// active.visits, and location matches the current-location snapshot. "" (and
// "Unknown" for location, its all-values sentinel) mean the filter is off.
func activeLiveListFilters(params *studentListParams) []string {
	var active []string
	if params.roomID > 0 {
		active = append(active, "room_id")
	}
	if params.locationState != "" {
		active = append(active, "location_state")
	}
	if params.location != "" && params.location != "Unknown" {
		active = append(active, "location")
	}
	return active
}

// locationDerivedExportStatuses are the export status buckets exportStatus
// reads off the live location snapshot. A dated export strips that snapshot, so
// filtering on one of them would classify every row "abwesend" — they are
// answerable for today only.
//
// The remaining buckets (krank, klassenfahrt, entschuldigt) are NOT live: they
// come from the Sick/ClassTrip/Excused flags, which applyStatusDaysForDate
// overlays from the status days recorded for the requested date. Planning an
// absence for a future day is exactly what those filters are for, so a dated
// export must keep accepting them (#1939).
var locationDerivedExportStatuses = map[string]struct{}{
	"abwesend":  {},
	"unterwegs": {},
	"schulhof":  {},
	"anwesend":  {},
}

// activeLiveExportFilters is the export counterpart of activeLiveListFilters.
func activeLiveExportFilters(filters studentExportFilters) []string {
	var active []string
	if strings.TrimSpace(filters.RoomID) != "" {
		active = append(active, "room_id")
	}
	if _, isLive := locationDerivedExportStatuses[filters.Status]; isLive {
		active = append(active, "status")
	}
	return active
}

// liveFilterError rejects a non-today planning request that carries a live
// presence filter. Silently ignoring one would be worse than a 400: the caller
// would receive a plausible-looking list that answers a different question than
// the one asked. The page already neutralizes these filters for a non-today
// date, so this only ever fires for a direct or stale client — the same
// boundary reasoning as the past-date and horizon guards above (#1939).
func liveFilterError(active []string, date timezone.Date, isToday bool) error {
	if isToday || len(active) == 0 {
		return nil
	}
	return fmt.Errorf(
		"filter %s reflects the current presence state and is only available for today, not for the planning date %s",
		strings.Join(active, ", "), date.String(),
	)
}

// dayPlanningTimes carries the bulk-loaded effective-time maps out of
// enrichWithDayPlanning so later pipeline stages reuse them instead of
// re-running the same three SELECTs per kind (#2098). Maps are keyed by
// student ID and cover every full-access student of the pre-pagination
// response set; both may be nil when no full-access student exists.
type dayPlanningTimes struct {
	arrivals map[int64]*scheduleService.EffectiveArrivalTime
	pickups  map[int64]*scheduleService.EffectivePickupTime
}

func (rs *Resource) enrichWithDayPlanning(ctx context.Context, responses []StudentResponse, planningDate timezone.Date, isToday bool, attendances map[int64]*activeService.AttendanceStatus) (dayPlanningTimes, error) {
	// The parent's pending note runs FIRST and outside the full-access branch
	// below: it is the review queue's signal, not part of the day plan, and its
	// audience is whoever may decide the request. In a school without fixed
	// groups that person supervises no group, so every child reaches them as a
	// restricted entry — tying the note to the read scope would hide from the
	// deciding staffer exactly the request they own (#2232).
	if err := rs.applyPendingExcusedNotes(ctx, responses, planningDate); err != nil {
		return dayPlanningTimes{}, err
	}

	fullAccessIDs := collectFullAccessStudentIDs(responses)
	if len(fullAccessIDs) == 0 {
		return dayPlanningTimes{}, nil
	}

	arrivals, err := rs.loadDayPlanningArrivals(ctx, fullAccessIDs, planningDate)
	if err != nil {
		return dayPlanningTimes{}, err
	}
	pickups, err := rs.loadDayPlanningPickups(ctx, fullAccessIDs, planningDate)
	if err != nil {
		return dayPlanningTimes{}, err
	}
	timetableIDs, err := rs.loadDayPlanningTimetableIDs(ctx, fullAccessIDs, planningDate)
	if err != nil {
		return dayPlanningTimes{}, err
	}

	applyDayPlanning(responses, arrivals, pickups, attendances, timetableIDs, isToday)
	return dayPlanningTimes{arrivals: arrivals, pickups: pickups}, nil
}

// applyPendingExcusedNotes attaches the guardian's note of a pending
// excused-absence request to the children it was submitted for.
//
// Per-child scope comes from the loader: PendingByStudentForDate returns only
// the children the caller may DECIDE the request for (the queue's own
// AbsenceWritableStudentFilter), so a group supervisor still sees nothing
// beyond their own children and no second filter is needed here.
func (rs *Resource) applyPendingExcusedNotes(ctx context.Context, responses []StudentResponse, date timezone.Date) error {
	if len(responses) == 0 {
		return nil
	}
	pendingExcused, err := rs.loadPendingExcusedForDayPlanning(ctx, date)
	if err != nil {
		return err
	}
	if len(pendingExcused) == 0 {
		return nil
	}
	for i := range responses {
		req, ok := pendingExcused[responses[i].ID]
		if !ok {
			continue
		}
		note := req.Note
		responses[i].PendingExcusedNote = &note
	}
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
	return rs.filterTimetableIDsByCareDay(ctx, timetableIDs, date)
}

// filterTimetableIDsByCareDay drops the children a timetable block lists but
// the care plan does not place in the OGS that weekday (#1747).
//
// Assigning a whole group or year to an activity (#1838) puts every member on
// every occurrence, so the raw assignment alone would report a child as
// "kommt heute — Betreuungsplan" on days they are not booked for. The timetable
// roster and the expected counts already resolve this through CareDayService;
// filtering the same signal here is what keeps the student search from
// contradicting them about the same child on the same day.
//
// Both non-expected verdicts remove the signal — "not booked that weekday" and
// "cancelled" — exactly as the timetable roster and the expected counts treat
// them. Restricting this to not_scheduled leaves a weekend hole: the bulk
// arrival/pickup loaders below return early on Saturdays and Sundays without
// looking at exceptions, so a timeless "kommt heute nicht" exception on a
// weekend block reaches ResolveDayPlanning as nothing but HasTimetable and the
// search reports "kommt heute" for a child the roster calls abgemeldet.
//
// A child with no plan on file at all resolves to unknown and keeps the
// signal — schools that do not maintain arrival/pickup plans must keep seeing
// their full roster.
func (rs *Resource) filterTimetableIDsByCareDay(
	ctx context.Context, timetableIDs map[int64]struct{}, date timezone.Date,
) (map[int64]struct{}, error) {
	if rs.CareDayService == nil || len(timetableIDs) == 0 {
		return timetableIDs, nil
	}

	studentIDs := make([]int64, 0, len(timetableIDs))
	for id := range timetableIDs {
		studentIDs = append(studentIDs, id)
	}

	careDays, err := rs.CareDayService.ResolveForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}

	for id := range timetableIDs {
		// A missing entry is the empty status, which Expected() reports true —
		// an unresolvable child keeps the signal rather than disappearing.
		if !careDays[id].Expected() {
			delete(timetableIDs, id)
		}
	}
	return timetableIDs, nil
}

// loadPendingExcusedForDayPlanning returns the pending parent excused-absence
// requests covering the date (#1845), shown as an informational badge with the
// parent's note without changing the planning status — the child stays
// "expected" until staff decide the request.
//
// The badge exposes the parent's note and belongs to the review queue
// (Änderungsanfragen). This enrichment also runs inside the users:read
// list/detail/export handlers, so a read-only group supervisor would otherwise
// see the note and approval marker without any right to the queue or to decide
// the request. Only enrich when the caller actually holds a permission that
// gates the queue itself — users:update, or users:absence for the staff who
// decide exactly these requests in a school without fixed groups (#2232).
func (rs *Resource) loadPendingExcusedForDayPlanning(ctx context.Context, date timezone.Date) (map[int64]*activeModels.ExcusedAbsenceRequest, error) {
	if rs.ExcusedRequestService == nil ||
		!authorize.CanReviewExcusedAbsenceRequests(jwt.PermissionsFromCtx(ctx)) {
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
	isToday bool,
) {
	for i := range responses {
		if !responses[i].HasFullAccess {
			continue
		}
		id := responses[i].ID
		status, reason, label := resolveDayPlanningForDate(responses[i], arrivals[id], pickups[id], attendances[id], timetableIDs, isToday)
		responses[i].DayPlanningStatus = status
		responses[i].DayPlanningReason = reason
		responses[i].DayPlanningLabel = label
	}
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

// resolveDayPlanningForDate applies the #1448 precedence (owned by
// scheduleService.ResolveDayPlanning) to one calendar day and renders the German
// label for the decision. For non-today dates the actual-attendance shortcut is
// skipped — a child being present right now says nothing about another day
// (#1939) — and labels avoid "heute" wording.
func resolveDayPlanningForDate(
	student StudentResponse,
	arrival *scheduleService.EffectiveArrivalTime,
	pickup *scheduleService.EffectivePickupTime,
	attendance *activeService.AttendanceStatus,
	timetableIDs map[int64]struct{},
	isToday bool,
) (string, string, string) {
	_, hasTimetable := timetableIDs[student.ID]
	decision := scheduleService.ResolveDayPlanning(scheduleService.DayPlanningInputs{
		HasActualAttendance: isToday && hasActualAttendanceToday(attendance),
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
	return status, decision.Reason, dayPlanningLabel(decision, isToday)
}

// dayPlanningLabel renders the German UI label for a resolved day-planning
// decision. The exception reasons carry two labels each: a planned-time variant
// (ComesToday) and a no-time absence variant that surfaces the exception note.
func dayPlanningLabel(decision scheduleService.DayPlanningDecision, isToday bool) string {
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
			return dayLabel("geplante Ankunft heute", "geplante Ankunft", isToday)
		}
		return dayPlanningExceptionLabel(decision.ExceptionNotes)
	case dayPlanningReasonArrivalSchedule:
		return dayLabel("Ankunftsplan heute", "Ankunftsplan", isToday)
	case dayPlanningReasonPickupException:
		if decision.ComesToday {
			return dayLabel("geplante Abholung heute", "geplante Abholung", isToday)
		}
		return dayPlanningExceptionLabel(decision.ExceptionNotes)
	case dayPlanningReasonPickupSchedule:
		return dayLabel("Abholplan heute", "Abholplan", isToday)
	case dayPlanningReasonTimetable:
		return dayLabel("Betreuungsplan heute", "Betreuungsplan", isToday)
	default:
		return dayLabel("kein Plan für heute", "kein Plan für diesen Tag", isToday)
	}
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

// resetLiveLocationFields clears the live-location snapshot (current location,
// the since-timestamp, and the room colour) that buildStudentResponses seeds
// from today's active.visits/attendance state. Those fields describe where the
// child is RIGHT NOW; a dated (non-today) export must not carry them, or a
// document labelled for another day would leak today's whereabouts. This also
// keeps exportStatus (which derives its snapshot bucket from Location) from
// classifying a future day by the current location (#1939).
func resetLiveLocationFields(responses []StudentResponse) {
	for i := range responses {
		responses[i].Location = ""
		responses[i].LocationSince = nil
		responses[i].RoomColor = nil
	}
}

func hasActualAttendanceToday(attendance *activeService.AttendanceStatus) bool {
	return attendance.IsCurrentlyPresent()
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
