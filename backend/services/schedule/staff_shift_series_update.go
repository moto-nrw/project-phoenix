package schedule

import (
	"context"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// GetSeries returns the rule behind a shift so the planner can edit the series
// itself — its weekdays, rhythm, window, and validity — instead of only one
// occurrence (#2028). The edit is written through SplitSeries: it takes effect
// from the opened occurrence onwards and leaves everything before it alone.
func (s *staffShiftSeriesService) GetSeries(ctx context.Context, seriesID int64) (*scheduleModels.StaffShiftSeries, error) {
	return s.loadSeries(ctx, seriesID)
}
