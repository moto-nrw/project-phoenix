package planning

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/realtime"
)

var (
	// ErrSeriesNotFound signals that the requested shift series does not exist.
	ErrSeriesNotFound = errors.New("shift series not found")
	// ErrSeriesInvalid wraps series validation failures (maps to 400).
	ErrSeriesInvalid = errors.New("invalid shift series")
)

// SeriesResult reports what a series create or split materialized. Skipped
// dates are occurrences the series left out because an existing shift of the
// same staff member would overlap there (collision rule: skip + report,
// existing single shifts stay untouched).
type SeriesResult struct {
	Series       *StaffShiftSeries
	OldSeriesID  int64
	Created      int
	Deleted      int64
	SkippedDates []timezone.Date
	// SkippedNonWorkingDays counts the occurrences left out because they fell
	// on a statutory holiday, a Ferien day or a closing day (#3820).
	SkippedNonWorkingDays int
}

// SplitSeriesInput carries the edited field set applied from the effective
// date onward ("Ab jetzt dauerhaft"). The calendar period stays with the
// series lineage — a split changes the rule, not the planning period.
// Optional fields (nil) inherit the predecessor's value, so a client editing
// only times never has to know or re-send the series' rhythm.
type SplitSeriesInput struct {
	SeriesID      int64
	EffectiveDate timezone.Date
	// OccurrenceShiftID identifies the concrete occurrence opened by the
	// planner. When EffectiveDate is today, this row is updated in place before
	// the series is split from tomorrow, so its identity and audit references
	// survive the permanent rule change.
	OccurrenceShiftID int64
	Weekdays          []int16 // nil = keep predecessor weekdays
	StartTime         time.Time
	EndTime           time.Time
	BreakMinutes      int
	ShiftTypeID       *int64
	// ShiftTypeIDSet distinguishes "clear the type" (true, nil value) from
	// "keep the predecessor's type" (false) — same tri-state as the single
	// shift update payload.
	ShiftTypeIDSet bool
	Notes          *string // nil = keep predecessor notes
	WeekPattern    *int    // nil = keep predecessor pattern
	// ValidUntil (exclusive) ends the successor earlier or later than the
	// predecessor did. ValidUntilSet distinguishes "run until the period ends"
	// (true, nil value) from "keep the predecessor's end" (false), so editing
	// only the weekdays never silently drops a stored end date (#2028).
	ValidUntil    *timezone.Date
	ValidUntilSet bool
	// IncludeSchoolBreaks nil keeps the predecessor's opt-in into Ferien and
	// closing days (#3820).
	IncludeSchoolBreaks *bool
	ActorStaffID        int64
}

// StaffShiftSeriesService manages recurring shift series (#1889). Series
// materialize concrete StaffShift rows upfront over the calendar period —
// every existing reader keeps working on concrete rows only.
type StaffShiftSeriesService interface {
	// CreateSeries validates the series, persists it, and materializes its
	// concrete shifts from max(valid_from, tomorrow). Past and current dates are
	// never changed.
	CreateSeries(ctx context.Context, series *StaffShiftSeries) (*SeriesResult, error)
	// SplitSeries caps the series at the effective date and creates a
	// successor with the edited fields ("Ab jetzt dauerhaft"). Detached rows
	// and exceptions from the effective date onward move to the successor.
	SplitSeries(ctx context.Context, input SplitSeriesInput) (*SeriesResult, error)
	// EndSeries caps the series at the given date and deletes its
	// non-detached rows from that date onward.
	EndSeries(ctx context.Context, seriesID int64, from timezone.Date) (*SeriesResult, error)
	// GetSeries returns one series segment (the rule behind a shift), so the
	// planner can edit weekdays, rhythm, and validity instead of only the
	// window of a single occurrence (#2028). The edit itself goes through
	// SplitSeries — it applies from the opened occurrence onwards.
	GetSeries(ctx context.Context, seriesID int64) (*StaffShiftSeries, error)
}

