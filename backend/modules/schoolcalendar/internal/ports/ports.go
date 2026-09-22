package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
)

// Store is the persistence port over schedule.calendar_periods and
// schedule.closing_days. Reads honour the tenant in context when one is
// present; writes require one.
type Store interface {
	FindCalendarPeriod(ctx context.Context, id int64) (domain.CalendarPeriod, bool, domain.OperationStats, error)
	ListCalendarPeriods(context.Context, domain.CalendarPeriodFilter) ([]domain.CalendarPeriod, domain.OperationStats, error)
	// CreateCalendarPeriod inserts the period. With ifAbsent the insert is
	// skipped when the tenant already has a period of that name; the bool
	// reports whether a row was written.
	CreateCalendarPeriod(ctx context.Context, fields domain.CalendarPeriodFields, ifAbsent bool) (domain.CalendarPeriod, bool, domain.OperationStats, error)
	UpdateCalendarPeriod(context.Context, int64, domain.CalendarPeriodFields) (domain.CalendarPeriod, domain.OperationStats, error)
	DeleteCalendarPeriod(context.Context, int64) (domain.OperationStats, error)

	FindClosingDay(ctx context.Context, id int64) (domain.ClosingDay, bool, domain.OperationStats, error)
	ListClosingDays(context.Context, domain.ClosingDayFilter) ([]domain.ClosingDay, domain.OperationStats, error)
	CreateClosingDay(context.Context, domain.ClosingDayFields) (domain.ClosingDay, domain.OperationStats, error)
	UpdateClosingDay(context.Context, int64, domain.ClosingDayFields) (domain.ClosingDay, domain.OperationStats, error)
	DeleteClosingDay(context.Context, int64) (domain.OperationStats, error)

	FindDateframe(ctx context.Context, id int64) (domain.Dateframe, bool, domain.OperationStats, error)
	ListDateframes(context.Context, domain.DateframeFilter) ([]domain.Dateframe, domain.OperationStats, error)
	CreateDateframe(context.Context, domain.DateframeFields) (domain.Dateframe, domain.OperationStats, error)
	UpdateDateframe(context.Context, int64, domain.DateframeFields) (domain.Dateframe, domain.OperationStats, error)
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
type CareOfferingGuard func(ctx context.Context, periodID int64, replacement *domain.CalendarPeriodFields) error

// Today names the current calendar day in YYYY-MM-DD.
type Today func() string

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
