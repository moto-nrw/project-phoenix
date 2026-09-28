package compose

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Exception conflicts (WP-B13): two classes of planning conflict between an
// activity exception and a student's arrival expectation. Read-only; every
// kind of row is loaded in one batched read (per date for the arrival
// exceptions, which the read range cap bounds), never per student.

var errArrivalBaselineMissing = errors.New("load arrival schedules: baseline projection is not configured")

// DetectExceptionConflicts computes the cancelled_instance_with_scheduled_arrivals
// and modified_instance_time_mismatch conflicts in the inclusive [from, to]
// window and returns them unsorted (the caller sorts before responding).
func (d *conflictDetection) DetectExceptionConflicts(ctx context.Context, from, to timezone.Date) ([]timetable.ExceptionConflict, error) {
	exceptions, err := d.deps.Exceptions.FindByDateRange(ctx, scheduleModels.Date(from), scheduleModels.Date(to))
	if err != nil {
		return nil, err
	}
	if len(exceptions) == 0 {
		return []timetable.ExceptionConflict{}, nil
	}
	instances, err := d.deps.Instances.FindByTenantAndDateRange(ctx, scheduleModels.Date(from), scheduleModels.Date(to))
	if err != nil {
		return nil, err
	}
	affected := d.buildAffectedPairs(exceptions, instances)
	if len(affected) == 0 {
		return []timetable.ExceptionConflict{}, nil
	}
	expectedStudents, err := d.deps.InstanceStudents.FindExpectedByInstanceIDs(ctx, uniqueAffectedInstanceIDs(affected))
	if err != nil {
		return nil, err
	}
	if len(expectedStudents) == 0 {
		return []timetable.ExceptionConflict{}, nil
	}
	expectedByInstance := make(map[int64][]*scheduleModels.InstanceStudent, len(expectedStudents))
	for _, row := range expectedStudents {
		expectedByInstance[row.InstanceID] = append(expectedByInstance[row.InstanceID], row)
	}
	arrivals, err := d.loadArrivalPreload(ctx, datesToStudents(affected, expectedByInstance))
	if err != nil {
		return nil, err
	}
	templates, err := d.loadTemplatePreload(ctx, affected)
	if err != nil {
		return nil, err
	}
	return d.buildExceptionConflicts(affected, expectedByInstance, arrivals, templates), nil
}

type groupDateKey struct {
	GroupID int64
	Date    timezone.Date
}

type exceptionInstancePair struct {
	instance  *scheduleModels.ActivityInstance
	exception *scheduleModels.ActivityException
}

func (p exceptionInstancePair) date() timezone.Date {
	return timezone.Date(p.exception.ExceptionDate)
}

// buildAffectedPairs joins exceptions to their materialized blocks. An
// exception with no matching block (created before the scheduler next runs)
// is skipped with a debug log — materialization and exception creation are
// independent flows.
func (d *conflictDetection) buildAffectedPairs(
	exceptions []*scheduleModels.ActivityException,
	instances []*scheduleModels.ActivityInstance,
) []exceptionInstancePair {
	instancesByKey := indexInstancesByGroupDate(instances)
	affected := make([]exceptionInstancePair, 0, len(exceptions))
	for _, exception := range exceptions {
		key := groupDateKey{GroupID: exception.ActivityGroupID, Date: timezone.Date(exception.ExceptionDate)}
		matching := instancesByKey[key]
		if len(matching) == 0 {
			d.logger.Debug("exception without matching instance, skipping",
				slog.Int64("activity_group_id", exception.ActivityGroupID),
				slog.String("exception_date", key.Date.String()),
				slog.String("exception_type", exception.ExceptionType),
			)
			continue
		}
		for _, instance := range matching {
			affected = append(affected, exceptionInstancePair{instance: instance, exception: exception})
		}
	}
	return affected
}

func indexInstancesByGroupDate(instances []*scheduleModels.ActivityInstance) map[groupDateKey][]*scheduleModels.ActivityInstance {
	out := make(map[groupDateKey][]*scheduleModels.ActivityInstance, len(instances))
	for _, instance := range instances {
		if instance.ActivityGroupID == nil {
			continue // spontaneous — no template, so never tied to an exception
		}
		key := groupDateKey{GroupID: *instance.ActivityGroupID, Date: timezone.Date(instance.Date)}
		out[key] = append(out[key], instance)
	}
	return out
}