type staffShiftSeriesService struct {
	seriesRepo    staffShiftSeriesRows
	exceptionRepo staffShiftSeriesExceptionRows
	shiftRepo     staffShiftRows
	staffRepo     staffDirectory
	periodRepo    SeriesPeriodLookup
	shiftTypes    ShiftTypeService
	shiftService  StaffShiftService
	// lockStaffShifts is the per-staff write lock shared with single-shift
	// CRUD. Nil skips the lock (unit tests without a database).
	lockStaffShifts func(ctx context.Context, staffID int64) error
	broadcaster     realtime.Broadcaster
	logger          *slog.Logger
	today           func() timezone.Date
	// nonWorkingDays is the School Calendar read behind the holiday, Ferien
	// and closing-day skips (#3820). Nil skips none.
	nonWorkingDays SeriesNonWorkingDayReader
}

// NewStaffShiftSeriesService creates a new staff shift series service.
// lockStaffShifts is the per-staff write lock shared with single-shift CRUD.
func NewStaffShiftSeriesService(
	seriesRepo staffShiftSeriesRows,
	exceptionRepo staffShiftSeriesExceptionRows,
	shiftRepo staffShiftRows,
	staffRepo staffDirectory,
	periodRepo SeriesPeriodLookup,
	shiftTypes ShiftTypeService,
	lockStaffShifts func(ctx context.Context, staffID int64) error,
	logger *slog.Logger,
	shiftService StaffShiftService,
	opts ...StaffShiftSeriesOption,
) StaffShiftSeriesService {
	service := &staffShiftSeriesService{
		seriesRepo:      seriesRepo,
		exceptionRepo:   exceptionRepo,
		shiftRepo:       shiftRepo,
		staffRepo:       staffRepo,
		periodRepo:      periodRepo,
		shiftTypes:      shiftTypes,
		shiftService:    shiftService,
		lockStaffShifts: lockStaffShifts,
		logger:          logger,
		today:           timezone.TodayDate,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

// StaffShiftSeriesOption binds an optional collaborator at construction time
// (#3219; see StaffShiftOption).
type StaffShiftSeriesOption func(*staffShiftSeriesService)

// WithStaffShiftSeriesToday overrides the calendar-day clock the
// materializer and the replans compare against. A nil clock keeps the
// default.
func WithStaffShiftSeriesToday(today func() timezone.Date) StaffShiftSeriesOption {
	return func(s *staffShiftSeriesService) {
		if today != nil {
			s.today = today
		}
	}
}

// WithStaffShiftSeriesBroadcaster injects the tenant-wide SSE broadcaster
// used to invalidate time-tracking views after committed series writes.
func WithStaffShiftSeriesBroadcaster(broadcaster realtime.Broadcaster) StaffShiftSeriesOption {
	return func(s *staffShiftSeriesService) { s.broadcaster = broadcaster }
}

func (s *staffShiftSeriesService) todayDate() timezone.Date {
	return s.today()
}

// updateTodayOccurrence keeps a currently planned series occurrence as the
// same concrete row while applying a permanent rule change. UpdateShift marks
// it detached, so the successor re-plan beginning tomorrow cannot replace it.
// Existing deviations, including rows moved away from today, remain untouched
// for the split to preserve and re-point. RetainedOccurrenceShiftID explicitly
// identifies the row retained by an earlier same-day permanent edit; Detached
// alone is also used for independent one-off deviations.
// The caller already runs inside the request's tenant transaction.
func (s *staffShiftSeriesService) updateTodayOccurrence(
	ctx context.Context,
	input SplitSeriesInput,
	updateRetainedOccurrence bool,
) (bool, error) {
	if input.OccurrenceShiftID <= 0 {
		return false, nil
	}
	if s.shiftService == nil {
		return false, fmt.Errorf("%w: concrete occurrence updater is not configured", ErrSeriesInvalid)
	}

	occurrence, err := s.shiftRepo.FindByID(ctx, input.OccurrenceShiftID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return false, fmt.Errorf("%w: current series occurrence not found", ErrSeriesInvalid)
		}
		return false, fmt.Errorf("find current series occurrence: %w", err)
	}
	if occurrence == nil || occurrence.SeriesID == nil || *occurrence.SeriesID != input.SeriesID ||
		occurrence.SeriesOccurrenceDate == nil || *occurrence.SeriesOccurrenceDate != input.EffectiveDate {
		return false, fmt.Errorf("%w: current occurrence does not belong to this series date", ErrSeriesInvalid)
	}
	// A cancellation (and its replacement coverage) is a deliberate current-day
	// deviation. Resizing it through UpdateShift can invalidate its covers, so
	// retain the cancellation and apply the permanent rule only from tomorrow.
	if occurrence.Date != s.todayDate() || occurrence.Cancelled ||
		(occurrence.Detached && !updateRetainedOccurrence) {
		return false, nil
	}

	updated := *occurrence
	updated.StartTime = input.StartTime
	updated.EndTime = input.EndTime
	updated.BreakMinutes = input.BreakMinutes
	if input.ShiftTypeIDSet {
		updated.ShiftTypeID = input.ShiftTypeID
	}
	if input.Notes != nil {
		updated.Notes = *input.Notes
	}
	if input.ActorStaffID > 0 {
		updated.UpdatedBy = &input.ActorStaffID
	}
	if _, err := s.shiftService.UpdateShiftWithOptions(ctx, &updated, StaffShiftUpdateOptions{
		SuppressTimeTrackingBroadcast: true,
	}); err != nil {
		return false, fmt.Errorf("update current series occurrence: %w", err)
	}
	return true, nil
}

func (s *staffShiftSeriesService) getLogger() *slog.Logger {
	return loggerOrDefault(s.logger)
}

func (s *staffShiftSeriesService) broadcastTimeTrackingChanged(ctx context.Context) {
	realtimeevents.QueueStaffTimeTrackingChanged(ctx, s.broadcaster, s.getLogger())
}

func (s *staffShiftSeriesService) lockShiftWrites(ctx context.Context, staffID int64) error {
	if s.lockStaffShifts == nil {
		return nil
	}
	return s.lockStaffShifts(ctx, staffID)
}

// weekCycle is the period's A/B alternation in the School Calendar's
// week-pattern engine.
func (p *SeriesPeriod) weekCycle() schoolcalendar.WeekCycle {
	return schoolcalendar.WeekCycle{Length: p.WeekCycleLength, Anchor: p.WeekCycleAnchor}
}

// loadPeriodForSeries loads the series' calendar period and enforces the
// period-dependent rules: validity within the period and week A/B only when
// the period actually alternates.
func (s *staffShiftSeriesService) loadPeriodForSeries(ctx context.Context, series *StaffShiftSeries) (*SeriesPeriod, error) {
	found, err := s.periodRepo.FindSeriesPeriod(ctx, series.CalendarPeriodID)
	if err != nil {
		if modelBase.IsNoRows(err) || errors.Is(err, schoolcalendar.ErrCalendarPeriodNotFound) {
			return nil, fmt.Errorf("%w: calendar period not found", ErrSeriesInvalid)
		}
		return nil, fmt.Errorf("find calendar period for series: %w", err)
	}
	period := &found
	if series.WeekPattern != WeekPatternEvery && period.WeekCycleLength <= 1 {
		return nil, withReason(fmt.Errorf("%w: week A/B requires a calendar period with a week cycle", ErrSeriesInvalid), workforce.ErrShiftSeriesWeekCycleMissing)
	}
	if series.ValidFrom.Before(period.StartDate) || series.ValidFrom.After(period.EndDate) {
		return nil, withReason(fmt.Errorf("%w: valid from must lie within the calendar period", ErrSeriesInvalid), workforce.ErrShiftSeriesOutsidePeriod)
	}
	if series.ValidUntil != nil && series.ValidUntil.After(period.EndDate.AddDays(1)) {
		return nil, withReason(fmt.Errorf("%w: valid until must not exceed the calendar period", ErrSeriesInvalid), workforce.ErrShiftSeriesOutsidePeriod)
	}
	return period, nil
}

// seriesMaterialization reports one materialization run: the created rows,
// the dates skipped because an existing shift overlaps, and how many
// occurrences fell on a holiday, a Ferien day or a closing day (#3820).
type seriesMaterialization struct {
	created           int
	skipped           []timezone.Date
	nonWorkingSkipped int
}

// materializeSeries generates the series' concrete shifts between
// max(valid_from, period start, tomorrow) and min(valid_until-1, period end).
// Dates with an exception are skipped; so are statutory holidays, and Ferien
// and closing days unless the series includes them (#3820). Dates where the
// generated shift would overlap ANY existing shift of the staff member
// (standalone, detached, or other series) are skipped and reported.
func (s *staffShiftSeriesService) materializeSeries(ctx context.Context, series *StaffShiftSeries, period *SeriesPeriod) (seriesMaterialization, error) {
	var result seriesMaterialization
	from, to := seriesWindow(series, period, s.todayDate())
	if to.Before(from) {
		return result, nil
	}

	exceptionDates, err := s.exceptionRepo.FindDatesBySeriesID(ctx, series.ID)
	if err != nil {
		return result, err
	}
	excepted := make(map[timezone.Date]bool, len(exceptionDates))
	for _, d := range exceptionDates {
		excepted[d] = true
	}
	existing, err := s.shiftRepo.FindByStaffAndDateRange(ctx, series.StaffID, from, to)
	if err != nil {
		return result, err
	}
	existingByDate, ownedDates := indexSeriesShifts(existing, series.ID)
	nonWorking, err := s.loadNonWorkingDays(ctx, series, from, to)
	if err != nil {
		return result, err
	}

	var candidates []*StaffShift
	for d := from; !d.After(to); d = d.AddDays(1) {
		if excepted[d] || ownedDates[d] || !seriesOccursOn(series, period, d) {
			continue
		}
		if nonWorking.skips(d, series.IncludeSchoolBreaks) {
			result.nonWorkingSkipped++
			continue
		}
		candidate := seriesCandidate(series, d)
		if overlapsActiveShift(candidate, existingByDate[d]) {
			result.skipped = append(result.skipped, d)
			continue
		}
		candidates = append(candidates, candidate)
	}

	if err := s.shiftRepo.BulkCreate(ctx, candidates); err != nil {
		return result, err
	}
	result.created = len(candidates)
	return result, nil
}

// seriesWindow is the inclusive range a series materializes over:
// max(valid_from, period start, tomorrow) to min(valid_until-1, period end).
// An empty window has to before from.
func seriesWindow(series *StaffShiftSeries, period *SeriesPeriod, today timezone.Date) (timezone.Date, timezone.Date) {
	from := series.ValidFrom
	if period.StartDate.After(from) {
		from = period.StartDate
	}
	tomorrow := today.AddDays(1)
	if tomorrow.After(from) {
		from = tomorrow
	}
	to := period.EndDate
	if series.ValidUntil != nil {
		lastIncluded := series.ValidUntil.AddDays(-1) // valid_until is exclusive
		if lastIncluded.Before(to) {
			to = lastIncluded
		}
	}
	return from, to
}

// seriesOccursOn applies the recurrence itself: weekday and week A/B.
func seriesOccursOn(series *StaffShiftSeries, period *SeriesPeriod, d timezone.Date) bool {
	return series.ContainsWeekday(isoWeekday(d)) &&
		schoolcalendar.WeekPatternApplies(series.WeekPattern, d.String(), period.weekCycle())
}

// indexSeriesShifts groups the staff member's existing shifts by date and
// collects the recurrence slots that already have a row of THIS series
// (detached survivors of a re-plan, or split re-points). Those slots are
// owned: the user deviated that occurrence, so the series must not add a
// second shift there. A moved row's current Date may be another genuine
// occurrence; ownership follows the immutable source slot instead.
func indexSeriesShifts(existing []*StaffShift, seriesID int64) (map[timezone.Date][]*StaffShift, map[timezone.Date]bool) {
	byDate := make(map[timezone.Date][]*StaffShift, len(existing))
	owned := make(map[timezone.Date]bool)
	for _, shift := range existing {
		byDate[shift.Date] = append(byDate[shift.Date], shift)
		if shift.SeriesID != nil && *shift.SeriesID == seriesID {
			owned[seriesOccurrenceDate(shift)] = true
		}
	}
	return byDate, owned
}

func seriesCandidate(series *StaffShiftSeries, d timezone.Date) *StaffShift {
	seriesID := series.ID
	occurrenceDate := d
	return &StaffShift{
		StaffID:              series.StaffID,
		Date:                 d,
		StartTime:            timezone.NormalizeWallClock(series.StartTime),
		EndTime:              timezone.NormalizeWallClock(series.EndTime),
		BreakMinutes:         series.BreakMinutes,
		ShiftTypeID:          series.ShiftTypeID,
		Notes:                series.Notes,
		SeriesID:             &seriesID,
		SeriesOccurrenceDate: &occurrenceDate,
		CreatedBy:            series.CreatedBy,
	}
}

// overlapsActiveShift reports whether the candidate collides with a shift
// that takes place. A cancelled shift neither blocks nor is blocked by the
// materialized candidate — the freed window is available. This matches the
// single-shift overlap rule (checkOverlap) and the partial unique index, both
// of which treat a cancelled row as a free window (#1841). Own detached
// cancellations are handled earlier via the owned dates, which skip the date
// outright.
func overlapsActiveShift(candidate *StaffShift, others []*StaffShift) bool {
	for _, other := range others {
		if !other.Cancelled && candidate.Overlaps(other) {
			return true
		}
	}
	return false
}

// hasFutureSeriesOccurrence checks the recurrence itself before a split mutates
// the predecessor. Exceptions and overlapping shifts intentionally do not
// count here: they are deviations of an otherwise valid recurring rule.
func hasFutureSeriesOccurrence(series *StaffShiftSeries, period *SeriesPeriod, today timezone.Date) bool {
	from, to := seriesWindow(series, period, today)
	for d := from; !d.After(to); d = d.AddDays(1) {
		if seriesOccursOn(series, period, d) {
			return true
		}
	}
	return false
}

func (s *staffShiftSeriesService) CreateSeries(ctx context.Context, series *StaffShiftSeries) (*SeriesResult, error) {
	if err := series.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSeriesInvalid, err.Error())
	}
	staff, err := s.staffRepo.FindByID(ctx, series.StaffID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, fmt.Errorf("%w: staff member not found", ErrSeriesInvalid)
		}
		return nil, fmt.Errorf("find staff member for series: %w", err)
	}
	if staff == nil {
		return nil, fmt.Errorf("%w: staff member not found", ErrSeriesInvalid)
	}
	if err := s.ensureShiftTypeActive(ctx, series.ShiftTypeID); err != nil {
		return nil, err
	}
	period, err := s.loadPeriodForSeries(ctx, series)
	if err != nil {
		return nil, err
	}
	if !hasFutureSeriesOccurrence(series, period, s.todayDate()) {
		return nil, withReason(fmt.Errorf(
			"%w: no occurrences left to create for the selected weekdays and week pattern",
			ErrSeriesInvalid,
		), workforce.ErrShiftSeriesNoOccurrences)
	}
	if err := s.lockShiftWrites(ctx, series.StaffID); err != nil {
		return nil, err
	}
	if err := s.seriesRepo.Create(ctx, series); err != nil {
		return nil, err
	}
	run, err := s.materializeSeries(ctx, series, period)
	if err != nil {
		return nil, err
	}
	s.getLogger().Info("staff shift series created",
		"series_id", series.ID,
		"staff_id", series.StaffID,
		"created", run.created,
		"skipped", len(run.skipped),
		"non_working_skipped", run.nonWorkingSkipped,
	)
	s.broadcastTimeTrackingChanged(ctx)
	return &SeriesResult{Series: series, Created: run.created, SkippedDates: run.skipped, SkippedNonWorkingDays: run.nonWorkingSkipped}, nil
}

