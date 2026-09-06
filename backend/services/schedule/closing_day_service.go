package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// ClosingDayService manages tenant-defined OGS closure periods (#1418 3b).
// Closing days carry the same Soll=0 semantics as public holidays; the
// union with holidays happens in NewNonWorkingDayResolver.
type ClosingDayService interface {
	// GetAll returns all closing days of the current tenant.
	GetAll(ctx context.Context) ([]*schedule.ClosingDay, error)
	// GetByID returns a single closing day of the current tenant.
	GetByID(ctx context.Context, id int64) (*schedule.ClosingDay, error)
	// Create validates and stores a new closing day.
	Create(ctx context.Context, day *schedule.ClosingDay) error
	// Update validates and updates an existing closing day.
	Update(ctx context.Context, day *schedule.ClosingDay) error
	// Delete removes a closing day.
	Delete(ctx context.Context, id int64) error

	// ClosingDaysInRange returns the closure periods overlapping [from, to]
	// as stored ranges (for display).
	ClosingDaysInRange(ctx context.Context, from, to timezone.Date) ([]*schedule.ClosingDay, error)
	// ClosingDayDates expands the closure periods overlapping [from, to]
	// into a date set clamped to [from, to] for O(1) lookups in Soll
	// computations — the closing-day analog of HolidayService.HolidayDates.
	ClosingDayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error)
}

type closingDayService struct {
	repo schedule.ClosingDayRepository
}

// NewClosingDayService creates the closing day service.
func NewClosingDayService(repo schedule.ClosingDayRepository) ClosingDayService {
	return &closingDayService{repo: repo}
}

func (s *closingDayService) GetAll(ctx context.Context) ([]*schedule.ClosingDay, error) {
	days, err := s.repo.FindByTenantID(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "get all closing days", Err: err}
	}
	return days, nil
}

func (s *closingDayService) GetByID(ctx context.Context, id int64) (*schedule.ClosingDay, error) {
	day, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, &ScheduleError{Op: "get closing day", Err: err}
	}
	return day, nil
}

func (s *closingDayService) Create(ctx context.Context, day *schedule.ClosingDay) error {
	if err := day.Validate(); err != nil {
		return err
	}
	if err := s.repo.Create(ctx, day); err != nil {
		return &ScheduleError{Op: "create closing day", Err: err}
	}
	return nil
}

func (s *closingDayService) Update(ctx context.Context, day *schedule.ClosingDay) error {
	if err := day.Validate(); err != nil {
		return err
	}
	if err := s.repo.Update(ctx, day); err != nil {
		return &ScheduleError{Op: "update closing day", Err: err}
	}
	return nil
}

func (s *closingDayService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return &ScheduleError{Op: "delete closing day", Err: err}
	}
	return nil
}

func (s *closingDayService) ClosingDaysInRange(ctx context.Context, from, to timezone.Date) ([]*schedule.ClosingDay, error) {
	days, err := s.repo.FindOverlappingRange(ctx, schedule.Date(from), schedule.Date(to))
	if err != nil {
		return nil, &ScheduleError{Op: "closing days in range", Err: err}
	}
	return days, nil
}

func (s *closingDayService) ClosingDayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	if to.Before(from) {
		return map[timezone.Date]bool{}, nil
	}
	days, err := s.repo.FindOverlappingRange(ctx, schedule.Date(from), schedule.Date(to))
	if err != nil {
		return nil, &ScheduleError{Op: "closing day dates", Err: err}
	}

	set := make(map[timezone.Date]bool)
	for _, day := range days {
		start := day.StartDate
		if start.Before(from) {
			start = schedule.Date(from)
		}
		end := day.EndDate
		if end.After(to) {
			end = schedule.Date(to)
		}
		for d := start; !d.After(end); d = d.AddDays(1) {
			set[timezone.Date(d)] = true
		}
	}
	return set, nil
}
