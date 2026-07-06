package schedule

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CalendarPeriodService defines operations for managing calendar periods
type CalendarPeriodService interface {
	// CRUD
	GetAllPeriods(ctx context.Context) ([]*schedule.CalendarPeriod, error)
	GetActivePeriods(ctx context.Context) ([]*schedule.CalendarPeriod, error)
	GetPeriodByID(ctx context.Context, id int64) (*schedule.CalendarPeriod, error)
	CreatePeriod(ctx context.Context, period *schedule.CalendarPeriod) error
	UpdatePeriod(ctx context.Context, period *schedule.CalendarPeriod) error
	DeletePeriod(ctx context.Context, id int64) error

	// EnsureDefaultSchoolYear guarantees the tenant has at least one calendar
	// period. If none exists, it creates the current German school year
	// (Aug 1 – Jul 31) as an active period. Idempotent and race-safe;
	// created reports whether this call inserted the default period.
	EnsureDefaultSchoolYear(ctx context.Context) (periods []*schedule.CalendarPeriod, created bool, err error)

	// FindActiveOverlaps returns the active periods whose date range overlaps
	// the given period (excluding the period itself). Returns nil when the
	// period is inactive — only active/active collisions are advisory-worthy.
	FindActiveOverlaps(ctx context.Context, period *schedule.CalendarPeriod) ([]*schedule.CalendarPeriod, error)

	// A/B week resolution — weekPattern: 0=every, 1=week A, 2=week B
	ShouldMaterialize(weekPattern int, instanceDate timezone.Date, period *schedule.CalendarPeriod) bool
}

// calendarPeriodService implements CalendarPeriodService
type calendarPeriodService struct {
	repo   schedule.CalendarPeriodRepository
	logger *slog.Logger
}

// NewCalendarPeriodService creates a new calendar period service
func NewCalendarPeriodService(
	repo schedule.CalendarPeriodRepository,
	logger *slog.Logger,
) CalendarPeriodService {
	return &calendarPeriodService{
		repo:   repo,
		logger: logger,
	}
}

func (s *calendarPeriodService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

// GetAllPeriods returns all calendar periods for the current tenant
func (s *calendarPeriodService) GetAllPeriods(ctx context.Context) ([]*schedule.CalendarPeriod, error) {
	periods, err := s.repo.FindByTenantID(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "get all calendar periods", Err: err}
	}
	return periods, nil
}

// GetActivePeriods returns all active calendar periods for the current tenant
func (s *calendarPeriodService) GetActivePeriods(ctx context.Context) ([]*schedule.CalendarPeriod, error) {
	periods, err := s.repo.FindActiveByTenantID(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "get active calendar periods", Err: err}
	}
	return periods, nil
}

// GetPeriodByID returns a calendar period by its ID
func (s *calendarPeriodService) GetPeriodByID(ctx context.Context, id int64) (*schedule.CalendarPeriod, error) {
	period, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, &ScheduleError{Op: "get calendar period by id", Err: err}
	}
	return period, nil
}

// CreatePeriod creates a new calendar period
func (s *calendarPeriodService) CreatePeriod(ctx context.Context, period *schedule.CalendarPeriod) error {
	if err := period.Validate(); err != nil {
		return &ScheduleError{Op: "create calendar period", Err: err}
	}

	// Check for duplicate name
	existing, err := s.repo.FindByName(ctx, period.Name)
	if err != nil {
		return &ScheduleError{Op: "create calendar period", Err: err}
	}
	if existing != nil {
		return &ScheduleError{Op: "create calendar period", Err: schedule.ErrCalendarPeriodNameConflict}
	}

	period.SetTenantID(tenant.FromContext(ctx))
	if err := s.repo.Create(ctx, period); err != nil {
		return &ScheduleError{Op: "create calendar period", Err: err}
	}

	s.getLogger().Info("calendar period created",
		slog.Int64("period_id", period.ID),
		slog.String("name", period.Name),
		slog.String("period_type", period.PeriodType),
	)

	return nil
}

// UpdatePeriod updates an existing calendar period
func (s *calendarPeriodService) UpdatePeriod(ctx context.Context, period *schedule.CalendarPeriod) error {
	if err := period.Validate(); err != nil {
		return &ScheduleError{Op: "update calendar period", Err: err}
	}

	// Check for duplicate name (excluding self)
	existing, err := s.repo.FindByName(ctx, period.Name)
	if err != nil {
		return &ScheduleError{Op: "update calendar period", Err: err}
	}
	if existing != nil && existing.ID != period.ID {
		return &ScheduleError{Op: "update calendar period", Err: schedule.ErrCalendarPeriodNameConflict}
	}

	if err := s.repo.Update(ctx, period); err != nil {
		return &ScheduleError{Op: "update calendar period", Err: err}
	}

	return nil
}

// DeletePeriod deletes a calendar period by ID
func (s *calendarPeriodService) DeletePeriod(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return &ScheduleError{Op: "delete calendar period", Err: err}
	}
	return nil
}

