package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/schedule"
)

// Dateframe operations

// GetDateframe retrieves a dateframe by its ID
func (s *service) GetDateframe(ctx context.Context, id int64) (*schedule.Dateframe, error) {
	dateframe, err := s.dateframeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ScheduleError{Op: "get dateframe", Err: ErrDateframeNotFound}
	}
	return dateframe, nil
}

// CreateDateframe creates a new dateframe
func (s *service) CreateDateframe(ctx context.Context, dateframe *schedule.Dateframe) error {
	if err := dateframe.Validate(); err != nil {
		return &ScheduleError{Op: "create dateframe", Err: err}
	}

	if err := s.dateframeRepo.Create(ctx, dateframe); err != nil {
		return &ScheduleError{Op: "create dateframe", Err: err}
	}

	return nil
}

// UpdateDateframe updates an existing dateframe
func (s *service) UpdateDateframe(ctx context.Context, dateframe *schedule.Dateframe) error {
	if err := dateframe.Validate(); err != nil {
		return &ScheduleError{Op: "update dateframe", Err: err}
	}

	if err := s.dateframeRepo.Update(ctx, dateframe); err != nil {
		return &ScheduleError{Op: "update dateframe", Err: err}
	}

	return nil
}

// DeleteDateframe deletes a dateframe by its ID
func (s *service) DeleteDateframe(ctx context.Context, id int64) error {
	if err := s.dateframeRepo.Delete(ctx, id); err != nil {
		return &ScheduleError{Op: "delete dateframe", Err: err}
	}

	return nil
}

// ListDateframes retrieves all dateframes matching the provided filters
func (s *service) ListDateframes(ctx context.Context, options *base.QueryOptions) ([]*schedule.Dateframe, error) {
	dateframes, err := s.dateframeRepo.List(ctx, options)
	if err != nil {
		return nil, &ScheduleError{Op: "list dateframes", Err: err}
	}

	return dateframes, nil
}

// FindDateframesByDate finds all dateframes that include the given date
func (s *service) FindDateframesByDate(ctx context.Context, date time.Time) ([]*schedule.Dateframe, error) {
	dateframes, err := s.dateframeRepo.FindByDate(ctx, date)
	if err != nil {
		return nil, &ScheduleError{Op: "find dateframes by date", Err: err}
	}

	return dateframes, nil
}

// FindOverlappingDateframes finds all dateframes that overlap with the given date range
func (s *service) FindOverlappingDateframes(ctx context.Context, startDate, endDate time.Time) ([]*schedule.Dateframe, error) {
	if startDate.After(endDate) {
		return nil, &ScheduleError{Op: "find overlapping dateframes", Err: ErrInvalidDateRange}
	}

	dateframes, err := s.dateframeRepo.FindOverlapping(ctx, startDate, endDate)
	if err != nil {
		return nil, &ScheduleError{Op: "find overlapping dateframes", Err: err}
	}

	return dateframes, nil
}

// GetCurrentDateframe gets the active dateframe for the current date
func (s *service) GetCurrentDateframe(ctx context.Context) (*schedule.Dateframe, error) {
	now := time.Now()

	dateframes, err := s.dateframeRepo.FindByDate(ctx, now)
	if err != nil {
		return nil, &ScheduleError{Op: "get current dateframe", Err: err}
	}

	if len(dateframes) == 0 {
		return nil, &ScheduleError{Op: "get current dateframe", Err: ErrDateframeNotFound}
	}

	// If multiple dateframes are active, prioritize by name or creation date
	// For now, just return the first one
	return dateframes[0], nil
}
