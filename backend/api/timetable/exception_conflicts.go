// Package timetable — WP-B13 exception-conflict warnings.
//
//	GET /api/timetable/exception-conflicts?date=YYYY-MM-DD&date_to=YYYY-MM-DD
//
// Surfaces two classes of planning conflicts between activity_exceptions and
// student arrival expectations:
//
//   - cancelled_instance_with_scheduled_arrivals: a student is still flagged
//     as expected on an instance whose exception cancels the occurrence.
//   - modified_instance_time_mismatch: a modified exception pushes the
//     start time earlier than the student's resolved arrival, meaning the
//     student will miss the opening.
//
// Permission: SchedulesRead. Max range 14 days. Only today/future (same
// Berlin-local rule as /gaps). Empty list on no-conflict → 200, never 404.
package timetable

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// maxExceptionConflictRangeDays mirrors the /gaps and /week caps. A longer
// horizon has no frontend consumer today and would enlarge the bulk arrival
// lookups linearly with N dates.
const maxExceptionConflictRangeDays = 14

// Conflict kinds — stable strings surfaced to clients.
const (
	ConflictKindCancelledArrivals = "cancelled_instance_with_scheduled_arrivals"
	ConflictKindModifiedMismatch  = "modified_instance_time_mismatch"
)

// ConflictEntry is a single row in the conflict response. The optional
// string fields are populated per kind (reason only for cancelled, the
// two start-time strings only for modified) and serialise as omitempty.
type ConflictEntry struct {
	Kind               string `json:"kind"`
	Date               string `json:"date"`
	ActivityGroupID    int64  `json:"activity_group_id"`
	InstanceID         int64  `json:"instance_id"`
	ActivityTitle      string `json:"activity_title"`
	StudentID          int64  `json:"student_id"`
	ExpectedArrival    string `json:"expected_arrival,omitempty"`
	ArrivalSource      string `json:"arrival_source"`
	CancellationReason string `json:"cancellation_reason,omitempty"`
	OriginalStartTime  string `json:"original_start_time,omitempty"`
	ModifiedStartTime  string `json:"modified_start_time,omitempty"`
}

// ConflictsResponse is the 200 body for GET /exception-conflicts.
type ConflictsResponse struct {
	From      string          `json:"from"`
	To        string          `json:"to"`
	Conflicts []ConflictEntry `json:"conflicts"`
}

