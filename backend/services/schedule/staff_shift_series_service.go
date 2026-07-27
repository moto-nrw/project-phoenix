package schedule

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/uptrace/bun"
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
	Series       *scheduleModels.StaffShiftSeries
	OldSeriesID  int64
	Created      int
	Deleted      int64
	SkippedDates []timezone.Date
}

// SplitSeriesInput carries the edited field set applied from the effective
// date onward ("Ab jetzt dauerhaft"). The calendar period stays with the
// series lineage — a split changes the rule, not the planning period.
// Optional fields (nil) inherit the predecessor's value, so a client editing
// only times never has to know or re-send the series' rhythm.
type SplitSeriesInput struct {
	SeriesID      int64
	EffectiveDate timezone.Date
	Weekdays      []int16 // nil = keep predecessor weekdays
	StartTime     time.Time
	EndTime       time.Time
	BreakMinutes  int
	ShiftTypeID   *int64
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
	ActorStaffID  int64
}

// StaffShiftSeriesService manages recurring shift series (#1889). Series
// materialize concrete StaffShift rows upfront over the calendar period —
// every existing reader keeps working on concrete rows only.
type StaffShiftSeriesService interface {
	// CreateSeries validates the series, persists it, and materializes its
	// concrete shifts from max(valid_from, tomorrow).
	CreateSeries(ctx context.Context, series *scheduleModels.StaffShiftSeries) (*SeriesResult, error)
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
	GetSeries(ctx context.Context, seriesID int64) (*scheduleModels.StaffShiftSeries, error)
}

type staffShiftSeriesService struct {
	seriesRepo    scheduleModels.StaffShiftSeriesRepository
	exceptionRepo scheduleModels.StaffShiftSeriesExceptionRepository
	shiftRepo     scheduleModels.StaffShiftRepository
	staffRepo     usersModels.StaffRepository
	periodRepo    scheduleModels.CalendarPeriodRepository
	shiftTypes    ShiftTypeService
	db            *bun.DB
	broadcaster   realtime.Broadcaster
	logger        *slog.Logger
}

// NewStaffShiftSeriesService creates a new staff shift series service. db is
// used for the per-staff advisory lock shared with single-shift CRUD.
func NewStaffShiftSeriesService(
	seriesRepo scheduleModels.StaffShiftSeriesRepository,
	exceptionRepo scheduleModels.StaffShiftSeriesExceptionRepository,
	shiftRepo scheduleModels.StaffShiftRepository,
	staffRepo usersModels.StaffRepository,
	periodRepo scheduleModels.CalendarPeriodRepository,
	shiftTypes ShiftTypeService,
	db *bun.DB,
	logger *slog.Logger,
) StaffShiftSeriesService {
	return &staffShiftSeriesService{
		seriesRepo:    seriesRepo,
		exceptionRepo: exceptionRepo,
		shiftRepo:     shiftRepo,
		staffRepo:     staffRepo,
		periodRepo:    periodRepo,
		shiftTypes:    shiftTypes,
		db:            db,
		logger:        logger,
	}
}

func (s *staffShiftSeriesService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

// SetBroadcaster injects the tenant-wide SSE broadcaster used to invalidate
// time-tracking views after committed series writes.
func (s *staffShiftSeriesService) SetBroadcaster(broadcaster realtime.Broadcaster) {
	s.broadcaster = broadcaster
}

func (s *staffShiftSeriesService) broadcastTimeTrackingChanged(ctx context.Context) {
	realtime.QueueStaffTimeTrackingChanged(ctx, s.broadcaster, s.getLogger())
}

func (s *staffShiftSeriesService) lockShiftWrites(ctx context.Context, staffID int64) error {
	if s.db == nil {
		return nil
	}
	return LockStaffShiftWrites(ctx, s.db, staffID)
}

// loadPeriodForSeries loads the series' calendar period and enforces the
// period-dependent rules: validity within the period and week A/B only when
// the period actually alternates.
func (s *staffShiftSeriesService) loadPeriodForSeries(ctx context.Context, series *scheduleModels.StaffShiftSeries) (*scheduleModels.CalendarPeriod, error) {
	period, err := s.periodRepo.FindByID(ctx, series.CalendarPeriodID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: calendar period not found", ErrSeriesInvalid)
		}
		return nil, fmt.Errorf("find calendar period for series: %w", err)
	}
	if period == nil {
		return nil, fmt.Errorf("%w: calendar period not found", ErrSeriesInvalid)
	}
	if series.WeekPattern != scheduleModels.WeekPatternEvery && !period.HasWeekCycle() {
		return nil, fmt.Errorf("%w: week A/B requires a calendar period with a week cycle", ErrSeriesInvalid)
	}
	if series.ValidFrom.Before(period.StartDate) || series.ValidFrom.After(period.EndDate) {
		return nil, fmt.Errorf("%w: valid from must lie within the calendar period", ErrSeriesInvalid)
	}
	if series.ValidUntil != nil && series.ValidUntil.After(period.EndDate.AddDays(1)) {
		return nil, fmt.Errorf("%w: valid until must not exceed the calendar period", ErrSeriesInvalid)
	}
	return period, nil
}

