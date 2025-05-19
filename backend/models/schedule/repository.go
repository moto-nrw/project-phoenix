package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// DateframeRepository defines operations for managing date frames
type DateframeRepository interface {
	base.Repository[*Dateframe]

	// FindByName finds a dateframe by its name
	FindByName(ctx context.Context, name string) (*Dateframe, error)

	// FindByDate finds all dateframes that include the given date
	FindByDate(ctx context.Context, date time.Time) ([]*Dateframe, error)

	// FindOverlapping finds all dateframes that overlap with the given date range
	FindOverlapping(ctx context.Context, startDate, endDate time.Time) ([]*Dateframe, error)
}

// TimeframeRepository defines operations for managing time frames
type TimeframeRepository interface {
	base.Repository[*Timeframe]

	// FindActive finds all active timeframes
	FindActive(ctx context.Context) ([]*Timeframe, error)

	// FindByTimeRange finds all timeframes that overlap with the given time range
	FindByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*Timeframe, error)

	// FindByDescription finds timeframes with matching description
	FindByDescription(ctx context.Context, description string) ([]*Timeframe, error)
}

// RecurrenceRuleRepository defines operations for managing recurrence rules
type RecurrenceRuleRepository interface {
	base.Repository[*RecurrenceRule]

	// FindByFrequency finds all recurrence rules with the specified frequency
	FindByFrequency(ctx context.Context, frequency string) ([]*RecurrenceRule, error)

	// FindByWeekday finds all recurrence rules that include the specified weekday
	FindByWeekday(ctx context.Context, weekday string) ([]*RecurrenceRule, error)

	// FindByMonthDay finds all recurrence rules that include the specified month day
	FindByMonthDay(ctx context.Context, day int) ([]*RecurrenceRule, error)

	// FindByDateRange finds all recurrence rules that apply within the given date range
	FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*RecurrenceRule, error)
}