// getExceptionConflicts handles GET /api/timetable/exception-conflicts.
func (rs *Resource) getExceptionConflicts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	from, to, ok := parseConflictRange(w, r)
	if !ok {
		return
	}

	if rs.timetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	exceptions, err := rs.timetableData.GetActivityExceptionsByDateRange(ctx, from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load activity exceptions failed", err))
		return
	}
	if len(exceptions) == 0 {
		respondEmptyConflicts(w, r, from, to)
		return
	}

	instances, err := rs.timetableData.GetActivityInstancesByDateRange(ctx, from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load activity instances failed", err))
		return
	}

	// Build a per-exception list of affected instances. Exceptions that do
	// not map to any materialised instance (e.g. the exception was created
	// before the scheduler next runs) are skipped with a debug log — not an
	// error. This is expected because materialisation and exception creation
	// are independent flows.
	instancesByKey := indexInstancesByGroupDate(instances)
	affected := make([]exceptionInstancePair, 0, len(exceptions))
	for _, exc := range exceptions {
		key := groupDateKey{
			GroupID: exc.ActivityGroupID,
			Date:    dateKey(exc.ExceptionDate),
		}
		matching := instancesByKey[key]
		if len(matching) == 0 {
			rs.getLogger().Debug("exception without matching instance, skipping",
				slog.Int64("activity_group_id", exc.ActivityGroupID),
				slog.String("exception_date", dateKey(exc.ExceptionDate)),
				slog.String("exception_type", exc.ExceptionType),
			)
			continue
		}
		for _, inst := range matching {
			affected = append(affected, exceptionInstancePair{instance: inst, exception: exc})
		}
	}
	if len(affected) == 0 {
		respondEmptyConflicts(w, r, from, to)
		return
	}

	affectedInstanceIDs := uniqueInstanceIDs(affected)
	expectedStudents, err := rs.timetableData.GetExpectedInstanceStudentsByInstanceIDs(ctx, affectedInstanceIDs)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load expected students failed", err))
		return
	}
	if len(expectedStudents) == 0 {
		respondEmptyConflicts(w, r, from, to)
		return
	}

	expectedByInstance := make(map[int64][]*scheduleModel.InstanceStudent, len(affectedInstanceIDs))
	for _, es := range expectedStudents {
		expectedByInstance[es.InstanceID] = append(expectedByInstance[es.InstanceID], es)
	}

	// Collect the (student, date) tuples we still need arrival info for.
	datesToStudents := make(map[string]map[int64]struct{})
	for _, a := range affected {
		stus, ok := expectedByInstance[a.instance.ID]
		if !ok {
			continue
		}
		dk := dateKey(a.exception.ExceptionDate)
		if datesToStudents[dk] == nil {
			datesToStudents[dk] = make(map[int64]struct{})
		}
		for _, s := range stus {
			datesToStudents[dk][s.StudentID] = struct{}{}
		}
	}

	arrivalPre, err := rs.loadArrivalPreload(ctx, affected, datesToStudents)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load arrival preload failed", err))
		return
	}

	templatePre, err := rs.loadTemplatePreload(ctx, affected)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load template start-times failed", err))
		return
	}

	conflicts := rs.buildConflicts(affected, expectedByInstance, arrivalPre, templatePre)

	sort.SliceStable(conflicts, func(i, j int) bool {
		if conflicts[i].Date != conflicts[j].Date {
			return conflicts[i].Date < conflicts[j].Date
		}
		if conflicts[i].ActivityGroupID != conflicts[j].ActivityGroupID {
			return conflicts[i].ActivityGroupID < conflicts[j].ActivityGroupID
		}
		return conflicts[i].StudentID < conflicts[j].StudentID
	})

	rs.getLogger().Info("exception conflicts",
		slog.String("from", from.String()),
		slog.String("to", to.String()),
		slog.Int("exception_count", len(exceptions)),
		slog.Int("conflict_count", len(conflicts)),
	)

	common.Respond(w, r, http.StatusOK, ConflictsResponse{
		From:      from.String(),
		To:        to.String(),
		Conflicts: conflicts,
	}, "Exception conflicts retrieved")
}

// parseConflictRange mirrors /gaps: date required, date_to optional (defaults
// to date), range <= 14 days, date >= today in Berlin-local.
func parseConflictRange(w http.ResponseWriter, r *http.Request) (from, to timezone.Date, ok bool) {
	q := r.URL.Query()

	dateStr := q.Get("date")
	if dateStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date is required")))
		return timezone.Date{}, timezone.Date{}, false
	}
	parsedFrom, err := berlinDate(dateStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date format, expected YYYY-MM-DD")))
		return timezone.Date{}, timezone.Date{}, false
	}

	parsedTo := parsedFrom
	if toStr := q.Get("date_to"); toStr != "" {
		parsedTo, err = berlinDate(toStr)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date_to format, expected YYYY-MM-DD")))
			return timezone.Date{}, timezone.Date{}, false
		}
	}

	if parsedFrom.After(parsedTo) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("'date' must be before or equal to 'date_to'")))
		return timezone.Date{}, timezone.Date{}, false
	}
	if inclusiveDayCount(parsedFrom, parsedTo) > maxExceptionConflictRangeDays {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			fmt.Errorf("date range exceeds maximum of %d days", maxExceptionConflictRangeDays)))
		return timezone.Date{}, timezone.Date{}, false
	}

	// "today or future" comparison on Berlin calendar days, matching the
	// /gaps endpoint's rule.
	if parsedFrom.Before(timezone.TodayDate()) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("'date' must be today or a future date")))
		return timezone.Date{}, timezone.Date{}, false
	}

	return parsedFrom, parsedTo, true
}

func respondEmptyConflicts(w http.ResponseWriter, r *http.Request, from, to timezone.Date) {
	common.Respond(w, r, http.StatusOK, ConflictsResponse{
		From:      from.String(),
		To:        to.String(),
		Conflicts: []ConflictEntry{},
	}, "Exception conflicts retrieved")
}

// -----------------------------------------------------------------------------
// Indexing / helpers
// -----------------------------------------------------------------------------

type groupDateKey struct {
	GroupID int64
	Date    string
}

type exceptionInstancePair struct {
	instance  *scheduleModel.ActivityInstance
	exception *scheduleModel.ActivityException
}