// materializeSeries generates the series' concrete shifts between
// max(valid_from, period start, tomorrow) and min(valid_until-1, period end).
// Dates with an exception are skipped; dates where the generated shift would
// overlap ANY existing shift of the staff member (standalone, detached, or
// other series) are skipped and reported.
func (s *staffShiftSeriesService) materializeSeries(ctx context.Context, series *scheduleModels.StaffShiftSeries, period *scheduleModels.CalendarPeriod) (int, []timezone.Date, error) {
	from := series.ValidFrom
	if period.StartDate.After(from) {
		from = period.StartDate
	}
	tomorrow := timezone.TodayDate().AddDays(1)
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
	if to.Before(from) {
		return 0, nil, nil
	}

	exceptionDates, err := s.exceptionRepo.FindDatesBySeriesID(ctx, series.ID)
	if err != nil {
		return 0, nil, err
	}
	excepted := make(map[timezone.Date]bool, len(exceptionDates))
	for _, d := range exceptionDates {
		excepted[d] = true
	}

	existing, err := s.shiftRepo.FindByStaffAndDateRange(ctx, series.StaffID, from, to)
	if err != nil {
		return 0, nil, err
	}
	existingByDate := make(map[timezone.Date][]*scheduleModels.StaffShift, len(existing))
	// Recurrence slots that already have a row of THIS series (detached survivors
	// of a re-plan, or split re-points) are owned: the user deviated that
	// occurrence, so the series must not add a second shift there. A moved row's
	// current Date may be another genuine occurrence; ownership follows the
	// immutable source slot instead.
	ownedDates := make(map[timezone.Date]bool)
	for _, shift := range existing {
		existingByDate[shift.Date] = append(existingByDate[shift.Date], shift)
		if shift.SeriesID != nil && *shift.SeriesID == series.ID {
			ownedDates[seriesOccurrenceDate(shift)] = true
		}
	}

	startTime := timezone.WallClock(series.StartTime)
	endTime := timezone.WallClock(series.EndTime)

	var candidates []*scheduleModels.StaffShift
	var skipped []timezone.Date
	for d := from; !d.After(to); d = d.AddDays(1) {
		if !series.ContainsWeekday(isoWeekday(d)) {
			continue
		}
		if excepted[d] || ownedDates[d] {
			continue
		}
		if !ShouldMaterializeWeekPattern(series.WeekPattern, d, period) {
			continue
		}
		seriesID := series.ID
		occurrenceDate := d
		candidate := &scheduleModels.StaffShift{
			StaffID:              series.StaffID,
			Date:                 d,
			StartTime:            startTime,
			EndTime:              endTime,
			BreakMinutes:         series.BreakMinutes,
			ShiftTypeID:          series.ShiftTypeID,
			Notes:                series.Notes,
			SeriesID:             &seriesID,
			SeriesOccurrenceDate: &occurrenceDate,
			CreatedBy:            series.CreatedBy,
		}
		overlaps := false
		for _, other := range existingByDate[d] {
			// A cancelled shift does not take place, so it neither blocks nor is
			// blocked by the materialized candidate — the freed window is available.
			// This matches the single-shift overlap rule (checkOverlap) and the
			// partial unique index, both of which treat a cancelled row as a free
			// window (#1841). Own detached cancellations are handled earlier via
			// ownedDates, which skips the date outright.
			if other.Cancelled {
				continue
			}
			if candidate.Overlaps(other) {
				overlaps = true
				break
			}
		}
		if overlaps {
			skipped = append(skipped, d)
			continue
		}
		candidates = append(candidates, candidate)
	}

	if err := s.shiftRepo.BulkCreate(ctx, candidates); err != nil {
		return 0, nil, err
	}
	return len(candidates), skipped, nil
}