func uniqueAffectedInstanceIDs(affected []exceptionInstancePair) []int64 {
	seen := make(map[int64]struct{}, len(affected))
	ids := make([]int64, 0, len(affected))
	for _, pair := range affected {
		if _, ok := seen[pair.instance.ID]; ok {
			continue
		}
		seen[pair.instance.ID] = struct{}{}
		ids = append(ids, pair.instance.ID)
	}
	return ids
}

// datesToStudents collects the (date → student set) tuples that still need
// arrival information.
func datesToStudents(
	affected []exceptionInstancePair,
	expectedByInstance map[int64][]*scheduleModels.InstanceStudent,
) map[timezone.Date]map[int64]struct{} {
	out := make(map[timezone.Date]map[int64]struct{})
	for _, pair := range affected {
		students, ok := expectedByInstance[pair.instance.ID]
		if !ok {
			continue
		}
		date := pair.date()
		if out[date] == nil {
			out[date] = make(map[int64]struct{})
		}
		for _, student := range students {
			out[date][student.StudentID] = struct{}{}
		}
	}
	return out
}

type arrivalPreload struct {
	// byException[date][studentID] — a non-nil row means an arrival exception
	// exists for the student on that date (ExpectedArrival may still be nil
	// inside the row, which flags an explicit absence).
	byException map[timezone.Date]map[int64]*scheduleModels.StudentArrivalException
	// bySchedule[date][studentID] — the recurring arrival time applicable on
	// that date. Keyed by date rather than weekday because whether a weekday
	// is a care day at all can change within the window once the approved
	// bookings decide it (#2414, ADR 0005).
	bySchedule map[timezone.Date]map[int64]time.Time
}

func (d *conflictDetection) loadArrivalPreload(ctx context.Context, dates map[timezone.Date]map[int64]struct{}) (*arrivalPreload, error) {
	pre := &arrivalPreload{
		byException: map[timezone.Date]map[int64]*scheduleModels.StudentArrivalException{},
		bySchedule:  map[timezone.Date]map[int64]time.Time{},
	}
	if err := d.loadArrivalExceptions(ctx, pre, dates); err != nil {
		return nil, err
	}
	if err := d.fillArrivalSchedules(ctx, pre, pre.withoutException(dates)); err != nil {
		return nil, err
	}
	return pre, nil
}

// loadArrivalExceptions issues one query per unique date. The range is capped
// at MaxTimetableReadRangeDays upstream, so this is bounded.
func (d *conflictDetection) loadArrivalExceptions(ctx context.Context, pre *arrivalPreload, dates map[timezone.Date]map[int64]struct{}) error {
	for date, students := range dates {
		rows, err := d.deps.ArrivalExceptions.FindByStudentIDsAndDate(ctx, slices.Collect(maps.Keys(students)), scheduleModels.Date(date))
		if err != nil {
			return err
		}
		byStudent := make(map[int64]*scheduleModels.StudentArrivalException, len(rows))
		for _, row := range rows {
			byStudent[row.StudentID] = row
		}
		pre.byException[date] = byStudent
	}
	return nil
}

// withoutException keeps the weekday students without an arrival exception
// on the date: exceptions win, and only Monday to Friday carries a weekly
// arrival.
func (pre *arrivalPreload) withoutException(dates map[timezone.Date]map[int64]struct{}) map[timezone.Date]map[int64]struct{} {
	needed := make(map[timezone.Date]map[int64]struct{})
	for date, students := range dates {
		if weekday := isoWeekday(date); weekday < scheduleModels.WeekdayMonday || weekday > scheduleModels.WeekdayFriday {
			continue
		}
		for studentID := range students {
			if _, hasException := pre.byException[date][studentID]; hasException {
				continue
			}
			if needed[date] == nil {
				needed[date] = make(map[int64]struct{})
			}
			needed[date][studentID] = struct{}{}
		}
	}
	return needed
}