func indexInstancesByGroupDate(instances []*scheduleModel.ActivityInstance) map[groupDateKey][]*scheduleModel.ActivityInstance {
	out := make(map[groupDateKey][]*scheduleModel.ActivityInstance, len(instances))
	for _, inst := range instances {
		if inst.ActivityGroupID == nil {
			continue // spontaneous — no template, so never tied to an exception
		}
		key := groupDateKey{GroupID: *inst.ActivityGroupID, Date: dateKey(inst.Date)}
		out[key] = append(out[key], inst)
	}
	return out
}

func uniqueInstanceIDs(affected []exceptionInstancePair) []int64 {
	seen := make(map[int64]struct{}, len(affected))
	ids := make([]int64, 0, len(affected))
	for _, a := range affected {
		if _, ok := seen[a.instance.ID]; ok {
			continue
		}
		seen[a.instance.ID] = struct{}{}
		ids = append(ids, a.instance.ID)
	}
	return ids
}

// -----------------------------------------------------------------------------
// Arrival preload
// -----------------------------------------------------------------------------

type arrivalPreload struct {
	// byException[dateKey][studentID] — non-nil row means "arrival exception
	// exists for this student on this date" (ExpectedArrival may still be nil
	// inside the row — that flags an explicit absence).
	byException map[string]map[int64]*scheduleModel.StudentArrivalException
	// bySchedule[weekday][studentID] — weekly recurring schedule.
	bySchedule map[int]map[int64]*scheduleModel.StudentArrivalSchedule
}

func (rs *Resource) loadArrivalPreload(
	ctx context.Context,
	affected []exceptionInstancePair,
	datesToStudents map[string]map[int64]struct{},
) (*arrivalPreload, error) {
	pre := &arrivalPreload{
		byException: map[string]map[int64]*scheduleModel.StudentArrivalException{},
		bySchedule:  map[int]map[int64]*scheduleModel.StudentArrivalSchedule{},
	}

	// Arrival exceptions: one query per unique date. Range is capped at 14
	// by parseConflictRange so this is bounded and predictable.
	dateObjByKey := make(map[string]timezone.Date)
	for _, a := range affected {
		dateObjByKey[dateKey(a.exception.ExceptionDate)] = a.exception.ExceptionDate
	}
	for dk, stuMap := range datesToStudents {
		ids := make([]int64, 0, len(stuMap))
		for id := range stuMap {
			ids = append(ids, id)
		}
		excs, err := rs.timetableData.GetArrivalExceptionsByStudentIDsAndDate(ctx, ids, dateObjByKey[dk])
		if err != nil {
			return nil, fmt.Errorf("load arrival exceptions for %s: %w", dk, err)
		}
		byStu := make(map[int64]*scheduleModel.StudentArrivalException, len(excs))
		for _, e := range excs {
			byStu[e.StudentID] = e
		}
		pre.byException[dk] = byStu
	}

	// Arrival schedules: one query per unique weekday, and only for students
	// that don't already have an exception on that date (exceptions win).
	schedulesNeededByWd := make(map[int]map[int64]struct{})
	for dk, stuMap := range datesToStudents {
		wd := isoWeekday(dateObjByKey[dk])
		if wd < scheduleModel.WeekdayMonday || wd > scheduleModel.WeekdayFriday {
			continue
		}
		for stuID := range stuMap {
			if byStu, ok := pre.byException[dk]; ok {
				if _, hasExc := byStu[stuID]; hasExc {
					continue
				}
			}
			if schedulesNeededByWd[wd] == nil {
				schedulesNeededByWd[wd] = make(map[int64]struct{})
			}
			schedulesNeededByWd[wd][stuID] = struct{}{}
		}
	}
	for wd, stuMap := range schedulesNeededByWd {
		ids := make([]int64, 0, len(stuMap))
		for id := range stuMap {
			ids = append(ids, id)
		}
		schedules, err := rs.timetableData.GetArrivalSchedulesByStudentIDsAndWeekday(ctx, ids, wd)
		if err != nil {
			return nil, fmt.Errorf("load arrival schedules for weekday %d: %w", wd, err)
		}
		byStu := make(map[int64]*scheduleModel.StudentArrivalSchedule, len(schedules))
		for _, s := range schedules {
			byStu[s.StudentID] = s
		}
		pre.bySchedule[wd] = byStu
	}

	return pre, nil
}

