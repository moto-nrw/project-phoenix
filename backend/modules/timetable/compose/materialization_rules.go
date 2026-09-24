package compose

import (
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The per-date rules of the recurrence engine, shared by materialization and
// the lost-edit detection so the two can never drift apart. Pure functions,
// unit-tested without a database.

// materialParams groups the three fields a schedule/exception can vary:
// start/end times and room. Title comes from the template unchanged.
type materialParams struct {
	StartTime time.Time
	EndTime   time.Time
	RoomID    int64
}

// existingKey identifies a row in the (template, date, start_time) search
// space. The date is a timezone.Date (comparable, no instant); the start time
// stays a formatted "15:04:05" string because bun reads PostgreSQL TIME
// columns back with arbitrary (driver-chosen) time zones on the Go side —
// string formatting makes the key timezone-independent by construction.
type existingKey struct {
	ActivityGroupID int64
	Date            timezone.Date
	StartTime       string // "15:04:05"
}

type exceptionKey struct {
	ActivityGroupID int64
	Date            timezone.Date
}

// candidateSkip names why a (template, schedule, date) candidate produces no
// occurrence; each value is one bucket of timetable.MaterializationResult.
type candidateSkip uint8

const (
	candidateKept candidateSkip = iota
	candidateEnded
	candidateNotStarted
	candidateNoPeriod
	candidateABWeek
	candidateHoliday
	candidateClosingDay
	candidateIncomplete
	candidateCancelled
)

// candidateSlot applies the engine's per-date rules to one schedule row of
// the template on a weekday-matching date: schedule validity and the series'
// last day, period selection, the A/B week pattern, holidays and closing days
// (#3594), the timeframe, the dated exception and the room. It returns the
// effective slot and period of a kept candidate. The zero NonWorkingDays
// skips no holiday or closing day.
//
// Both schedule.timeframes and schedule.activity_instances store clock values
// as SQL TIME. The slot passes through the wall-clock normalization anyway so
// driver-specific date anchors never affect comparisons or writes.
func candidateSlot(
	tmpl *activities.Group,
	sch *activities.Schedule,
	date timezone.Date,
	periods []*schedule.CalendarPeriod,
	days timetable.NonWorkingDays,
	exc *schedule.ActivityException,
	timeframeByID map[int64]*schedule.Timeframe,
	logger *slog.Logger,
) (materialParams, *schedule.CalendarPeriod, candidateSkip) {
	if scheduleEndedOn(sch, date) || (tmpl.SeriesLastDay != nil && tmpl.SeriesLastDay.Before(date)) {
		return materialParams{}, nil, candidateEnded
	}
	if scheduleNotStartedOn(sch, date) {
		return materialParams{}, nil, candidateNotStarted
	}
	period := selectPeriod(tmpl, sch, date, periods, logger)
	if period == nil {
		return materialParams{}, nil, candidateNoPeriod
	}
	if !weekPatternApplies(sch.WeekPattern, date, period) {
		return materialParams{}, nil, candidateABWeek
	}
	switch days.Skip(date.String(), tmpl.IncludeClosingDays) {
	case timetable.SkipHoliday:
		return materialParams{}, nil, candidateHoliday
	case timetable.SkipClosingDay:
		return materialParams{}, nil, candidateClosingDay
	}
	tfID := int64(0)
	if sch.TimeframeID != nil {
		tfID = *sch.TimeframeID
	}
	tf, ok := timeframeByID[tfID]
	if !ok || tf.EndTime == nil {
		return materialParams{}, nil, candidateIncomplete
	}
	base := materialParams{
		StartTime: extractTimeOfDay(tf.StartTime),
		EndTime:   extractTimeOfDay(*tf.EndTime),
	}
	if tmpl.PlannedRoomID != nil {
		base.RoomID = *tmpl.PlannedRoomID
	}
	effective, skip := applyException(base, exc)
	if skip {
		return materialParams{}, nil, candidateCancelled
	}
	if effective.RoomID <= 0 {
		// No primary room on the template and no override from an exception —
		// the NOT NULL on room_id cannot be satisfied.
		return materialParams{}, nil, candidateIncomplete
	}
	return effective, period, candidateKept
}

// countSkippedCandidate files a skipped candidate under its bucket.
func countSkippedCandidate(result *timetable.MaterializationResult, skip candidateSkip) {
	switch skip {
	case candidateEnded:
		result.CandidatesSkippedEnded++
	case candidateNotStarted:
		result.CandidatesSkippedNotStarted++
	case candidateNoPeriod:
		result.CandidatesSkippedNoPeriod++
	case candidateABWeek:
		result.CandidatesSkippedABWeek++
	case candidateHoliday:
		result.CandidatesSkippedHoliday++
	case candidateClosingDay:
		result.CandidatesSkippedClosingDay++
	case candidateIncomplete:
		result.CandidatesSkippedIncomplete++
	case candidateCancelled:
		result.CandidatesSkippedException++
	}
}

// weekPatternApplies asks the School Calendar's A/B-week engine whether a
// planning row with weekPattern occurs on date inside period. A nil period
// means no alternation is configured.
func weekPatternApplies(weekPattern int, date timezone.Date, period *schedule.CalendarPeriod) bool {
	if period == nil {
		return true
	}
	return schoolcalendar.WeekPatternApplies(weekPattern, date.String(), schoolcalendar.WeekCycleOf(period.WeekCycleLength, period.WeekCycleAnchor))
}

func isWeekend(date timezone.Date) bool {
	return date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
}

// resolveWindow picks the next-Monday / following-Sunday window the scheduler
// uses by default. If baseDate is a Monday we intentionally skip to the
// following Monday — planning always targets the next block, never the
// current partial one. weeksAhead is clamped to [1, 8].
func resolveWindow(baseDate timezone.Date, weeksAhead int) (from, to timezone.Date) {
	if weeksAhead < 1 {
		weeksAhead = 1
	}
	if weeksAhead > 8 {
		weeksAhead = 8
	}
	// Go's Weekday: Sunday=0, Monday=1, ..., Saturday=6.
	// Days until next Monday (strictly after baseDate):
	//   Sunday → 1, Monday → 7, Tuesday → 6, ..., Saturday → 2.
	var delta int
	switch baseDate.Weekday() {
	case time.Sunday:
		delta = 1
	case time.Monday:
		delta = 7
	default:
		delta = int(time.Saturday-baseDate.Weekday()) + 2
	}
	from = baseDate.AddDays(delta)
	to = from.AddDays(weeksAhead*7 - 1)
	return from, to
}

// enrollmentStudentIsAlumnus reports whether the People Directory projection
// marks the enrollment's student as graduated. Graduated students keep their
// enrollment rows for transition reverts but must drop off every current and
// future planning surface (#405).
func enrollmentStudentIsAlumnus(e *activities.StudentEnrollment) bool {
	return e != nil && e.StudentAlumnus
}

// isEnrollmentValidOn answers: does this enrollment row contribute a student
// to an instance dated `date`, given the instance was scoped to `periodID`?
//
// Rules (RFC §5.3 / E17 / team-iteration-4):
//   - valid_from ≤ date
//   - valid_until IS NULL OR valid_until > date  (end is exclusive; a row
//     whose valid_until equals the instance date is NO LONGER contributing)
//   - calendar_period_id IS NULL OR calendar_period_id == periodID
//   - weekday IS NULL OR weekday == date's ISO weekday (#2129)
//   - selected_weekdays IS NULL/empty OR contains date's ISO weekday
func isEnrollmentValidOn(e *activities.StudentEnrollment, date timezone.Date, periodID int64) bool {
	if e == nil {
		return false
	}
	if !e.ValidFrom.IsZero() && e.ValidFrom.After(date) {
		return false
	}
	if e.ValidUntil != nil && !e.ValidUntil.After(date) {
		return false
	}
	if e.CalendarPeriodID != nil && *e.CalendarPeriodID != periodID {
		return false
	}
	if !rosterWeekdayApplies(e.Weekday, date) {
		return false
	}
	if len(e.SelectedWeekdays) > 0 {
		weekday := isoWeekday(date)
		for _, selected := range e.SelectedWeekdays {
			if selected == weekday {
				return true
			}
		}
		return false
	}
	return true
}

// scheduleEndedOn answers: has this schedule's recurrence ended by `date`?
// valid_until is EXCLUSIVE — the schedule no longer produces instances ON or
// AFTER that date (same convention as enrollment valid_until). A nil
// valid_until means open-ended.
func scheduleEndedOn(sch *activities.Schedule, date timezone.Date) bool {
	return sch != nil && sch.ValidUntil != nil && !sch.ValidUntil.After(date)
}

// scheduleNotStartedOn answers: has this schedule's recurrence not yet begun
// on `date`? valid_from is INCLUSIVE — the schedule produces instances ON and
// AFTER that date, never before. A nil valid_from means an open start.
// Symmetric guard to scheduleEndedOn: the template split sets valid_from on
// successor schedules so materializing a window that begins before the
// effective date does not emit phantom successor instances next to the old
// template's rows.
func scheduleNotStartedOn(sch *activities.Schedule, date timezone.Date) bool {
	return sch != nil && sch.ValidFrom != nil && sch.ValidFrom.After(date)
}

// isSupervisorValidOn mirrors isEnrollmentValidOn for activities.supervisors.
func isSupervisorValidOn(sp *activities.SupervisorPlanned, date timezone.Date, periodID int64) bool {
	if sp == nil {
		return false
	}
	if !sp.ValidFrom.IsZero() && sp.ValidFrom.After(date) {
		return false
	}
	if sp.ValidUntil != nil && !sp.ValidUntil.After(date) {
		return false
	}
	if sp.CalendarPeriodID != nil && *sp.CalendarPeriodID != periodID {
		return false
	}
	return rosterWeekdayApplies(sp.Weekday, date)
}

// effectivePrimarySupervisor resolves overlapping legacy and scoped primary
// rows for one concrete occurrence. A NULL period or weekday is a fallback,
// while an exact period/weekday is more specific and therefore wins. Period
// specificity precedes weekday specificity because materialization first
// selects the occurrence's calendar period and then its weekday.
//
// The trigger can only enforce uniqueness inside an exact scope: clearing an
// unscoped legacy row when a Monday override is inserted would also remove the
// legacy primary from Tuesday. Resolve that overlap here instead, where both
// the occurrence period and date are known.
func effectivePrimarySupervisor(
	supervisors []*activities.SupervisorPlanned,
	date timezone.Date,
	periodID int64,
) (int64, bool) {
	selectedRank := -1
	var selectedStaffID, selectedRowID int64
	for _, supervisor := range supervisors {
		if !isSupervisorValidOn(supervisor, date, periodID) || !supervisor.IsPrimary {
			continue
		}
		rank := 0
		if supervisor.CalendarPeriodID != nil {
			rank += 2
		}
		if supervisor.Weekday != nil {
			rank++
		}
		if rank > selectedRank || (rank == selectedRank && supervisor.ID > selectedRowID) {
			selectedRank = rank
			selectedStaffID = supervisor.StaffID
			selectedRowID = supervisor.ID
		}
	}
	return selectedStaffID, selectedRank >= 0
}

// rosterWeekdayApplies answers whether a weekday-scoped roster row (#2129)
// contributes on `date`. A nil scope is the series-wide default and applies on
// every weekday the template runs; a set scope applies only on that ISO
// weekday. This is the single rule behind per-weekday staff and child lists —
// the template writer expands "shared default + deviations" into concrete
// per-weekday rows, so nothing here needs to know about that distinction.
func rosterWeekdayApplies(weekday *int, date timezone.Date) bool {
	return weekday == nil || *weekday == isoWeekday(date)
}

// applyException returns the effective (start, end, room) for a candidate and
// whether the candidate should be skipped outright (cancelled exception).
// A nil exception is a no-op.
//
// Start/end overrides are normalized through extractTimeOfDay so the values
// land cleanly in schedule.activity_instances' TIME columns. The exception
// table's columns are TIME-typed but bun reads them as time.Time with year 0
// (Postgres TIME has no date), which would be rejected by Postgres on the
// subsequent INSERT if passed through unchanged.
func applyException(base materialParams, exc *schedule.ActivityException) (materialParams, bool) {
	if exc == nil {
		return base, false
	}
	if exc.ExceptionType == schedule.ActivityExceptionCancelled {
		return base, true
	}
	out := base
	if exc.StartTime != nil {
		out.StartTime = extractTimeOfDay(*exc.StartTime)
	}
	if exc.EndTime != nil {
		out.EndTime = extractTimeOfDay(*exc.EndTime)
	}
	if exc.RoomID != nil {
		out.RoomID = *exc.RoomID
	}
	return out, false
}

// selectPeriod implements the period-selection rule from the WP-B8 plan §1:
//
//  1. If template pins calendar_period_id, use that period iff `date` is
//     within its [start_date, end_date] range.
//  2. Otherwise, among active periods containing `date`, pick the one with
//     the lowest ID (deterministic). Warn if more than one matches so
//     operators see overlap in logs without the service blowing up.
//  3. If no period matches, return nil (caller counts as SkippedNoPeriod).
//
// Active-only filtering is the caller's responsibility: `periods` must already
// contain only is_active = true rows.
//
// The schedule may also pin a period via activities.schedules.calendar_period_id;
// we prefer the schedule's pin over the template's when both are present
// because the schedule is the more specific scope.
func selectPeriod(
	tmpl *activities.Group,
	sch *activities.Schedule,
	date timezone.Date,
	periods []*schedule.CalendarPeriod,
	logger *slog.Logger,
) *schedule.CalendarPeriod {
	if pinned := schedulePinnedPeriodID(tmpl, sch); pinned != nil {
		return pinnedPeriodOn(periods, *pinned, date)
	}
	matches := periodsContaining(periods, date)
	if len(matches) == 0 {
		return nil
	}
	if len(matches) > 1 && logger != nil {
		ids := make([]int64, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		logger.Warn("overlapping active calendar periods for materialization",
			slog.Int64("template_id", tmpl.ID),
			slog.String("date", date.String()),
			slog.Any("candidate_period_ids", ids),
			slog.Int64("chosen_period_id", matches[0].ID),
		)
	}
	return matches[0]
}

// pinnedPeriodOn returns the pinned period when it is active and contains the
// date. A pin to a period outside the active set is treated as no match.
func pinnedPeriodOn(periods []*schedule.CalendarPeriod, pinned int64, date timezone.Date) *schedule.CalendarPeriod {
	for _, p := range periods {
		if p.ID == pinned {
			if p.ContainsDay(schedule.Date(date)) {
				return p
			}
			return nil
		}
	}
	return nil
}

// periodsContaining returns the active periods containing the date, sorted
// ascending by ID.
func periodsContaining(periods []*schedule.CalendarPeriod, date timezone.Date) []*schedule.CalendarPeriod {
	var matches []*schedule.CalendarPeriod
	for _, p := range periods {
		if p.ContainsDay(schedule.Date(date)) {
			matches = append(matches, p)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	return matches
}

// schedulePinnedPeriodID prefers the schedule's calendar_period_id when set
// (it's the more specific scope), falling back to the template's if present.
// Returns nil when neither is set.
func schedulePinnedPeriodID(tmpl *activities.Group, sch *activities.Schedule) *int64 {
	if sch != nil && sch.CalendarPeriodID != nil {
		return sch.CalendarPeriodID
	}
	// The schedule row's own pin (checked above) is the more specific,
	// materialization-time-authoritative value. The template's pin
	// (activities.Group.CalendarPeriodID, issue #1838) is the fallback for
	// templates whose schedule rows carry no explicit pin.
	if tmpl != nil && tmpl.CalendarPeriodID != nil {
		return tmpl.CalendarPeriodID
	}
	return nil
}

func buildExistingIndex(existing []*schedule.ActivityInstance) map[existingKey]struct{} {
	idx := make(map[existingKey]struct{}, len(existing))
	for _, inst := range existing {
		if inst.ActivityGroupID == nil {
			continue // spontaneous rows are not keyed by template
		}
		k := existingKey{
			ActivityGroupID: *inst.ActivityGroupID,
			Date:            timezone.Date(inst.Date),
			StartTime:       formatTimeOfDay(inst.StartTime),
		}
		idx[k] = struct{}{}
	}
	return idx
}

func buildExceptionIndex(exceptions []*schedule.ActivityException) map[exceptionKey]*schedule.ActivityException {
	idx := make(map[exceptionKey]*schedule.ActivityException, len(exceptions))
	for _, e := range exceptions {
		idx[exceptionKey{e.ActivityGroupID, timezone.Date(e.ExceptionDate)}] = e
	}
	return idx
}

// formatTimeOfDay formats the time-of-day component of t as "15:04:05", based
// on the time's own Hour/Minute/Second components. Also location-independent.
func formatTimeOfDay(t time.Time) string {
	return fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
}

// extractTimeOfDay is a thin wrapper around timezone.NormalizeWallClock preserved so
// the local call sites read unchanged. See timezone.NormalizeWallClock for the
// full rationale on why TIMESTAMPTZ → TIME round-trips need this.
func extractTimeOfDay(t time.Time) time.Time {
	return timezone.NormalizeWallClock(t)
}