// ensureShiftTypeActive mirrors the single-shift rule: a freshly assigned
// type must exist and be active; nil is an untyped series.
func (s *staffShiftSeriesService) ensureShiftTypeActive(ctx context.Context, id *int64) error {
	if id == nil || s.shiftTypes == nil {
		return nil
	}
	shiftType, err := s.shiftTypes.GetShiftType(ctx, *id)
	if err != nil {
		return err
	}
	if !shiftType.IsActive {
		return ErrShiftTypeInactive
	}
	return nil
}

func (s *staffShiftSeriesService) loadSeries(ctx context.Context, id int64) (*StaffShiftSeries, error) {
	if id <= 0 {
		return nil, ErrSeriesNotFound
	}
	series, err := s.seriesRepo.FindByID(ctx, id)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, ErrSeriesNotFound
		}
		return nil, fmt.Errorf("find staff shift series: %w", err)
	}
	if series == nil {
		return nil, ErrSeriesNotFound
	}
	return series, nil
}

func (s *staffShiftSeriesService) SplitSeries(ctx context.Context, input SplitSeriesInput) (*SeriesResult, error) {
	old, err := s.loadSeries(ctx, input.SeriesID)
	if err != nil {
		return nil, err
	}

	today := s.todayDate()
	updateToday := input.EffectiveDate == today
	effective := input.EffectiveDate
	tomorrow := today.AddDays(1)
	if tomorrow.After(effective) {
		effective = tomorrow
	}
	if effective.Before(old.ValidFrom) {
		effective = old.ValidFrom
	}
	// The successor is bounded by the EDITED end, not the predecessor's: an
	// editor that extends "Gültig bis" is deliberately re-opening a series whose
	// stored end already passed, and that is a legitimate edit. Checking the old
	// bound here made every series whose last day had arrived permanently
	// uneditable, because the effective date is clamped to tomorrow (#2028).
	validUntil := old.ValidUntil
	if input.ValidUntilSet {
		validUntil = nil
		if input.ValidUntil != nil {
			converted := *input.ValidUntil
			validUntil = &converted
		}
	}
	if validUntil != nil && !effective.Before(*validUntil) {
		return nil, withReason(fmt.Errorf(
			"%w: series ends before %s, no occurrences left to change",
			ErrSeriesInvalid, effective.String(),
		), workforce.ErrShiftSeriesNoOccurrences)
	}

	rootID := old.RootID()
	successor := successorSeries(old, input, rootID, effective, validUntil)
	successor.TenantID = old.TenantID
	if err := successor.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSeriesInvalid, err.Error())
	}
	if !sameShiftTypeID(successor.ShiftTypeID, old.ShiftTypeID) {
		if err := s.ensureShiftTypeActive(ctx, successor.ShiftTypeID); err != nil {
			return nil, err
		}
	}
	period, err := s.loadPeriodForSeries(ctx, successor)
	if err != nil {
		return nil, err
	}
	if err := s.lockShiftWrites(ctx, old.StaffID); err != nil {
		return nil, err
	}
	// A predecessor can be edited before a later segment begins. Keep the new
	// segment strictly before that successor, but never reopen a segment that a
	// current successor has already superseded.
	next, err := s.seriesRepo.FindOverlappingInLineage(ctx, rootID, old.ID, effective)
	if err != nil {
		return nil, err
	}
	if next != nil && !effective.Before(next.ValidFrom) {
		return nil, fmt.Errorf("%w: series segment has already been superseded", ErrSeriesInvalid)
	}
	if next != nil && (successor.ValidUntil == nil || next.ValidFrom.Before(*successor.ValidUntil)) {
		until := next.ValidFrom
		successor.ValidUntil = &until
	}
	if !hasFutureSeriesOccurrence(successor, period, today) {
		return nil, withReason(fmt.Errorf(
			"%w: no occurrences left to change for the selected weekdays and week pattern",
			ErrSeriesInvalid,
		), workforce.ErrShiftSeriesNoOccurrences)
	}
	if updateToday {
		updateRetainedOccurrence := old.RetainedOccurrenceShiftID != nil &&
			*old.RetainedOccurrenceShiftID == input.OccurrenceShiftID
		updated, err := s.updateTodayOccurrence(ctx, input, updateRetainedOccurrence)
		if err != nil {
			return nil, err
		}
		if updated {
			retainedID := input.OccurrenceShiftID
			successor.RetainedOccurrenceShiftID = &retainedID
		}
	}
	if err := s.seriesRepo.CapValidUntil(ctx, old.ID, effective); err != nil {
		return nil, err
	}
	deleted, err := s.shiftRepo.DeleteNonDetachedBySeriesFrom(ctx, old.ID, effective)
	if err != nil {
		return nil, err
	}
	if err := s.seriesRepo.Create(ctx, successor); err != nil {
		return nil, err
	}
	// Deviation preservation (#1890 counterpart): detached rows and removed
	// occurrences from the effective date onward belong to the successor now.
	// A retained today occurrence was just detached in place, so it must join
	// the successor too; otherwise reopening it would submit the capped segment
	// ID and a second permanent edit would be rejected as superseded.
	repointFrom := effective
	if updateToday {
		repointFrom = input.EffectiveDate
	}
	if _, err := s.shiftRepo.RepointDetachedSeriesFrom(ctx, old.ID, successor.ID, repointFrom); err != nil {
		return nil, err
	}
	if _, err := s.exceptionRepo.RepointToSeriesFrom(ctx, old.ID, successor.ID, effective); err != nil {
		return nil, err
	}
	run, err := s.materializeSeries(ctx, successor, period)
	if err != nil {
		return nil, err
	}
	s.getLogger().Info("staff shift series split",
		"series_id", old.ID,
		"successor_id", successor.ID,
		"staff_id", old.StaffID,
		"effective", effective.String(),
		"deleted", deleted,
		"created", run.created,
		"skipped", len(run.skipped),
		"non_working_skipped", run.nonWorkingSkipped,
	)
	s.broadcastTimeTrackingChanged(ctx)
	return &SeriesResult{
		Series: successor, OldSeriesID: old.ID, Created: run.created, Deleted: deleted,
		SkippedDates: run.skipped, SkippedNonWorkingDays: run.nonWorkingSkipped,
	}, nil
}

