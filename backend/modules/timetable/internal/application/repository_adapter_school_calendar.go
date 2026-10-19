package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

// The retained calendar period, closing day and dateframe repositories serve
// the legacy schedule contracts over the School Calendar owner (#2666). They
// name the schedule models, so they live with the Timetable composition that
// owns those models (#2743).

// NewPlanningCalendarPeriodUsage composes the School Calendar usage read from the
// two owners that hold calendar-period references: Enrollment (phases) and
// Timetable (planning tables), one statement each (#3124).
func NewPlanningCalendarPeriodUsage(enrollment EnrollmentPhaseQueries, planning timetable.Capability) ports.CalendarPeriodUsageRepository {
	return NewCalendarPeriodUsageRepository(enrollment, func(ctx context.Context) (map[int64]ports.CalendarPeriodReferences, error) {
		references, err := planning.CountCalendarPeriodReferences(ctx)
		if err != nil {
			return nil, err
		}
		counts := make(map[int64]ports.CalendarPeriodReferences, len(references))
		for id, entry := range references {
			counts[id] = ports.CalendarPeriodReferences(entry)
		}
		return counts, nil
	})
}

// NewCalendarPeriodRepository serves the legacy calendar period repository
// over the School Calendar owner; the reference counts come from usage
// (#2666, #3124).
func NewCalendarPeriodRepository(calendar schoolcalendar.Capability, usage ports.CalendarPeriodUsageRepository) ports.CalendarPeriodRecords {
	return newCalendarPeriodCalendarRepository(calendar, usage)
}

const opFindByID = "find by id"

// calendarError maps an owner error onto the legacy repository error shape:
// a missing row keeps the sql.ErrNoRows callers classify on, a name
// collision surfaces the domain sentinel with its driver cause intact.
func calendarError(op string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, schoolcalendar.ErrCalendarPeriodNotFound), errors.Is(err, schoolcalendar.ErrClosingDayNotFound),
		errors.Is(err, schoolcalendar.ErrDateframeNotFound):
		return calendarNotFoundError(op)
	case errors.Is(err, schoolcalendar.ErrCalendarPeriodNameConflict):
		return calendarWrapError(op, fmt.Errorf("%w: %w", scheduleModels.ErrCalendarPeriodNameConflict, err))
	default:
		return calendarWrapError(op, err)
	}
}

// The legacy models carry the repository calendar-date type; these helpers
// move YYYY-MM-DD strings in and out of it by shape, so the composition never
// names the type itself.
func setOptionalDate[D ~string](target **D, value string) {
	if value == "" {
		*target = nil
		return
	}
	date := D(value)
	*target = &date
}

func optionalDateString[D ~string](value *D) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

// --- calendar periods ---

// calendarPeriodCalendarRepository serves the legacy
// schedule.CalendarPeriodRecords over the School Calendar capability
// (#2666). schedule.calendar_periods belongs to that owner; the reference
// counts stay with the planning owner's usage repository.
type calendarPeriodCalendarRepository struct {
	CalendarPeriodOverlapListing
	calendar schoolcalendar.Capability
	ports.CalendarPeriodUsageRepository
}

var _ ports.CalendarPeriodRecords = calendarPeriodCalendarRepository{}

func newCalendarPeriodCalendarRepository(capability schoolcalendar.Capability, usage ports.CalendarPeriodUsageRepository) calendarPeriodCalendarRepository {
	repo := calendarPeriodCalendarRepository{calendar: capability, CalendarPeriodUsageRepository: usage}
	repo.Source = repo.listActiveOverlapping
	return repo
}

func calendarPeriodFieldsFromLegacy(period *scheduleModels.CalendarPeriod) schoolcalendar.CalendarPeriodFields {
	return schoolcalendar.CalendarPeriodFields{
		Name:            period.Name,
		PeriodType:      period.PeriodType,
		StartDate:       string(period.StartDate),
		EndDate:         string(period.EndDate),
		WeekCycleLength: period.WeekCycleLength,
		WeekCycleAnchor: optionalDateString(period.WeekCycleAnchor),
		IsActive:        period.IsActive,
	}
}

func applyCalendarPeriodToLegacy(target *scheduleModels.CalendarPeriod, value schoolcalendar.CalendarPeriod) {
	target.ID = value.ID
	target.CreatedAt = value.CreatedAt
	target.UpdatedAt = value.UpdatedAt
	target.SetTenantID(value.TenantID)
	target.Name = value.Name
	target.PeriodType = value.PeriodType
	SetDate(&target.StartDate, value.StartDate)
	SetDate(&target.EndDate, value.EndDate)
	target.WeekCycleLength = value.WeekCycleLength
	setOptionalDate(&target.WeekCycleAnchor, value.WeekCycleAnchor)
	target.IsActive = value.IsActive
}

func toLegacyCalendarPeriod(value schoolcalendar.CalendarPeriod) *scheduleModels.CalendarPeriod {
	period := new(scheduleModels.CalendarPeriod)
	applyCalendarPeriodToLegacy(period, value)
	return period
}

func toLegacyCalendarPeriods(values []schoolcalendar.CalendarPeriod) []*scheduleModels.CalendarPeriod {
	result := make([]*scheduleModels.CalendarPeriod, 0, len(values))
	for _, value := range values {
		result = append(result, toLegacyCalendarPeriod(value))
	}
	return result
}