func (s *staffShiftSeriesService) CreateSeries(ctx context.Context, series *scheduleModels.StaffShiftSeries) (*SeriesResult, error) {
	if err := series.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSeriesInvalid, err.Error())
	}
	staff, err := s.staffRepo.FindByID(ctx, series.StaffID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	if err := s.lockShiftWrites(ctx, series.StaffID); err != nil {
		return nil, err
	}
	if err := s.seriesRepo.Create(ctx, series); err != nil {
		return nil, err
	}
	created, skipped, err := s.materializeSeries(ctx, series, period)
	if err != nil {
		return nil, err
	}
	s.getLogger().Info("staff shift series created",
		"series_id", series.ID,
		"staff_id", series.StaffID,
		"created", created,
		"skipped", len(skipped),
	)
	s.broadcastTimeTrackingChanged(ctx)
	return &SeriesResult{Series: series, Created: created, SkippedDates: skipped}, nil
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

func (s *staffShiftSeriesService) loadSeries(ctx context.Context, id int64) (*scheduleModels.StaffShiftSeries, error) {
	if id <= 0 {
		return nil, ErrSeriesNotFound
	}
	series, err := s.seriesRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

	effective := input.EffectiveDate
	tomorrow := timezone.TodayDate().AddDays(1)
	if tomorrow.After(effective) {
		effective = tomorrow
	}
	if effective.Before(old.ValidFrom) {
		effective = old.ValidFrom
	}
	if old.ValidUntil != nil && !effective.Before(*old.ValidUntil) {
		return nil, fmt.Errorf("%w: effective date lies outside the series segment", ErrSeriesInvalid)
	}

	rootID := old.RootID()
	weekdays := input.Weekdays
	if weekdays == nil {
		weekdays = old.Weekdays
	}
	weekPattern := old.WeekPattern
	if input.WeekPattern != nil {
		weekPattern = *input.WeekPattern
	}
	shiftTypeID := old.ShiftTypeID
	if input.ShiftTypeIDSet {
		shiftTypeID = input.ShiftTypeID
	}
	notes := old.Notes
	if input.Notes != nil {
		notes = *input.Notes
	}
	validUntil := old.ValidUntil
	if input.ValidUntilSet {
		validUntil = input.ValidUntil
	}
	successor := &scheduleModels.StaffShiftSeries{
		StaffID:          old.StaffID,
		Weekdays:         weekdays,
		StartTime:        input.StartTime,
		EndTime:          input.EndTime,
		BreakMinutes:     input.BreakMinutes,
		ShiftTypeID:      shiftTypeID,
		Notes:            notes,
		CalendarPeriodID: old.CalendarPeriodID,
		WeekPattern:      weekPattern,
		ValidFrom:        effective,
		ValidUntil:       validUntil,
		SeriesRootID:     &rootID,
		CreatedBy:        input.ActorStaffID,
	}
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
	// A predecessor can be edited after a later segment already exists. Keep
	// the new segment strictly before that successor even when the editor clears
	// a stored valid_until, otherwise two rules of one lineage overlap.
	next, err := s.seriesRepo.FindNextInLineage(ctx, rootID, effective)
	if err != nil {
		return nil, err
	}
	if next != nil && (successor.ValidUntil == nil || next.ValidFrom.Before(*successor.ValidUntil)) {
		until := next.ValidFrom
		successor.ValidUntil = &until
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
	if _, err := s.shiftRepo.RepointDetachedSeriesFrom(ctx, old.ID, successor.ID, effective); err != nil {
		return nil, err
	}
	if _, err := s.exceptionRepo.RepointToSeriesFrom(ctx, old.ID, successor.ID, effective); err != nil {
		return nil, err
	}
	created, skipped, err := s.materializeSeries(ctx, successor, period)
	if err != nil {
		return nil, err
	}
	s.getLogger().Info("staff shift series split",
		"series_id", old.ID,
		"successor_id", successor.ID,
		"staff_id", old.StaffID,
		"effective", effective.String(),
		"deleted", deleted,
		"created", created,
		"skipped", len(skipped),
	)
	s.broadcastTimeTrackingChanged(ctx)
	return &SeriesResult{Series: successor, OldSeriesID: old.ID, Created: created, Deleted: deleted, SkippedDates: skipped}, nil
}

func (s *staffShiftSeriesService) EndSeries(ctx context.Context, seriesID int64, from timezone.Date) (*SeriesResult, error) {
	series, err := s.loadSeries(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	effective := from
	tomorrow := timezone.TodayDate().AddDays(1)
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
