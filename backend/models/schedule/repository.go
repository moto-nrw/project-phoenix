package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

// StaffShiftRepository is the data-access boundary for planned staff shifts
// (Dienstplan). CRUD comes from the generic repository; the finders below
// are the date-scoped lookups the Dienstplan grid and the auto-checkout job
// need (mirrors the MealPlanEntryRepository shape).
type StaffShiftRepository interface {
	Create(ctx context.Context, shift *StaffShift) error
	FindByID(ctx context.Context, id any) (*StaffShift, error)
	Update(ctx context.Context, shift *StaffShift) error
	Delete(ctx context.Context, id any) error

	// List and UpdateColumns surface the embedded generic repository: the
	// #1843 sick cascade filters by sick_absence_id and stamps/clears that
	// single column without whole-model writes.
	List(ctx context.Context, filters map[string]any) ([]*StaffShift, error)
	UpdateColumns(ctx context.Context, shift *StaffShift, columns ...string) (int64, error)

	// FindByDateRange returns all shifts with start <= date <= end for the
	// current tenant, ordered by date, staff, start time.
	FindByDateRange(ctx context.Context, start, end timezone.Date) ([]*StaffShift, error)

	// FindByStaffAndDateRange returns one staff member's shifts in the range.
	FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end timezone.Date) ([]*StaffShift, error)

	// FindByStaffIDsAndDateRange is FindByStaffAndDateRange batched over many
	// staff members, keyed by staff ID.
	FindByStaffIDsAndDateRange(ctx context.Context, staffIDs []int64, start, end timezone.Date) (map[int64][]*StaffShift, error)

	// FindByStaffIDsAndDate returns the shifts of the given staff members on
	// one date (batch lookup for the auto-checkout job).
	FindByStaffIDsAndDate(ctx context.Context, staffIDs []int64, date timezone.Date) ([]*StaffShift, error)

	// FindByOriginShiftID returns every replacement shift covering the given
	// origin (its cover set). Used when re-planning or reactivating a cancelled
	// shift so its replacements can be resolved atomically (#1841).
	FindByOriginShiftID(ctx context.Context, originShiftID int64) ([]*StaffShift, error)

	// FindByStaffIDsAndDates returns only the shifts relevant to a batched
	// hypothetical coverage probe.
	FindByStaffIDsAndDates(ctx context.Context, staffIDs []int64, dates []timezone.Date) ([]*StaffShift, error)

	// FindUsedCalendarWeeks returns the Monday of every ISO week containing at
	// least one tenant shift in the inclusive range.
	FindUsedCalendarWeeks(ctx context.Context, start, end timezone.Date) ([]timezone.Date, error)

	// DeleteUpcomingByStaffID removes planned shifts on or after from. Past
	// shifts stay as history. Used by staff offboarding.
	DeleteUpcomingByStaffID(ctx context.Context, staffID int64, from timezone.Date) (int64, error)

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

// StaffShiftSeriesRepository is the data-access boundary for recurring shift
// series (#1889). CRUD comes from the generic repository (mirrors the
// StaffShiftRepository shape).
type StaffShiftSeriesRepository interface {
	Create(ctx context.Context, series *StaffShiftSeries) error
	FindByID(ctx context.Context, id any) (*StaffShiftSeries, error)
	Update(ctx context.Context, series *StaffShiftSeries) error
	Delete(ctx context.Context, id any) error

	// CapValidUntil bounds a series segment at the exclusive date (split /
	// end / offboarding).
	CapValidUntil(ctx context.Context, id int64, until timezone.Date) error

	// CapAllByStaffID bounds every series segment of one staff member at the
	// exclusive date (staff offboarding).
	CapAllByStaffID(ctx context.Context, staffID int64, until timezone.Date) (int64, error)

	// FindOverlappingInLineage returns another segment of a split lineage that
	// is active on or after the given date. A superseded predecessor must not be
	// reopened across it.
	FindOverlappingInLineage(ctx context.Context, rootID, excludeID int64, from timezone.Date) (*StaffShiftSeries, error)
}

// StaffShiftSeriesExceptionRepository stores deliberately removed single
// occurrences of a series so re-plans never regenerate them.
type StaffShiftSeriesExceptionRepository interface {
	Create(ctx context.Context, exception *StaffShiftSeriesException) error

	// FindDatesBySeriesID returns the excepted dates of one series.
	FindDatesBySeriesID(ctx context.Context, seriesID int64) ([]timezone.Date, error)

	// RepointToSeriesFrom moves exceptions on or after from to the successor
	// series created by a split.
	RepointToSeriesFrom(ctx context.Context, fromSeriesID, toSeriesID int64, from timezone.Date) (int64, error)
}

// ShiftTypeRepository is the data-access boundary for tenant-defined shift
// types (Schichtarten, #1836). CRUD comes from the generic repository; ListAll
// returns every type for the current tenant (active and inactive), ordered by
// name, for the management UI. The Dienstplan shift picker filters to active.
type ShiftTypeRepository interface {
	repository[*ShiftType]

	// ListAll returns all shift types for the current tenant, ordered by name.
	ListAll(ctx context.Context) ([]*ShiftType, error)

	// CreateIfAbsent inserts a shift type unless one with the same
	// (tenant_id, LOWER(name)) already exists (uniq_shift_types_tenant_name).
	// It returns true when a new row was inserted. Used by idempotent default
	// seeding so a concurrent seed cannot raise a unique violation that aborts
	// the request's tenant transaction.
	CreateIfAbsent(ctx context.Context, shiftType *ShiftType) (bool, error)
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
	crudRepository[*ClassArrivalException]

	// FindByClassesAndDateRange returns the exceptions of the given classes
	// with from <= date <= to, matched case-insensitively on the normalized
	// class and ordered by date.
	FindByClassesAndDateRange(ctx context.Context, classes []string, from, to timezone.Date) ([]*ClassArrivalException, error)

	// Upsert stores the exception of one class and date, replacing what was
	// there.
	Upsert(ctx context.Context, row *ClassArrivalException) error

	// DeleteByClassAndDate removes the exception of one class and date and
	// reports whether a row existed.
	DeleteByClassAndDate(ctx context.Context, schoolClass string, date timezone.Date) (bool, error)
}
