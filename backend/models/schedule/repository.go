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

	// FindByDate finds all dateframes that include the given date
	FindByDate(ctx context.Context, date time.Time) ([]*Dateframe, error)

	// FindOverlapping finds all dateframes that overlap with the given date range
	FindOverlapping(ctx context.Context, startDate, endDate time.Time) ([]*Dateframe, error)
}

// TimeframeRepository defines operations for managing time frames
type TimeframeRepository interface {
	repository[*Timeframe]
	ListAll(ctx context.Context) ([]*Timeframe, error)

	// FindActive finds all active timeframes
	FindActive(ctx context.Context) ([]*Timeframe, error)

	// FindByTimeRange finds all timeframes that overlap with the given time range
	FindByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*Timeframe, error)
}

// RecurrenceRuleRepository defines operations for managing recurrence rules
type RecurrenceRuleRepository interface {
	repository[*RecurrenceRule]

	// FindByFrequency finds all recurrence rules with the specified frequency
	FindByFrequency(ctx context.Context, frequency string) ([]*RecurrenceRule, error)

	// FindByWeekday finds all recurrence rules that include the specified weekday
	FindByWeekday(ctx context.Context, weekday string) ([]*RecurrenceRule, error)

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