// successorSeries builds the segment a split creates. Fields the edit left
// out (nil) inherit the predecessor's value; staff, period and lineage stay
// with the series.
func successorSeries(old *StaffShiftSeries, input SplitSeriesInput, rootID int64, effective timezone.Date, validUntil *timezone.Date) *StaffShiftSeries {
	successor := &StaffShiftSeries{
		StaffID:             old.StaffID,
		Weekdays:            old.Weekdays,
		StartTime:           input.StartTime,
		EndTime:             input.EndTime,
		BreakMinutes:        input.BreakMinutes,
		ShiftTypeID:         old.ShiftTypeID,
		Notes:               old.Notes,
		CalendarPeriodID:    old.CalendarPeriodID,
		WeekPattern:         old.WeekPattern,
		IncludeSchoolBreaks: old.IncludeSchoolBreaks,
		ValidFrom:           effective,
		ValidUntil:          validUntil,
		SeriesRootID:        &rootID,
		CreatedBy:           input.ActorStaffID,
	}
	if input.Weekdays != nil {
		successor.Weekdays = input.Weekdays
	}
	if input.WeekPattern != nil {
		successor.WeekPattern = *input.WeekPattern
	}
	if input.ShiftTypeIDSet {
		successor.ShiftTypeID = input.ShiftTypeID
	}
	if input.Notes != nil {
		successor.Notes = *input.Notes
	}
	if input.IncludeSchoolBreaks != nil {
		successor.IncludeSchoolBreaks = *input.IncludeSchoolBreaks
	}
	return successor
}

func (s *staffShiftSeriesService) EndSeries(ctx context.Context, seriesID int64, from timezone.Date) (*SeriesResult, error) {
	series, err := s.loadSeries(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	effective := from
	tomorrow := s.todayDate().AddDays(1)
	if tomorrow.After(effective) {
		effective = tomorrow
	}
	if effective.Before(series.ValidFrom) {
		effective = series.ValidFrom
	}
	if err := s.lockShiftWrites(ctx, series.StaffID); err != nil {
		return nil, err
	}
	if err := s.seriesRepo.CapValidUntil(ctx, series.ID, effective); err != nil {
		return nil, err
	}
	deleted, err := s.shiftRepo.DeleteNonDetachedBySeriesFrom(ctx, series.ID, effective)
	if err != nil {
		return nil, err
	}
	s.getLogger().Info("staff shift series ended",
		"series_id", series.ID,
		"staff_id", series.StaffID,
		"effective", effective.String(),
		"deleted", deleted,
	)
	s.broadcastTimeTrackingChanged(ctx)
	return &SeriesResult{Series: series, Deleted: deleted}, nil
}
