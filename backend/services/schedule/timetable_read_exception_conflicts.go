package schedule

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// Conflict kinds — stable strings surfaced to clients.
const (
	ConflictKindCancelledArrivals = "cancelled_instance_with_scheduled_arrivals"
	ConflictKindModifiedMismatch  = "modified_instance_time_mismatch"
)

// ExceptionConflict is one detected planning conflict between an
// activity_exception and a student's arrival expectation. It is the read-side
// domain result the handler maps 1:1 onto its wire DTO. Optional string fields
// are populated per kind and are empty otherwise.
type ExceptionConflict struct {
	Kind               string
	Date               string
	ActivityGroupID    int64
	InstanceID         int64
	ActivityTitle      string
	StudentID          int64
	ExpectedArrival    string
	ArrivalSource      string
	CancellationReason string
	OriginalStartTime  string
	ModifiedStartTime  string
}

// DetectExceptionConflicts computes the two classes of planning conflict
// (cancelled_instance_with_scheduled_arrivals, modified_instance_time_mismatch)
// between activity_exceptions and student arrival expectations in the inclusive
// [from, to] window. Read-only: no writes, no transactional concerns. Returns
// an unsorted slice (the caller sorts before responding).
func (s *TimetableDataService) DetectExceptionConflicts(ctx context.Context, from, to timezone.Date, logger *slog.Logger) ([]ExceptionConflict, error) {
	exceptions, err := s.deps.ActivityExceptionRepo.FindByDateRange(ctx, from, to)
	if err != nil {
		return nil, err
	}
	if len(exceptions) == 0 {
		return []ExceptionConflict{}, nil
	}

	instances, err := s.deps.ActivityInstanceRepo.FindByTenantAndDateRange(ctx, from, to)
	if err != nil {
		return nil, err
	}

	affected := buildAffectedPairs(exceptions, instances, logger)
	if len(affected) == 0 {
		return []ExceptionConflict{}, nil
	}

	expectedStudents, err := s.deps.InstanceStudentRepo.FindExpectedByInstanceIDs(ctx, uniqueInstanceIDs(affected))
	if err != nil {
		return nil, err
	}
	if len(expectedStudents) == 0 {
		return []ExceptionConflict{}, nil
	}

	expectedByInstance := make(map[int64][]*scheduleModel.InstanceStudent, len(expectedStudents))
	for _, es := range expectedStudents {
		expectedByInstance[es.InstanceID] = append(expectedByInstance[es.InstanceID], es)
	}

	arrivalPre, err := s.loadArrivalPreload(ctx, affected, datesToStudents(affected, expectedByInstance))
	if err != nil {
		return nil, err
	}
	templatePre, err := s.loadTemplatePreload(ctx, affected)
	if err != nil {
		return nil, err
	}

	return buildConflicts(affected, expectedByInstance, arrivalPre, templatePre, logger), nil
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

// buildAffectedPairs joins exceptions to their materialised instances. An
// exception with no matching instance (created before the scheduler next runs)
// is skipped with a debug log — expected, since materialisation and exception
// creation are independent flows.
func buildAffectedPairs(
	exceptions []*scheduleModel.ActivityException,
	instances []*scheduleModel.ActivityInstance,
	logger *slog.Logger,
) []exceptionInstancePair {
	instancesByKey := indexInstancesByGroupDate(instances)
	affected := make([]exceptionInstancePair, 0, len(exceptions))
	for _, exc := range exceptions {
		key := groupDateKey{GroupID: exc.ActivityGroupID, Date: timetableDateKey(exc.ExceptionDate)}
		matching := instancesByKey[key]
		if len(matching) == 0 {
			logger.Debug("exception without matching instance, skipping",
				slog.Int64("activity_group_id", exc.ActivityGroupID),
				slog.String("exception_date", timetableDateKey(exc.ExceptionDate)),
				slog.String("exception_type", exc.ExceptionType),
			)
			continue
		}
		for _, inst := range matching {
			affected = append(affected, exceptionInstancePair{instance: inst, exception: exc})
		}
	}
	return affected
}

func indexInstancesByGroupDate(instances []*scheduleModel.ActivityInstance) map[groupDateKey][]*scheduleModel.ActivityInstance {
	out := make(map[groupDateKey][]*scheduleModel.ActivityInstance, len(instances))
	for _, inst := range instances {
		if inst.ActivityGroupID == nil {
			continue // spontaneous — no template, so never tied to an exception
		}
		key := groupDateKey{GroupID: *inst.ActivityGroupID, Date: timetableDateKey(inst.Date)}
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

// datesToStudents collects the (date → student-set) tuples we still need
// arrival info for.
func datesToStudents(
	affected []exceptionInstancePair,
	expectedByInstance map[int64][]*scheduleModel.InstanceStudent,
) map[string]map[int64]struct{} {
	out := make(map[string]map[int64]struct{})
	for _, a := range affected {
		stus, ok := expectedByInstance[a.instance.ID]
		if !ok {
			continue
		}
		dk := timetableDateKey(a.exception.ExceptionDate)
		if out[dk] == nil {
			out[dk] = make(map[int64]struct{})
		}
		for _, es := range stus {
			out[dk][es.StudentID] = struct{}{}
		}
	}
	return out
}

// -----------------------------------------------------------------------------
// Arrival preload
// -----------------------------------------------------------------------------

type arrivalPreload struct {
	// byException[dateKey][studentID] — non-nil row means "arrival exception
	// exists for this student on this date" (ExpectedArrival may still be nil
	// inside the row — that flags an explicit absence).
	byException map[string]map[int64]*scheduleModel.StudentArrivalException
	// bySchedule[dateKey][studentID] — the recurring arrival row applicable on
	// that date. Keyed by date rather than by weekday because whether a
	// weekday is a care day at all can change within the window once the
	// approved bookings decide it (#2414, ADR 0005).
	bySchedule map[string]map[int64]*scheduleModel.StudentArrivalSchedule
}

func (s *TimetableDataService) loadArrivalPreload(
	ctx context.Context,
	affected []exceptionInstancePair,
	dates map[string]map[int64]struct{},
) (*arrivalPreload, error) {
	pre := &arrivalPreload{
		byException: map[string]map[int64]*scheduleModel.StudentArrivalException{},
		bySchedule:  map[string]map[int64]*scheduleModel.StudentArrivalSchedule{},
	}

	// Arrival exceptions: one query per unique date. Range is capped at
	// MaxTimetableReadRangeDays upstream so this is bounded.
	dateObjByKey := make(map[string]timezone.Date)
	for _, a := range affected {
		dateObjByKey[timetableDateKey(a.exception.ExceptionDate)] = a.exception.ExceptionDate
	}
	for dk, stuMap := range dates {
		ids := slices.Collect(maps.Keys(stuMap))
		excs, err := s.deps.ArrivalExceptionRepo.FindByStudentIDsAndDate(ctx, ids, dateObjByKey[dk])
		if err != nil {
			return nil, err
		}
		byStu := make(map[int64]*scheduleModel.StudentArrivalException, len(excs))
		for _, e := range excs {
			byStu[e.StudentID] = e
		}
		pre.byException[dk] = byStu
	}

	// Arrival schedules, for the students that don't already have an exception
	// on that date (exceptions win).
	needed := make(map[string]map[int64]struct{})
	for dk, stuMap := range dates {
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
			if needed[dk] == nil {
				needed[dk] = make(map[int64]struct{})
			}
			needed[dk][stuID] = struct{}{}
		}
	}
	if err := s.fillArrivalSchedules(ctx, pre, needed, dateObjByKey); err != nil {
		return nil, err
	}

	return pre, nil
}

// fillArrivalSchedules resolves the recurring arrival row per (date, student)
// through the baseline projection (#2414): the class timetable supplies the
// time, and with enrollment.bookings_authoritative on the approved bookings
// supply the care days — so a stale row on an unbooked weekday no longer
// produces a conflict warning about a child that is not coming.
//
// Without a baseline reader (CLI, partial test facades) the stored rows apply
// on every date of their weekday, which is the pre-#2414 behaviour.
func (s *TimetableDataService) fillArrivalSchedules(
	ctx context.Context,
	pre *arrivalPreload,
	needed map[string]map[int64]struct{},
	dateObjByKey map[string]timezone.Date,
) error {
	if len(needed) == 0 {
		return nil
	}
	if s.deps.ArrivalBaselines == nil {
		return s.fillStoredArrivalSchedules(ctx, pre, needed, dateObjByKey)
	}

	studentIDs := make(map[int64]struct{})
	var from, to timezone.Date
	for dk, stuMap := range needed {
		date := dateObjByKey[dk]
		if from.IsZero() || date.Before(from) {
			from = date
		}
		if to.IsZero() || date.After(to) {
			to = date
		}
		for stuID := range stuMap {
			studentIDs[stuID] = struct{}{}
		}
	}

	projection, err := s.deps.ArrivalBaselines.Project(ctx, slices.Collect(maps.Keys(studentIDs)), from, to)
	if err != nil {
		return err
	}
	for dk, stuMap := range needed {
		byStu := make(map[int64]*scheduleModel.StudentArrivalSchedule, len(stuMap))
		for stuID := range stuMap {
			if row := projection.ForDate(stuID, dateObjByKey[dk]); row != nil {
				byStu[stuID] = row
			}
		}
		pre.bySchedule[dk] = byStu
	}
	return nil
}

func (s *TimetableDataService) fillStoredArrivalSchedules(
	ctx context.Context,
	pre *arrivalPreload,
	needed map[string]map[int64]struct{},
	dateObjByKey map[string]timezone.Date,
) error {
	// One query per unique weekday, as before the projection existed.
	byWeekday := make(map[int]map[int64]*scheduleModel.StudentArrivalSchedule)
	idsByWeekday := make(map[int]map[int64]struct{})
	for dk, stuMap := range needed {
		wd := isoWeekday(dateObjByKey[dk])
		if idsByWeekday[wd] == nil {
			idsByWeekday[wd] = make(map[int64]struct{})
		}
		maps.Copy(idsByWeekday[wd], stuMap)
	}
	for wd, stuMap := range idsByWeekday {
		schedules, err := s.deps.ArrivalScheduleRepo.FindByStudentIDsAndWeekday(ctx, slices.Collect(maps.Keys(stuMap)), wd)
		if err != nil {
			return err
		}
		byStu := make(map[int64]*scheduleModel.StudentArrivalSchedule, len(schedules))
		for _, sc := range schedules {
			byStu[sc.StudentID] = sc
		}
		byWeekday[wd] = byStu
	}
	for dk := range needed {
		pre.bySchedule[dk] = byWeekday[isoWeekday(dateObjByKey[dk])]
	}
	return nil
}

// resolveArrival returns (arrivalTime, source) for a student on a date using
// the shared exception-over-schedule precedence (ResolveSlotSource). arrivalTime
// is time.Time{} (IsZero) when unknown — source distinguishes "none" (no rule)
// from "exception without expected_arrival" (explicit absence).
func (pre *arrivalPreload) resolveArrival(studentID int64, date timezone.Date) (time.Time, string) {
	dk := timetableDateKey(date)
	exc, hasExc := lookupArrivalException(pre.byException, dk, studentID)
	wd := isoWeekday(date)
	sched, hasSched := lookupArrivalSchedule(pre.bySchedule, dk, studentID)

	switch ResolveSlotSource(hasExc, hasSched, wd) {
	case SlotSourceException:
		if exc.ExpectedArrival != nil {
			return *exc.ExpectedArrival, SlotSourceException
		}
		return time.Time{}, SlotSourceException // explicit absence
	case SlotSourceSchedule:
		return sched.ExpectedArrival, SlotSourceSchedule
	default:
		return time.Time{}, SlotSourceNone
	}
}

func lookupArrivalException(byException map[string]map[int64]*scheduleModel.StudentArrivalException, dateKey string, studentID int64) (*scheduleModel.StudentArrivalException, bool) {
	if byStu, ok := byException[dateKey]; ok {
		if exc, has := byStu[studentID]; has && exc != nil {
			return exc, true
		}
	}
	return nil, false
}

func lookupArrivalSchedule(bySchedule map[string]map[int64]*scheduleModel.StudentArrivalSchedule, dateKey string, studentID int64) (*scheduleModel.StudentArrivalSchedule, bool) {
	if byStu, ok := bySchedule[dateKey]; ok {
		if sched, has := byStu[studentID]; has && sched != nil {
			return sched, true
		}
	}
	return nil, false
}

// -----------------------------------------------------------------------------
// Template start-time preload
// -----------------------------------------------------------------------------

type templatePreload struct {
	// byKey[{group, weekday}] — all template start_times for that pair, in
	// ascending order. A single element is unambiguous; multiple elements mean
	// the template runs multiple slots that weekday and we cannot pick the
	// "original" without more context.
	byKey map[groupWeekdayKey][]time.Time
}

type groupWeekdayKey struct {
	GroupID int64
	Weekday int
}

func (s *TimetableDataService) loadTemplatePreload(
	ctx context.Context,
	affected []exceptionInstancePair,
) (*templatePreload, error) {
	// We only need template start-times for modified exceptions that carry a
	// new start_time. Room-only modifies don't trigger Type 2 and cancellations
	// don't need the "original" field at all.
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

	rows, err := s.deps.ActivityScheduleRepo.FindTemplateStartTimesByGroupIDs(ctx, slices.Collect(maps.Keys(needed)))
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		k := groupWeekdayKey{GroupID: row.ActivityGroupID, Weekday: row.Weekday}
		pre.byKey[k] = append(pre.byKey[k], row.StartTime)
	}
	return pre, nil
}

// resolveOriginalStart picks the unambiguous template start time for the
// (group, weekday) pair. If the template defines more than one schedule that
// weekday, we cannot tell which was modified — the warning is still emitted but
// original_start_time is left empty.
func (pre *templatePreload) resolveOriginalStart(groupID int64, weekday int, logger *slog.Logger, excDate timezone.Date) (string, bool) {
	starts, ok := pre.byKey[groupWeekdayKey{GroupID: groupID, Weekday: weekday}]
	if !ok || len(starts) == 0 {
		logger.Warn("modified exception but no template schedule for weekday",
			slog.Int64("activity_group_id", groupID),
			slog.Int("weekday", weekday),
			slog.String("exception_date", timetableDateKey(excDate)),
		)
		return "", false
	}
	if len(starts) > 1 {
		logger.Warn("multiple template schedules for weekday, cannot disambiguate original_start_time",
			slog.Int64("activity_group_id", groupID),
			slog.Int("weekday", weekday),
			slog.Int("schedule_count", len(starts)),
			slog.String("exception_date", timetableDateKey(excDate)),
		)
		return "", false
	}
	return starts[0].Format("15:04"), true
}

// -----------------------------------------------------------------------------
// Build conflicts
// -----------------------------------------------------------------------------

func buildConflicts(
	affected []exceptionInstancePair,
	expectedByInstance map[int64][]*scheduleModel.InstanceStudent,
	arrivalPre *arrivalPreload,
	templatePre *templatePreload,
	logger *slog.Logger,
) []ExceptionConflict {
	maxWarnings := 0
	for _, a := range affected {
		maxWarnings += len(expectedByInstance[a.instance.ID])
	}
	out := make([]ExceptionConflict, 0, maxWarnings)
	for _, a := range affected {
		for _, is := range expectedByInstance[a.instance.ID] {
			if entry, emit := conflictForStudent(a, is, arrivalPre, templatePre, logger); emit {
				out = append(out, entry)
			}
		}
	}
	return out
}

func conflictForStudent(
	a exceptionInstancePair,
	is *scheduleModel.InstanceStudent,
	arrivalPre *arrivalPreload,
	templatePre *templatePreload,
	logger *slog.Logger,
) (ExceptionConflict, bool) {
	arrivalTime, arrivalSource := arrivalPre.resolveArrival(is.StudentID, a.exception.ExceptionDate)

	// Explicit-absence handling: if the arrival exception says the student is
	// absent on that date (ExpectedArrival=nil → source=exception with zero
	// time), both conflict kinds are noise — the child isn't coming, so
	// cancelling or moving the activity has no impact on them.
	if arrivalSource == SlotSourceException && arrivalTime.IsZero() {
		return ExceptionConflict{}, false
	}

	base := ExceptionConflict{
		Date:            timetableDateKey(a.exception.ExceptionDate),
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
			return ExceptionConflict{}, false // room-only modify — no time mismatch possible
		}
		if arrivalSource == SlotSourceNone {
			return ExceptionConflict{}, false // no arrival info to compare against
		}
		// exception.start_time and the arrival columns are both TIME values but
		// bun deserialises them with different year anchors. WallClock on both
		// sides pins them to year 0001 UTC so the comparison is purely HH:MM:SS.
		modifiedStart := timezone.WallClock(*a.exception.StartTime)
		arrivalNorm := timezone.WallClock(arrivalTime)

		// Only emit when arrival is strictly after the new start — equal means
		// the student arrives exactly at the start, which is not a conflict.
		if !arrivalNorm.After(modifiedStart) {
			return ExceptionConflict{}, false
		}

		base.Kind = ConflictKindModifiedMismatch
		base.ModifiedStartTime = modifiedStart.Format("15:04")
		if original, ok := templatePre.resolveOriginalStart(
			a.exception.ActivityGroupID,
			isoWeekday(a.exception.ExceptionDate),
			logger,
			a.exception.ExceptionDate,
		); ok {
			base.OriginalStartTime = original
		}
		return base, true
	}

	// Unknown exception type: log and skip. Model validation rejects unknown
	// types on write, so this defends against future additions.
	logger.Warn("unknown exception type, skipping",
		slog.Int64("activity_group_id", a.exception.ActivityGroupID),
		slog.String("exception_type", a.exception.ExceptionType),
	)
	return ExceptionConflict{}, false
}
