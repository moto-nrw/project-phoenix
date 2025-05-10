// backend/services/schedule/interface.go
package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// Service defines operations for managing schedules
type Service interface {
	// Dateframe operations
	GetDateframe(ctx context.Context, id int64) (*schedule.Dateframe, error)
	CreateDateframe(ctx context.Context, dateframe *schedule.Dateframe) error
	UpdateDateframe(ctx context.Context, dateframe *schedule.Dateframe) error
	DeleteDateframe(ctx context.Context, id int64) error
	ListDateframes(ctx context.Context, options *base.QueryOptions) ([]*schedule.Dateframe, error)
	FindDateframesByDate(ctx context.Context, date time.Time) ([]*schedule.Dateframe, error)
	FindOverlappingDateframes(ctx context.Context, startDate, endDate time.Time) ([]*schedule.Dateframe, error)

	// Timeframe operations
	GetTimeframe(ctx context.Context, id int64) (*schedule.Timeframe, error)
	CreateTimeframe(ctx context.Context, timeframe *schedule.Timeframe) error
	UpdateTimeframe(ctx context.Context, timeframe *schedule.Timeframe) error
	DeleteTimeframe(ctx context.Context, id int64) error
	ListTimeframes(ctx context.Context, options *base.QueryOptions) ([]*schedule.Timeframe, error)
	FindActiveTimeframes(ctx context.Context) ([]*schedule.Timeframe, error)
	FindTimeframesByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*schedule.Timeframe, error)

	// RecurrenceRule operations
	GetRecurrenceRule(ctx context.Context, id int64) (*schedule.RecurrenceRule, error)
	CreateRecurrenceRule(ctx context.Context, rule *schedule.RecurrenceRule) error
	UpdateRecurrenceRule(ctx context.Context, rule *schedule.RecurrenceRule) error
	DeleteRecurrenceRule(ctx context.Context, id int64) error
	ListRecurrenceRules(ctx context.Context, options *base.QueryOptions) ([]*schedule.RecurrenceRule, error)
	FindRecurrenceRulesByFrequency(ctx context.Context, frequency string) ([]*schedule.RecurrenceRule, error)
	FindRecurrenceRulesByWeekday(ctx context.Context, weekday string) ([]*schedule.RecurrenceRule, error)

	// Advanced operations
	GenerateEvents(ctx context.Context, ruleID int64, startDate, endDate time.Time) ([]time.Time, error)
	CheckConflict(ctx context.Context, startTime, endTime time.Time) (bool, []*schedule.Timeframe, error)
	FindAvailableSlots(ctx context.Context, startDate, endDate time.Time, duration time.Duration) ([]*schedule.Timeframe, error)
	GetCurrentDateframe(ctx context.Context) (*schedule.Dateframe, error)
}