// resolveArrival returns (arrivalTime, source) for a student on a date.
// source is one of SlotSourceException / SlotSourceSchedule / SlotSourceNone.
// arrivalTime is time.Time{} (IsZero) when the arrival is unknown — source
// distinguishes "none" (no rule at all) from "exception without expected_arrival"
// (explicit absence).
func (pre *arrivalPreload) resolveArrival(studentID int64, date timezone.Date) (time.Time, string) {
	dk := dateKey(date)
	if byStu, ok := pre.byException[dk]; ok {
		if exc, has := byStu[studentID]; has && exc != nil {
			if exc.ExpectedArrival != nil {
				return *exc.ExpectedArrival, SlotSourceException
			}
			return time.Time{}, SlotSourceException // explicit absence
		}
	}
	wd := isoWeekday(date)
	if wd >= scheduleModel.WeekdayMonday && wd <= scheduleModel.WeekdayFriday {
		if byStu, ok := pre.bySchedule[wd]; ok {
			if sched, has := byStu[studentID]; has && sched != nil {
				return sched.ExpectedArrival, SlotSourceSchedule
			}
		}
	}
	return time.Time{}, SlotSourceNone
}

// -----------------------------------------------------------------------------
// Template start-time preload
// -----------------------------------------------------------------------------

type templatePreload struct {
	// byKey[{group, weekday}] — all template start_times for that pair,
	// in ascending order. A single element is the unambiguous case; multiple
	// elements mean the template runs multiple slots on that weekday and we
	// cannot pick the "original" without more context.
	byKey map[groupWeekdayKey][]time.Time
}

type groupWeekdayKey struct {
	GroupID int64
	Weekday int
}

func (rs *Resource) loadTemplatePreload(
	ctx context.Context,
	affected []exceptionInstancePair,
) (*templatePreload, error) {
	// We only need template start-times for modified exceptions that carry
	// a new start_time. Room-only modifies don't trigger Type 2 and
	// cancellations don't need the "original" field at all.
	needed := make(map[int64]struct{})
	for _, a := range affected {
		if a.exception.ExceptionType != scheduleModel.ActivityExceptionModified {
			continue
		}
		if a.exception.StartTime == nil {
			continue
		}
		needed[a.exception.ActivityGroupID] = struct{}{}
	}

	pre := &templatePreload{byKey: map[groupWeekdayKey][]time.Time{}}
	if len(needed) == 0 {
		return pre, nil
	}

	ids := make([]int64, 0, len(needed))
	for id := range needed {
		ids = append(ids, id)
	}
	rows, err := rs.timetableData.GetTemplateStartTimesByGroupIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load template start times: %w", err)
	}
	for _, row := range rows {
		k := groupWeekdayKey{GroupID: row.ActivityGroupID, Weekday: row.Weekday}
		pre.byKey[k] = append(pre.byKey[k], row.StartTime)
	}
	return pre, nil
}

// resolveOriginalStart picks the unambiguous template start time for the
// (group, weekday) pair. If the template defines more than one schedule on
// that weekday (e.g. morning + afternoon slots), we cannot tell which one
// was modified — we emit the warning anyway but surface original_start_time
// as empty.
func (pre *templatePreload) resolveOriginalStart(groupID int64, weekday int, logger *slog.Logger, excDate timezone.Date) (string, bool) {
	starts, ok := pre.byKey[groupWeekdayKey{GroupID: groupID, Weekday: weekday}]
	if !ok || len(starts) == 0 {
		logger.Warn("modified exception but no template schedule for weekday",
			slog.Int64("activity_group_id", groupID),
			slog.Int("weekday", weekday),
			slog.String("exception_date", dateKey(excDate)),
		)
		return "", false
	}
	if len(starts) > 1 {
		logger.Warn("multiple template schedules for weekday, cannot disambiguate original_start_time",
			slog.Int64("activity_group_id", groupID),
			slog.Int("weekday", weekday),
			slog.Int("schedule_count", len(starts)),
			slog.String("exception_date", dateKey(excDate)),
		)
		return "", false
	}
	return starts[0].Format("15:04"), true
}

// -----------------------------------------------------------------------------
// Build conflicts
// -----------------------------------------------------------------------------