// Create keeps the base repository's nil and validation gates in front of
// the owner so the messages callers assert on are unchanged.
func (r calendarPeriodCalendarRepository) Create(ctx context.Context, period *scheduleModels.CalendarPeriod) error {
	if period == nil {
		return errors.New("CalendarPeriod cannot be nil or zero value")
	}
	if err := period.Validate(); err != nil {
		return err
	}
	created, err := r.calendar.CreateCalendarPeriod(ctx, schoolcalendar.CreateCalendarPeriod{
		CalendarPeriodFields: calendarPeriodFieldsFromLegacy(period),
	})
	if err != nil {
		return calendarError("create", err)
	}
	applyCalendarPeriodToLegacy(period, created)
	return nil
}

func (r calendarPeriodCalendarRepository) FindByID(ctx context.Context, id int64) (*scheduleModels.CalendarPeriod, error) {
	periodID, err := membershipID(id)
	if err != nil {
		return nil, calendarWrapError(opFindByID, err)
	}
	value, err := r.calendar.FindCalendarPeriod(ctx, periodID)
	if err != nil {
		return nil, calendarError(opFindByID, err)
	}
	return toLegacyCalendarPeriod(value), nil
}

func (r calendarPeriodCalendarRepository) Update(ctx context.Context, period *scheduleModels.CalendarPeriod) error {
	if period == nil {
		return errors.New("CalendarPeriod cannot be nil or zero value")
	}
	if err := period.Validate(); err != nil {
		return err
	}
	updated, err := r.calendar.UpdateCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{
		ID: period.ID, CalendarPeriodFields: calendarPeriodFieldsFromLegacy(period),
	})
	if err != nil {
		return calendarError("update", err)
	}
	applyCalendarPeriodToLegacy(period, updated)
	return nil
}

// Delete stays idempotent like the base repository it replaces; a period
// still referenced by a non-nullable foreign key keeps its driver error.
func (r calendarPeriodCalendarRepository) Delete(ctx context.Context, id int64) error {
	periodID, err := membershipID(id)
	if err != nil {
		return calendarWrapError("delete", err)
	}
	err = r.calendar.DeleteCalendarPeriod(ctx, periodID)
	if errors.Is(err, schoolcalendar.ErrCalendarPeriodNotFound) {
		return nil
	}
	return calendarError("delete", err)
}

func (r calendarPeriodCalendarRepository) FindByTenantID(ctx context.Context) ([]*scheduleModels.CalendarPeriod, error) {
	values, err := r.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{})
	if err != nil {
		return nil, calendarError("find by tenant id", err)
	}
	return toLegacyCalendarPeriods(values), nil
}

func (r calendarPeriodCalendarRepository) FindActiveByTenantID(ctx context.Context) ([]*scheduleModels.CalendarPeriod, error) {
	values, err := r.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{ActiveOnly: true})
	if err != nil {
		return nil, calendarError("find active by tenant id", err)
	}
	return toLegacyCalendarPeriods(values), nil
}

// FindByName keeps the legacy contract of a nil period without error when
// the name is unknown.
func (r calendarPeriodCalendarRepository) FindByName(ctx context.Context, name string) (*scheduleModels.CalendarPeriod, error) {
	values, err := r.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{Name: name})
	if err != nil {
		return nil, calendarError("find by name", err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return toLegacyCalendarPeriod(values[0]), nil
}

func (r calendarPeriodCalendarRepository) CreateIfAbsent(ctx context.Context, period *scheduleModels.CalendarPeriod) (bool, error) {
	if period == nil {
		return false, errors.New("calendar period cannot be nil")
	}
	if err := period.Validate(); err != nil {
		return false, err
	}
	created, inserted, err := r.calendar.CreateCalendarPeriodIfAbsent(ctx, schoolcalendar.CreateCalendarPeriod{
		CalendarPeriodFields: calendarPeriodFieldsFromLegacy(period),
	})
	if err != nil {
		return false, calendarError("create if absent", err)
	}
	if inserted {
		applyCalendarPeriodToLegacy(period, created)
	}
	return inserted, nil
}

func (r calendarPeriodCalendarRepository) listActiveOverlapping(ctx context.Context, periodType, start, end string, excludeID int64) ([]*scheduleModels.CalendarPeriod, error) {
	op := "find active overlapping"
	if periodType != "" {
		op = "find active overlapping by type"
	}
	values, err := r.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{
		ActiveOnly: true, PeriodType: periodType, OverlappingFrom: start, OverlappingTo: end, ExcludeID: excludeID,
	})
	if err != nil {
		return nil, calendarError(op, err)
	}
	return toLegacyCalendarPeriods(values), nil
}

// calendarNotFoundError is the retained not-found shape: a repository error
// that matches both the base not-found sentinel and sql.ErrNoRows.
func calendarNotFoundError(op string) error {
	return &domain.StoreError{Op: op, Err: errors.Join(domain.ErrRecordNotFound, transactionruntime.ErrNoRows)}
}

// calendarWrapError wraps err in the retained repository error shape,
// preserving the chain.
func calendarWrapError(op string, err error) error {
	if err == nil {
		return nil
	}
	return &domain.StoreError{Op: op, Err: err}
}

func membershipID(id any) (int64, error) {
	switch value := id.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case int32:
		return int64(value), nil
	default:
		return 0, fmt.Errorf("unsupported id type %T", id)
	}
}
