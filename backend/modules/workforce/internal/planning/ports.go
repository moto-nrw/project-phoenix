package planning

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The persistence ports of the Dienstplan services. They are consumer-owned:
// each carries exactly the operations the services here call. The Dienstplan
// rows speak Workforce's own row types (rows.go) and are bound to the
// Workforce capability through ShiftRows and its siblings (#3418, #3424);
// the calendar period, the timetable blocks and their staffing arrive in the
// School Calendar's and the Timetable owner's public vocabulary, bound by the
// composition root.

// staffShiftRows reads and writes the concrete schedule.staff_shifts rows a
// planned shift is stored as.
type staffShiftRows interface {
	Create(ctx context.Context, shift *StaffShift) error
	FindByID(ctx context.Context, id any) (*StaffShift, error)
	Update(ctx context.Context, shift *StaffShift) error
	Delete(ctx context.Context, id any) error

	// FindByDateRange returns all shifts with start <= date <= end for the
	// current tenant, ordered by date, staff, start time.
	FindByDateRange(ctx context.Context, start, end timezone.Date) ([]*StaffShift, error)

	// FindByStaffAndDateRange returns one staff member's shifts in the range.
	FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end timezone.Date) ([]*StaffShift, error)

	// FindByOriginShiftID returns every replacement shift covering the given
	// origin (its cover set), so a cancellation resolves them atomically (#1841).
	FindByOriginShiftID(ctx context.Context, originShiftID int64) ([]*StaffShift, error)

	// BulkCreate inserts all shifts in one multi-row statement (series
	// materialization, #1889).
	BulkCreate(ctx context.Context, shifts []*StaffShift) error

	// DeleteNonDetachedBySeriesFrom removes a series' regenerable rows on or
	// after from. Detached rows ("Nur diese Woche" edits) survive.
	DeleteNonDetachedBySeriesFrom(ctx context.Context, seriesID int64, from timezone.Date) (int64, error)

	// RepointDetachedSeriesFrom moves a series' detached rows on or after
	// from to the successor series created by a split.
	RepointDetachedSeriesFrom(ctx context.Context, fromSeriesID, toSeriesID int64, from timezone.Date) (int64, error)
}

// staffShiftSeriesRows stores the recurrence rules concrete shifts are
// materialized from (#1889).
type staffShiftSeriesRows interface {
	Create(ctx context.Context, series *StaffShiftSeries) error
	FindByID(ctx context.Context, id any) (*StaffShiftSeries, error)

	// CapValidUntil bounds a series segment at the exclusive date (split /
	// end / offboarding).
	CapValidUntil(ctx context.Context, id int64, until timezone.Date) error

	// FindOverlappingInLineage returns another segment of a split lineage that
	// is active on or after the given date. A superseded predecessor must not be
	// reopened across it.
	FindOverlappingInLineage(ctx context.Context, rootID, excludeID int64, from timezone.Date) (*StaffShiftSeries, error)
}

// staffShiftSeriesExceptionRows stores deliberately removed single
// occurrences of a series so re-plans never regenerate them.
type staffShiftSeriesExceptionRows interface {
	Create(ctx context.Context, exception *StaffShiftSeriesException) error

	// FindDatesBySeriesID returns the excepted dates of one series.
	FindDatesBySeriesID(ctx context.Context, seriesID int64) ([]timezone.Date, error)

	// RepointToSeriesFrom moves exceptions on or after from to the successor
	// series created by a split.
	RepointToSeriesFrom(ctx context.Context, fromSeriesID, toSeriesID int64, from timezone.Date) (int64, error)
}

// shiftTypeRows stores the tenant-defined Schichtarten (#1836).
type shiftTypeRows interface {
	Create(ctx context.Context, shiftType *ShiftType) error
	FindByID(ctx context.Context, id any) (*ShiftType, error)
	Update(ctx context.Context, shiftType *ShiftType) error
	Delete(ctx context.Context, id any) error

	// ListAll returns all shift types for the current tenant, ordered by name.
	ListAll(ctx context.Context) ([]*ShiftType, error)

	// CreateIfAbsent inserts a shift type unless one with the same
	// (tenant_id, LOWER(name)) already exists, so idempotent default seeding
	// cannot raise a unique violation that aborts the request's transaction.
	CreateIfAbsent(ctx context.Context, shiftType *ShiftType) (bool, error)
}

// staffDirectory resolves the staff members shifts are planned for and the
// roster the Dienstplan grid lists. FindByID takes an untyped id because the
// bound repository is the generic one.
type staffDirectory interface {
	FindByID(ctx context.Context, id any) (*usersModels.Staff, error)
	ListAllWithPerson(ctx context.Context) ([]*usersModels.Staff, error)
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.Staff, error)
}

// SeriesPeriod is the part of a School Calendar period a series is bounded
// and materialized by: its calendar days and its A/B week cycle (length and
// Monday anchor in DateLayout, empty when unset).
type SeriesPeriod struct {
	StartDate       timezone.Date
	EndDate         timezone.Date
	WeekCycleLength int
	WeekCycleAnchor string
}

// SeriesPeriodLookup resolves the period a series materializes over. A
// missing period answers with a no-rows error or the School Calendar's
// not-found sentinel.
type SeriesPeriodLookup interface {
	FindSeriesPeriod(ctx context.Context, id int64) (SeriesPeriod, error)
}

// CalendarPeriodLookup is the School Calendar read the series periods come
// from; the School Calendar capability satisfies it.
type CalendarPeriodLookup interface {
	FindCalendarPeriod(ctx context.Context, id int64) (schoolcalendar.CalendarPeriod, error)
}

// SchoolCalendarPeriods binds the series periods to the School Calendar.
func SchoolCalendarPeriods(calendar CalendarPeriodLookup) SeriesPeriodLookup {
	return schoolCalendarPeriods{calendar: calendar}
}

type schoolCalendarPeriods struct{ calendar CalendarPeriodLookup }

func (p schoolCalendarPeriods) FindSeriesPeriod(ctx context.Context, id int64) (SeriesPeriod, error) {
	period, err := p.calendar.FindCalendarPeriod(ctx, id)
	if err != nil {
		return SeriesPeriod{}, err
	}
	return SeriesPeriod{
		StartDate: timezone.Date(period.StartDate), EndDate: timezone.Date(period.EndDate),
		WeekCycleLength: period.WeekCycleLength, WeekCycleAnchor: period.WeekCycleAnchor,
	}, nil
}

// AssignmentInstanceStaffReader returns one staff member's own
// activity-instance assignments in a date range.
type AssignmentInstanceStaffReader interface {
	FindByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*timetable.InstanceStaff, error)
}

// ActivityInstanceBatchReader loads the instances an assignment set refers
// to, with the execution state of their sessions.
type ActivityInstanceBatchReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*timetable.ScheduledInstance, error)
}

// ActivityGroupBatchReader resolves the Angebot names of an assignment set.
type ActivityGroupBatchReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*timetable.Group, error)
}
