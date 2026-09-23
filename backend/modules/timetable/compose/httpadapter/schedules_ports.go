package httpadapter

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Use shared constants from common package
var scheduleDateLayout = common.DateFormatISO

const (
	errMsgInvalidDateframeID      = "invalid dateframe ID"
	errMsgInvalidTimeframeID      = "invalid timeframe ID"
	errMsgInvalidRecurrenceRuleID = "invalid recurrence rule ID"
	errMsgInvalidStartDate        = "invalid start date format"
	errMsgInvalidEndDate          = "invalid end date format"
	errMsgInvalidStartTime        = "invalid start time format"
	errMsgInvalidEndTime          = "invalid end time format"
	msgRecurrenceRulesRetrieved   = "Recurrence rules retrieved successfully"
)

// Dateframes is the slice of the School Calendar the schedules API drives.
type Dateframes interface {
	FindDateframe(context.Context, int64) (schoolcalendar.Dateframe, error)
	ListDateframes(context.Context, schoolcalendar.DateframeFilter) ([]schoolcalendar.Dateframe, error)
	CreateDateframe(context.Context, schoolcalendar.CreateDateframe) (schoolcalendar.Dateframe, error)
	UpdateDateframe(context.Context, schoolcalendar.UpdateDateframe) (schoolcalendar.Dateframe, error)
	DeleteDateframe(context.Context, int64) error
}

// Timeframes is the slice of the Timetable owner the schedules API drives:
// timeframes, recurrence rules and recurrence expansion.
type Timeframes interface {
	timetableModule.TimeframeCapability
	timetableModule.RecurrenceRuleCapability
}

// TimeframeChangeGuard runs before a timeframe edit (replacement set) or
// deletion (replacement nil): it serializes the change under the tenant
// recurrence gate and refuses it with
// timetable.ErrTimeframeRequiredByCareOffering while a linked care offering
// still needs the timeframe. Nil skips the guard (unit fixtures).
type TimeframeChangeGuard func(ctx context.Context, timeframeID int64, replacement *timetableModule.TimeframeInput) error

// Recurrence frequencies the rule requests accept; mirrors the owner's
// persisted vocabulary.
const (
	recurrenceFrequencyDaily   = "daily"
	recurrenceFrequencyWeekly  = "weekly"
	recurrenceFrequencyMonthly = "monthly"
	recurrenceFrequencyYearly  = "yearly"
)

// dateframeMidnight drops the clock in the instant's own location, the
// normalisation the retained lookups applied before comparing.
func dateframeMidnight(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func (rs *SchedulesResource) findTimeframeSlot(ctx context.Context, id int64) (timeframeSlot, error) {
	value, err := rs.Timeframes.FindTimeframe(ctx, id)
	if err != nil {
		return timeframeSlot{}, err
	}
	return timeframeToSlot(value)
}

func (rs *SchedulesResource) listTimeframeSlots(ctx context.Context, filter timetableModule.TimeframeFilter) ([]timeframeSlot, error) {
	values, err := rs.Timeframes.ListTimeframes(ctx, filter)
	if err != nil {
		return nil, err
	}
	return timeframesToSlots(values)
}

func (rs *SchedulesResource) guardTimeframeChange(ctx context.Context, id int64, replacement *timetableModule.TimeframeInput) error {
	if rs.TimeframeGuard == nil {
		return nil
	}
	return rs.TimeframeGuard(ctx, id, replacement)
}
