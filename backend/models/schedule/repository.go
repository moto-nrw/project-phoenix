package schedule

import (
	"context"
	"time"
)

// DateframeRepository defines operations for managing date frames
type DateframeRepository interface {
	repository[*Dateframe]

	// FindByName finds a dateframe by its name
	FindByName(ctx context.Context, name string) (*Dateframe, error)
}

// TimeframeRepository defines operations for managing time frames
type TimeframeRepository interface {
	repository[*Timeframe]
	ListAll(ctx context.Context) ([]*Timeframe, error)

	// FindByTimeRange finds all timeframes that overlap with the given time range
	FindByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*Timeframe, error)
}

// RecurrenceRuleRepository defines operations for managing recurrence rules
type RecurrenceRuleRepository interface {
	repository[*RecurrenceRule]

	// FindByDateRange finds all recurrence rules that apply within the given date range
	FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*RecurrenceRule, error)
}

// ClassArrivalExceptionRepository is the data access boundary for class-wide
// arrival day exceptions (#2962).
type ClassArrivalExceptionRepository interface {
	// Upsert stores the exception of one class and date, replacing what was
	// there.
	Upsert(ctx context.Context, row *ClassArrivalException) error
}