// fillArrivalSchedules resolves the recurring arrival per (date, student)
// through Care Plan's baseline projection (#2414): the class timetable
// supplies the time, and with enrollment.bookings_authoritative on the
// approved bookings supply the care days — so a stale row on an unbooked
// weekday no longer produces a warning about a child that is not coming.
func (d *conflictDetection) fillArrivalSchedules(ctx context.Context, pre *arrivalPreload, needed map[timezone.Date]map[int64]struct{}) error {
	if len(needed) == 0 {
		return nil
	}
	if d.deps.ArrivalBaselines == nil {
		return errArrivalBaselineMissing
	}
	studentIDs, from, to := neededArrivalWindow(needed)
	projection, err := d.deps.ArrivalBaselines.ProjectArrivals(ctx, studentIDs, from, to)
	if err != nil {
		return err
	}
	for date, students := range needed {
		byStudent := make(map[int64]time.Time, len(students))
		for studentID := range students {
			if arrival, ok := projection.ExpectedArrival(studentID, date); ok {
				byStudent[studentID] = arrival
			}
		}
		pre.bySchedule[date] = byStudent
	}
	return nil
}

// neededArrivalWindow returns the students and the smallest inclusive window
// covering every needed date.
func neededArrivalWindow(needed map[timezone.Date]map[int64]struct{}) ([]int64, timezone.Date, timezone.Date) {
	studentIDs := make(map[int64]struct{})
	var from, to timezone.Date
	for date, students := range needed {
		if from.IsZero() || date.Before(from) {
			from = date
		}
		if to.IsZero() || date.After(to) {
			to = date
		}
		for studentID := range students {
			studentIDs[studentID] = struct{}{}
		}
	}
	return slices.Collect(maps.Keys(studentIDs)), from, to
}

// resolveArrival returns (arrival time, source) for a student on a date using
// the shared exception-over-schedule precedence (timetable.ResolveSlotSource).
// The time is zero when unknown — the source distinguishes "none" (no rule)
// from an exception without expected arrival (explicit absence).
func (pre *arrivalPreload) resolveArrival(studentID int64, date timezone.Date) (time.Time, string) {
	exception, hasException := pre.byException[date][studentID]
	hasException = hasException && exception != nil
	scheduled, hasSchedule := pre.bySchedule[date][studentID]
	switch timetable.ResolveSlotSource(hasException, hasSchedule, isoWeekday(date)) {
	case timetable.SlotSourceException:
		if exception.ExpectedArrival != nil {
			return *exception.ExpectedArrival, timetable.SlotSourceException
		}
		return time.Time{}, timetable.SlotSourceException // explicit absence
	case timetable.SlotSourceSchedule:
		return scheduled, timetable.SlotSourceSchedule
	default:
		return time.Time{}, timetable.SlotSourceNone
	}
}

type groupWeekdayKey struct {
	GroupID int64
	Weekday int
}

// templatePreload holds all template start times per (group, weekday) in
// ascending order. A single element is unambiguous; several mean the template
// runs several slots that weekday and the "original" cannot be picked
// without more context.
type templatePreload struct {
	byKey map[groupWeekdayKey][]time.Time
}

// loadTemplatePreload reads the template start times only for modified
// exceptions that carry a new start time. Room-only modifications cannot
// mismatch an arrival and cancellations need no original start.
func (d *conflictDetection) loadTemplatePreload(ctx context.Context, affected []exceptionInstancePair) (*templatePreload, error) {
	needed := make(map[int64]struct{})
	for _, pair := range affected {
		if pair.exception.ExceptionType == scheduleModels.ActivityExceptionModified && pair.exception.StartTime != nil {
			needed[pair.exception.ActivityGroupID] = struct{}{}
		}
	}
	pre := &templatePreload{byKey: map[groupWeekdayKey][]time.Time{}}
	if len(needed) == 0 {
		return pre, nil
	}
	rows, err := d.deps.Schedules.FindTemplateStartTimesByGroupIDs(ctx, slices.Collect(maps.Keys(needed)))
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		key := groupWeekdayKey{GroupID: row.ActivityGroupID, Weekday: row.Weekday}
		pre.byKey[key] = append(pre.byKey[key], row.StartTime)
	}
	return pre, nil
}

// resolveOriginalStart picks the unambiguous template start time for the
// (group, weekday) pair. With more than one schedule that weekday the warning
// is still emitted, but original_start_time stays empty.
func (pre *templatePreload) resolveOriginalStart(groupID int64, date timezone.Date, logger *slog.Logger) (string, bool) {
	weekday := isoWeekday(date)
	starts, ok := pre.byKey[groupWeekdayKey{GroupID: groupID, Weekday: weekday}]
	if !ok || len(starts) == 0 {
		logger.Warn("modified exception but no template schedule for weekday",
			slog.Int64("activity_group_id", groupID),
			slog.Int("weekday", weekday),
			slog.String("exception_date", date.String()),
		)
		return "", false
	}
	if len(starts) > 1 {
		logger.Warn("multiple template schedules for weekday, cannot disambiguate original_start_time",
			slog.Int64("activity_group_id", groupID),
			slog.Int("weekday", weekday),
			slog.Int("schedule_count", len(starts)),
			slog.String("exception_date", date.String()),
		)
		return "", false
	}
	return starts[0].Format("15:04"), true
}