func (rs *Resource) buildConflicts(
	affected []exceptionInstancePair,
	expectedByInstance map[int64][]*scheduleModel.InstanceStudent,
	arrivalPre *arrivalPreload,
	templatePre *templatePreload,
) []ConflictEntry {
	// Upper bound: every affected (instance, exception) pair contributes
	// at most one warning per expected student. Count them up-front so the
	// slice grows at most once for realistic workloads (cancellations often
	// surface 20-30 students per instance).
	maxWarnings := 0
	for _, a := range affected {
		maxWarnings += len(expectedByInstance[a.instance.ID])
	}
	out := make([]ConflictEntry, 0, maxWarnings)
	for _, a := range affected {
		stus, ok := expectedByInstance[a.instance.ID]
		if !ok {
			continue
		}
		for _, is := range stus {
			entry, emit := rs.conflictForStudent(a, is, arrivalPre, templatePre)
			if !emit {
				continue
			}
			out = append(out, entry)
		}
	}
	return out
}

func (rs *Resource) conflictForStudent(
	a exceptionInstancePair,
	is *scheduleModel.InstanceStudent,
	arrivalPre *arrivalPreload,
	templatePre *templatePreload,
) (ConflictEntry, bool) {
	arrivalTime, arrivalSource := arrivalPre.resolveArrival(is.StudentID, a.exception.ExceptionDate)

	// Explicit-absence handling (Q5): if the arrival exception says the
	// student is absent on that date (ExpectedArrival=nil → source=
	// "exception" with zero time), both conflict kinds are noise. The child
	// isn't coming to school — cancelling or moving the activity has no
	// impact on them. Reconciling the instance_students.status='expected'
	// row against the arrival-exception absence is a different concern
	// (future B17-style reconciliation job).
	if arrivalSource == SlotSourceException && arrivalTime.IsZero() {
		return ConflictEntry{}, false
	}

	base := ConflictEntry{
		Date:            dateKey(a.exception.ExceptionDate),
		ActivityGroupID: a.exception.ActivityGroupID,
		InstanceID:      a.instance.ID,
		ActivityTitle:   a.instance.Title,
		StudentID:       is.StudentID,
		ArrivalSource:   arrivalSource,
	}
	if !arrivalTime.IsZero() {
		base.ExpectedArrival = arrivalTime.Format("15:04")
	}

	switch a.exception.ExceptionType {
	case scheduleModel.ActivityExceptionCancelled:
		base.Kind = ConflictKindCancelledArrivals
		if a.exception.Reason != nil && *a.exception.Reason != "" {
			base.CancellationReason = *a.exception.Reason
		}
		return base, true

	case scheduleModel.ActivityExceptionModified:
		if a.exception.StartTime == nil {
			return ConflictEntry{}, false // room-only modify — no time mismatch possible
		}
		if arrivalSource == SlotSourceNone {
			return ConflictEntry{}, false // no arrival info to compare against
		}
		// Both the exception.start_time and the arrival columns are TIME
		// values, but bun deserialises them with different year anchors
		// (0000 vs 2000 depending on the column). Comparing via .After()
		// without normalisation would be driven by the year difference, not
		// the wall-clock. timezone.WallClock on both sides pins both to
		// year 0001 UTC so the comparison is purely HH:MM:SS.
		modifiedStart := timezone.WallClock(*a.exception.StartTime)
		arrivalNorm := timezone.WallClock(arrivalTime)

		// Only emit when the student's arrival is strictly after the new
		// start — equal means the student arrives exactly at the start,
		// which is not a conflict.
		if !arrivalNorm.After(modifiedStart) {
			return ConflictEntry{}, false
		}

		base.Kind = ConflictKindModifiedMismatch
		base.ModifiedStartTime = modifiedStart.Format("15:04")
		if original, ok := templatePre.resolveOriginalStart(
			a.exception.ActivityGroupID,
			isoWeekday(a.exception.ExceptionDate),
			rs.getLogger(),
			a.exception.ExceptionDate,
		); ok {
			base.OriginalStartTime = original
		}
		return base, true
	}

	// Unknown exception type: log and skip. The model validation rejects
	// unknown types on write, so this is defense against future additions
	// that forget to update this switch.
	rs.getLogger().Warn("unknown exception type, skipping",
		slog.Int64("activity_group_id", a.exception.ActivityGroupID),
		slog.String("exception_type", a.exception.ExceptionType),
	)
	return ConflictEntry{}, false
}