// defaultSchoolYearBounds computes the German school year containing the
// given calendar day: it starts on August 1st of the current year when the
// day is in August or later, otherwise on August 1st of the previous year,
// and always ends on July 31st of the following year.
//
// MUST stay in sync with the frontend helper schoolYearPeriodDefaults in
// frontend/src/app/[tenant]/(protected)/timetables/page.tsx — both sides
// derive the same name ("Schuljahr YYYY/YYYY+1") and the same date bounds
// so the bootstrap endpoint and the client-side prefill never diverge.
//
// Extracted as a pure function so the year-boundary logic is testable
// without injecting a clock.
func defaultSchoolYearBounds(today timezone.Date) (name string, start, end timezone.Date) {
	startYear := today.Year
	if today.Month < time.August {
		startYear--
	}
	name = fmt.Sprintf("Schuljahr %d/%d", startYear, startYear+1)
	return name, timezone.NewDate(startYear, time.August, 1), timezone.NewDate(startYear+1, time.July, 31)
}

// EnsureDefaultSchoolYear guarantees the tenant has at least one calendar
// period (WP-B1). When periods already exist, it returns them unchanged with
// created=false. Otherwise it inserts the current school year via the
// race-safe CreateIfAbsent upsert, re-lists, and returns the fresh state.
// Two concurrent calls therefore yield exactly one row and zero errors.
func (s *calendarPeriodService) EnsureDefaultSchoolYear(ctx context.Context) ([]*schedule.CalendarPeriod, bool, error) {
	periods, err := s.repo.FindByTenantID(ctx)
	if err != nil {
		return nil, false, &ScheduleError{Op: "ensure default school year", Err: err}
	}
	if len(periods) > 0 {
		return periods, false, nil
	}

	name, start, end := defaultSchoolYearBounds(timezone.TodayDate())
	period := &schedule.CalendarPeriod{
		Name:            name,
		PeriodType:      schedule.PeriodTypeSchoolYear,
		StartDate:       start,
		EndDate:         end,
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(tenant.FromContext(ctx))

	created, err := s.repo.CreateIfAbsent(ctx, period)
	if err != nil {
		return nil, false, &ScheduleError{Op: "ensure default school year", Err: err}
	}
	if created {
		s.getLogger().Info("default school year period created",
			slog.Int64("period_id", period.ID),
			slog.String("name", period.Name),
		)
	}

	periods, err = s.repo.FindByTenantID(ctx)
	if err != nil {
		return nil, false, &ScheduleError{Op: "ensure default school year", Err: err}
	}
	return periods, created, nil
}

// FindActiveOverlaps returns the active periods overlapping the given period
// (WP-B2, QA #1577 H3). The check is advisory only — callers attach the
// result as a warning, never as a save blocker. Inactive periods cannot
// collide with anything, so the repo round-trip is skipped for them.
func (s *calendarPeriodService) FindActiveOverlaps(ctx context.Context, period *schedule.CalendarPeriod) ([]*schedule.CalendarPeriod, error) {
	if period == nil || !period.IsActive {
		return nil, nil
	}
	overlaps, err := s.repo.FindActiveOverlapping(ctx, period.StartDate, period.EndDate, period.ID)
	if err != nil {
		return nil, &ScheduleError{Op: "find active overlapping periods", Err: err}
	}
	return overlaps, nil
}

// ShouldMaterialize determines whether a schedule with the given weekPattern should
// produce an instance on instanceDate, considering the period's A/B week cycle.
//
// Uses day-based difference calculation (NOT ISO week numbers) to avoid
// year-boundary bugs. See timetable-system-plan.md §6.1 for algorithm details.
//
// timezone.Date.DaysUntil anchors both calendar days at UTC midnight before
// subtracting, so DST transitions in Europe/Berlin (167- or 169-hour weeks at
// the end of March/October) can never skew the day count.
//
// weekPattern: 0=every week, 1=week A, 2=week B (maps to currentPattern 1, 2, ...)
func (s *calendarPeriodService) ShouldMaterialize(weekPattern int, instanceDate timezone.Date, period *schedule.CalendarPeriod) bool {
	if weekPattern == 0 {
		return true // every week
	}
	if period == nil || period.WeekCycleLength <= 1 {
		return true // no alternation configured
	}
	if period.WeekCycleAnchor == nil {
		return true // no anchor set, can't compute — allow by default
	}

	daysDiff := period.WeekCycleAnchor.DaysUntil(instanceDate)
	weeksDiff := daysDiff / 7
	// Go's integer division truncates toward zero, so for negative daysDiff
	// we need an explicit floor when the division isn't exact.
	if daysDiff < 0 && daysDiff%7 != 0 {
		weeksDiff--
	}

	// Modulo that handles negative values correctly.
	cycleLen := period.WeekCycleLength
	currentPattern := ((weeksDiff % cycleLen) + cycleLen) % cycleLen
	currentPattern++ // 1-based: 1=A, 2=B, etc.

	return currentPattern == weekPattern
}