func (d *conflictDetection) buildExceptionConflicts(
	affected []exceptionInstancePair,
	expectedByInstance map[int64][]*scheduleModels.InstanceStudent,
	arrivals *arrivalPreload,
	templates *templatePreload,
) []timetable.ExceptionConflict {
	maxWarnings := 0
	for _, pair := range affected {
		maxWarnings += len(expectedByInstance[pair.instance.ID])
	}
	out := make([]timetable.ExceptionConflict, 0, maxWarnings)
	for _, pair := range affected {
		for _, student := range expectedByInstance[pair.instance.ID] {
			if entry, emit := d.conflictForStudent(pair, student.StudentID, arrivals, templates); emit {
				out = append(out, entry)
			}
		}
	}
	return out
}

func (d *conflictDetection) conflictForStudent(
	pair exceptionInstancePair,
	studentID int64,
	arrivals *arrivalPreload,
	templates *templatePreload,
) (timetable.ExceptionConflict, bool) {
	arrival, source := arrivals.resolveArrival(studentID, pair.date())
	// Explicit absence: an arrival exception without expected arrival means
	// the child is not coming, so cancelling or moving the block has no
	// impact on them.
	if source == timetable.SlotSourceException && arrival.IsZero() {
		return timetable.ExceptionConflict{}, false
	}
	base := timetable.ExceptionConflict{
		Date:            pair.date().String(),
		ActivityGroupID: pair.exception.ActivityGroupID,
		InstanceID:      pair.instance.ID,
		ActivityTitle:   pair.instance.Title,
		StudentID:       studentID,
		ArrivalSource:   source,
	}
	if !arrival.IsZero() {
		base.ExpectedArrival = arrival.Format("15:04")
	}
	switch pair.exception.ExceptionType {
	case scheduleModels.ActivityExceptionCancelled:
		base.Kind = timetable.ConflictKindCancelledArrivals
		if reason := pair.exception.Reason; reason != nil && *reason != "" {
			base.CancellationReason = *reason
		}
		return base, true
	case scheduleModels.ActivityExceptionModified:
		return d.modifiedMismatch(base, pair, arrival, source, templates)
	}
	// Unknown exception type: log and skip. Model validation rejects unknown
	// types on write, so this defends against future additions.
	d.logger.Warn("unknown exception type, skipping",
		slog.Int64("activity_group_id", pair.exception.ActivityGroupID),
		slog.String("exception_type", pair.exception.ExceptionType),
	)
	return timetable.ExceptionConflict{}, false
}

// modifiedMismatch emits a conflict when a modified exception moves the start
// before the student's resolved arrival.
func (d *conflictDetection) modifiedMismatch(
	base timetable.ExceptionConflict,
	pair exceptionInstancePair,
	arrival time.Time,
	source string,
	templates *templatePreload,
) (timetable.ExceptionConflict, bool) {
	if pair.exception.StartTime == nil {
		return timetable.ExceptionConflict{}, false // room-only modify — no time mismatch possible
	}
	if source == timetable.SlotSourceNone {
		return timetable.ExceptionConflict{}, false // no arrival info to compare against
	}
	// The exception start and the arrival are both TIME values that scan with
	// different date anchors; normalizing both pins the comparison to
	// HH:MM:SS.
	modifiedStart := timezone.NormalizeWallClock(*pair.exception.StartTime)
	// Only emit when the arrival is strictly after the new start — arriving
	// exactly at the start is not a conflict.
	if !timezone.NormalizeWallClock(arrival).After(modifiedStart) {
		return timetable.ExceptionConflict{}, false
	}
	base.Kind = timetable.ConflictKindModifiedMismatch
	base.ModifiedStartTime = modifiedStart.Format("15:04")
	if original, ok := templates.resolveOriginalStart(pair.exception.ActivityGroupID, pair.date(), d.logger); ok {
		base.OriginalStartTime = original
	}
	return base, true
}
