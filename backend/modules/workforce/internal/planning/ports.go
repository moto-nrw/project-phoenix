package planning

import (
	"context"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// The persistence ports of the Dienstplan services. They are consumer-owned:
// each carries exactly the operations the services here call, and
// modules/workforce/compose binds them to the Workforce capability rows
// (#3418). The method shapes are the ones the deleted models/schedule
// Dienstplan contracts had, so the row adapters and the package's test
// doubles satisfy them unchanged.

// staffShiftRows reads and writes the concrete schedule.staff_shifts rows a
// planned shift is stored as.
type staffShiftRows interface {
	Create(ctx context.Context, shift *scheduleModels.StaffShift) error
	FindByID(ctx context.Context, id any) (*scheduleModels.StaffShift, error)
	Update(ctx context.Context, shift *scheduleModels.StaffShift) error
	Delete(ctx context.Context, id any) error

	// FindByDateRange returns all shifts with start <= date <= end for the
	// current tenant, ordered by date, staff, start time.
	FindByDateRange(ctx context.Context, start, end scheduleModels.Date) ([]*scheduleModels.StaffShift, error)

	// FindByStaffAndDateRange returns one staff member's shifts in the range.
	FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end scheduleModels.Date) ([]*scheduleModels.StaffShift, error)

	// FindByOriginShiftID returns every replacement shift covering the given
	// origin (its cover set), so a cancellation resolves them atomically (#1841).
	FindByOriginShiftID(ctx context.Context, originShiftID int64) ([]*scheduleModels.StaffShift, error)

	// BulkCreate inserts all shifts in one multi-row statement (series
	// materialization, #1889).
	BulkCreate(ctx context.Context, shifts []*scheduleModels.StaffShift) error

	// DeleteNonDetachedBySeriesFrom removes a series' regenerable rows on or
	// after from. Detached rows ("Nur diese Woche" edits) survive.
	DeleteNonDetachedBySeriesFrom(ctx context.Context, seriesID int64, from scheduleModels.Date) (int64, error)

	// RepointDetachedSeriesFrom moves a series' detached rows on or after
	// from to the successor series created by a split.
	RepointDetachedSeriesFrom(ctx context.Context, fromSeriesID, toSeriesID int64, from scheduleModels.Date) (int64, error)
}

// staffShiftSeriesRows stores the recurrence rules concrete shifts are
// materialized from (#1889).
type staffShiftSeriesRows interface {
	Create(ctx context.Context, series *scheduleModels.StaffShiftSeries) error
	FindByID(ctx context.Context, id any) (*scheduleModels.StaffShiftSeries, error)

	// CapValidUntil bounds a series segment at the exclusive date (split /
	// end / offboarding).
	CapValidUntil(ctx context.Context, id int64, until scheduleModels.Date) error

	// FindOverlappingInLineage returns another segment of a split lineage that
	// is active on or after the given date. A superseded predecessor must not be
	// reopened across it.
	FindOverlappingInLineage(ctx context.Context, rootID, excludeID int64, from scheduleModels.Date) (*scheduleModels.StaffShiftSeries, error)
}

// staffShiftSeriesExceptionRows stores deliberately removed single
// occurrences of a series so re-plans never regenerate them.
type staffShiftSeriesExceptionRows interface {
	Create(ctx context.Context, exception *scheduleModels.StaffShiftSeriesException) error

	// FindDatesBySeriesID returns the excepted dates of one series.
	FindDatesBySeriesID(ctx context.Context, seriesID int64) ([]scheduleModels.Date, error)

	// RepointToSeriesFrom moves exceptions on or after from to the successor
	// series created by a split.
	RepointToSeriesFrom(ctx context.Context, fromSeriesID, toSeriesID int64, from scheduleModels.Date) (int64, error)
}

// shiftTypeRows stores the tenant-defined Schichtarten (#1836).
type shiftTypeRows interface {
	Create(ctx context.Context, shiftType *scheduleModels.ShiftType) error
	FindByID(ctx context.Context, id any) (*scheduleModels.ShiftType, error)
	Update(ctx context.Context, shiftType *scheduleModels.ShiftType) error
	Delete(ctx context.Context, id any) error

	// ListAll returns all shift types for the current tenant, ordered by name.
	ListAll(ctx context.Context) ([]*scheduleModels.ShiftType, error)

	// CreateIfAbsent inserts a shift type unless one with the same
	// (tenant_id, LOWER(name)) already exists, so idempotent default seeding
	// cannot raise a unique violation that aborts the request's transaction.
	CreateIfAbsent(ctx context.Context, shiftType *scheduleModels.ShiftType) (bool, error)
}

// staffDirectory resolves the staff members shifts are planned for and the
// roster the Dienstplan grid lists. FindByID takes an untyped id because the
// bound repository is the generic one.
type staffDirectory interface {
	FindByID(ctx context.Context, id any) (*usersModels.Staff, error)
	ListAllWithPerson(ctx context.Context) ([]*usersModels.Staff, error)
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.Staff, error)
}

// calendarPeriodLookup resolves the planning period a series materializes
// over (its week cycle decides which weeks an A/B rhythm produces).
type calendarPeriodLookup interface {
	FindByID(ctx context.Context, id any) (*scheduleModels.CalendarPeriod, error)
}

// assignmentInstanceStaffReader returns one staff member's own
// activity-instance assignments in a date range.
type assignmentInstanceStaffReader interface {
	FindByStaffAndDateRange(ctx context.Context, staffID int64, from, to scheduleModels.Date) ([]*scheduleModels.InstanceStaff, error)
}

// activityInstanceBatchReader loads the instances an assignment set refers to.
type activityInstanceBatchReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*scheduleModels.ActivityInstance, error)
}

// activityGroupBatchReader resolves the Angebot names of an assignment set.
type activityGroupBatchReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*activitiesModels.Group, error)
}
