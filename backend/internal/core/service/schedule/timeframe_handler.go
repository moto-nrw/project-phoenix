package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/schedule"
	schedulePort "github.com/moto-nrw/project-phoenix/internal/core/port/schedule"
)

// timeframeHandler handles timeframe-specific operations
type timeframeHandler struct {
	repo schedulePort.TimeframeRepository
}

// newTimeframeHandler creates a new timeframe handler
func newTimeframeHandler(repo schedulePort.TimeframeRepository) *timeframeHandler {
	return &timeframeHandler{repo: repo}
}

// GetTimeframe retrieves a timeframe by its ID
func (h *timeframeHandler) GetTimeframe(ctx context.Context, id int64) (*schedule.Timeframe, error) {
	timeframe, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return nil, &ScheduleError{Op: "get timeframe", Err: ErrTimeframeNotFound}
	}
	return timeframe, nil
}

// CreateTimeframe creates a new timeframe
func (h *timeframeHandler) CreateTimeframe(ctx context.Context, timeframe *schedule.Timeframe) error {
	if err := timeframe.Validate(); err != nil {
		return &ScheduleError{Op: "create timeframe", Err: err}
	}

	if err := h.repo.Create(ctx, timeframe); err != nil {
		return &ScheduleError{Op: "create timeframe", Err: err}
	}

	return nil
}

// UpdateTimeframe updates an existing timeframe
func (h *timeframeHandler) UpdateTimeframe(ctx context.Context, timeframe *schedule.Timeframe) error {
	if err := timeframe.Validate(); err != nil {
		return &ScheduleError{Op: "update timeframe", Err: err}
	}

	if err := h.repo.Update(ctx, timeframe); err != nil {
		return &ScheduleError{Op: "update timeframe", Err: err}
	}

	return nil
}

// DeleteTimeframe deletes a timeframe by its ID
func (h *timeframeHandler) DeleteTimeframe(ctx context.Context, id int64) error {
	if err := h.repo.Delete(ctx, id); err != nil {
		return &ScheduleError{Op: "delete timeframe", Err: err}
	}

	return nil
}

// ListTimeframes retrieves all timeframes matching the provided filters
func (h *timeframeHandler) ListTimeframes(ctx context.Context, options *base.QueryOptions) ([]*schedule.Timeframe, error) {
	timeframes, err := h.repo.List(ctx, options)
	if err != nil {
		return nil, &ScheduleError{Op: "list timeframes", Err: err}
	}

	return timeframes, nil
}

// FindActiveTimeframes finds all active timeframes
func (h *timeframeHandler) FindActiveTimeframes(ctx context.Context) ([]*schedule.Timeframe, error) {
	timeframes, err := h.repo.FindActive(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "find active timeframes", Err: err}
	}

	return timeframes, nil
}

// FindTimeframesByTimeRange finds all timeframes that overlap with the given time range
func (h *timeframeHandler) FindTimeframesByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*schedule.Timeframe, error) {
	// Validate time range
	if !endTime.IsZero() && startTime.After(endTime) {
		return nil, &ScheduleError{Op: "find timeframes by time range", Err: ErrInvalidTimeRange}
	}

	timeframes, err := h.repo.FindByTimeRange(ctx, startTime, endTime)
	if err != nil {
		return nil, &ScheduleError{Op: "find timeframes by time range", Err: err}
	}

	return timeframes, nil
}
