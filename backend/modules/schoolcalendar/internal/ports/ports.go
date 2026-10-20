package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
)

// Store is the persistence port over schedule.calendar_periods and
// schedule.closing_days. Reads honour the tenant in context when one is
// present; writes require one.
type Store interface {
	FindCalendarPeriod(ctx context.Context, id int64) (schoolcalendar.CalendarPeriod, bool, domain.OperationStats, error)
	ListCalendarPeriods(context.Context, schoolcalendar.CalendarPeriodFilter) ([]schoolcalendar.CalendarPeriod, domain.OperationStats, error)
	// CreateCalendarPeriod inserts the period. With ifAbsent the insert is
	// skipped when the tenant already has a period of that name; the bool
	// reports whether a row was written.
	CreateCalendarPeriod(ctx context.Context, fields schoolcalendar.CalendarPeriodFields, ifAbsent bool) (schoolcalendar.CalendarPeriod, bool, domain.OperationStats, error)
	UpdateCalendarPeriod(context.Context, int64, schoolcalendar.CalendarPeriodFields) (schoolcalendar.CalendarPeriod, domain.OperationStats, error)
	DeleteCalendarPeriod(context.Context, int64) (domain.OperationStats, error)

	FindClosingDay(ctx context.Context, id int64) (schoolcalendar.ClosingDay, bool, domain.OperationStats, error)
	ListClosingDays(context.Context, schoolcalendar.ClosingDayFilter) ([]schoolcalendar.ClosingDay, domain.OperationStats, error)
	CreateClosingDay(context.Context, schoolcalendar.ClosingDayFields) (schoolcalendar.ClosingDay, domain.OperationStats, error)
	UpdateClosingDay(context.Context, int64, schoolcalendar.ClosingDayFields) (schoolcalendar.ClosingDay, domain.OperationStats, error)
	DeleteClosingDay(context.Context, int64) (domain.OperationStats, error)

	FindDateframe(ctx context.Context, id int64) (schoolcalendar.Dateframe, bool, domain.OperationStats, error)
	ListDateframes(context.Context, schoolcalendar.DateframeFilter) ([]schoolcalendar.Dateframe, domain.OperationStats, error)
	CreateDateframe(context.Context, schoolcalendar.DateframeFields) (schoolcalendar.Dateframe, domain.OperationStats, error)
	UpdateDateframe(context.Context, int64, schoolcalendar.DateframeFields) (schoolcalendar.Dateframe, domain.OperationStats, error)
	DeleteDateframe(context.Context, int64) (domain.OperationStats, error)
}

// WriteTransaction runs fn inside the tenant's write transaction, joining
// the ambient one when the caller already opened it.
type WriteTransaction func(ctx context.Context, fn func(context.Context) error) error

// RecurrenceGate serializes a calendar-period mutation against every
// recurrence-derived write of the tenant (template re-planning,
// materialization, care-offering rosters). It must run inside the
// transaction the mutation commits in.
type RecurrenceGate func(context.Context) error

// CareOfferingGuard refuses a period change (replacement set) or removal
// (replacement nil) while a linked care offering still needs the period. It
// answers with schoolcalendar.ErrCalendarPeriodRequiredByCareOffering in the
// error chain.
type CareOfferingGuard func(ctx context.Context, periodID int64, replacement *schoolcalendar.CalendarPeriodFields) error

// Today names the current calendar day in YYYY-MM-DD.
type Today func() string

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
