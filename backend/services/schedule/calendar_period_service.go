package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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

	// Auto-creation
	GetOrCreateDefaultPeriod(ctx context.Context) (*schedule.CalendarPeriod, error)

	// A/B week resolution — weekPattern: 0=every, 1=week A, 2=week B
	ShouldMaterialize(weekPattern int, instanceDate time.Time, period *schedule.CalendarPeriod) bool
}

// calendarPeriodService implements CalendarPeriodService
type calendarPeriodService struct {
	repo   schedule.CalendarPeriodRepository
	db     *bun.DB
	logger *slog.Logger
}

// NewCalendarPeriodService creates a new calendar period service
func NewCalendarPeriodService(
	repo schedule.CalendarPeriodRepository,
	db *bun.DB,
	logger *slog.Logger,
) CalendarPeriodService {
	return &calendarPeriodService{
		repo:   repo,
		db:     db,
		logger: logger,
	}
}

func (s *calendarPeriodService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
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
		return &ScheduleError{Op: "create calendar period", Err: errors.New("a period with this name already exists")}
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
		return &ScheduleError{Op: "update calendar period", Err: errors.New("a period with this name already exists")}
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

// GetOrCreateDefaultPeriod returns the default school-year period for the tenant,
// creating one if none exists. Uses the current school year dates (Aug 1 - Jul 31).
func (s *calendarPeriodService) GetOrCreateDefaultPeriod(ctx context.Context) (*schedule.CalendarPeriod, error) {
	periods, err := s.repo.FindByTenantID(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "get or create default period", Err: err}
	}

	if len(periods) > 0 {
		// Return the first active period, or the first period if none are active
		for _, p := range periods {
			if p.IsActive {
				return p, nil
			}
		}
		return periods[0], nil
	}

	// No periods exist — create a default school year
	now := time.Now()
	year := now.Year()

	// German school year: Aug 1 - Jul 31
	// If we're past Aug 1, current year starts this Aug; otherwise last Aug
	var startDate, endDate time.Time
	if now.Month() >= time.August {
		startDate = time.Date(year, time.August, 1, 0, 0, 0, 0, time.UTC)
		endDate = time.Date(year+1, time.July, 31, 0, 0, 0, 0, time.UTC)
	} else {
		startDate = time.Date(year-1, time.August, 1, 0, 0, 0, 0, time.UTC)
		endDate = time.Date(year, time.July, 31, 0, 0, 0, 0, time.UTC)
	}

	defaultName := fmt.Sprintf("Schuljahr %d/%d", startDate.Year(), endDate.Year())

	period := &schedule.CalendarPeriod{
		Name:            defaultName,
		PeriodType:      schedule.PeriodTypeSchoolYear,
		StartDate:       startDate,
		EndDate:         endDate,
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(tenant.FromContext(ctx))

	if err := s.repo.Create(ctx, period); err != nil {
		return nil, &ScheduleError{Op: "get or create default period", Err: err}
	}

	s.getLogger().Info("default calendar period created",
		slog.Int64("period_id", period.ID),
		slog.String("name", period.Name),
	)

	return period, nil
}

// ShouldMaterialize determines whether a schedule with the given weekPattern should
// produce an instance on instanceDate, considering the period's A/B week cycle.
//
// Uses day-based difference calculation (NOT ISO week numbers) to avoid
// year-boundary bugs. See timetable-system-plan.md §6.1 for algorithm details.
//
// weekPattern: 0=every week, 1=week A, 2=week B (maps to currentPattern 1, 2, ...)
func (s *calendarPeriodService) ShouldMaterialize(weekPattern int, instanceDate time.Time, period *schedule.CalendarPeriod) bool {
	if weekPattern == 0 {
		return true // every week
	}
	if period == nil || period.WeekCycleLength <= 1 {
		return true // no alternation configured
	}
	if period.WeekCycleAnchor == nil {
		return true // no anchor set, can't compute — allow by default
	}

	// Day-based difference from anchor to instance date
	anchorDate := period.WeekCycleAnchor.Truncate(24 * time.Hour)
	instDate := instanceDate.Truncate(24 * time.Hour)

	daysDiff := instDate.Sub(anchorDate).Hours() / 24
	weeksDiff := int(math.Floor(daysDiff / 7))

	// Modulo that handles negative values correctly
	cycleLen := period.WeekCycleLength
	currentPattern := ((weeksDiff % cycleLen) + cycleLen) % cycleLen
	currentPattern++ // 1-based: 1=A, 2=B, etc.

	return currentPattern == weekPattern
}
