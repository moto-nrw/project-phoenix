// Package compose wires the School Calendar module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
	// Administration resolves the owner collaborators of the period
	// administration on every call. The root binds it to a value it fills
	// once the legacy services factory exists, because the care-offering
	// guard is an Enrollment service that itself reads this calendar.
	// Optional: without it the administration runs without gate and guard
	// and the tenant holiday reads report a configuration error.
	Administration func() AdministrationRuntime
	// Today names the current calendar day in YYYY-MM-DD. Nil means the
	// Berlin calendar day, the school's day.
	Today func() string
}

// AdministrationRuntime is everything the period administration and the
// tenant holiday reads need from other owners.
type AdministrationRuntime struct {
	// FederalState resolves the tenant's federal-state ISO code for the
	// statutory-holiday reads (the operations.federal_state setting).
	// Nil: the tenant holiday reads report a configuration error.
	FederalState func(context.Context) (string, error)
	// RecurrenceGate serializes period administration against every
	// recurrence-derived write of the tenant. Nil: no gate (graphs without
	// planning); production binds the shared tenant recurrence lock.
	RecurrenceGate func(context.Context) error
	// CareOfferingGuard refuses a period change (replacement set) or
	// removal (replacement nil) a linked care offering still needs. It
	// reports the refusal as schoolcalendar.ErrCalendarPeriodRequiredByCareOffering
	// in the error chain. Nil: no guard.
	CareOfferingGuard func(ctx context.Context, periodID int64, replacement *schoolcalendar.CalendarPeriodFields) error
}

// PersistenceRuntime exposes the compose-owned ambient tenant transaction to
// legacy School Calendar persistence adapters during the migration.
type PersistenceRuntime struct {
	Database func(context.Context) bun.IDB
	TenantID func(context.Context) int64
}

// PersistenceRuntimeFor builds the migration bridge for legacy calendar
// repositories without leaking tenant-runtime imports into those adapters.
func PersistenceRuntimeFor(db *bun.DB) PersistenceRuntime {
	if db == nil {
		panic("school calendar compose: database is required")
	}
	return PersistenceRuntime{
		Database: func(ctx context.Context) bun.IDB {
			transaction, ok := tenant.TransactionFromContext(ctx)
			if !ok {
				return db
			}
			switch tx := transaction.(type) {
			case bun.Tx:
				return tx
			case *bun.Tx:
				if tx != nil {
					return tx
				}
				return db
			default:
				panic(fmt.Sprintf("school calendar compose: unsupported transaction %T", transaction))
			}
		},
		TenantID: tenant.FromContext,
	}
}

// New composes the School Calendar module. Every operation runs on the
// caller's ambient tenant transaction when one exists (tenant middleware,
// recurrence gate, scheduler loops) and otherwise on the shared connection,
// exactly like the legacy repositories' base.GetDB resolution, so RLS
// visibility and the explicit tenant predicate are unchanged.
func New(dependencies Dependencies) (*schoolcalendar.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("school calendar compose: all dependencies are required")
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID := tenant.FromContext(ctx)
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return dependencies.DB, tenantID, nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID, nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID, nil
			}
			return dependencies.DB, tenantID, nil
		default:
			return nil, 0, fmt.Errorf("school calendar postgres: unsupported transaction %T", transaction)
		}
	})
	resolve := dependencies.Administration
	if resolve == nil {
		resolve = func() AdministrationRuntime { return AdministrationRuntime{} }
	}
	service := application.New(store, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	}, administrationPorts(resolve, dependencies.Today))
	return schoolcalendar.NewModule(engine{service: service, federalState: resolve}), nil
}

// administrationPorts binds the application's administration ports to the
// runtime the resolver answers with on every call; a missing gate or guard
// is skipped, a missing clock means the Berlin calendar day.
func administrationPorts(resolve func() AdministrationRuntime, today func() string) application.Administration {
	if today == nil {
		today = berlinToday
	}
	return application.Administration{
		Transaction: writeTransaction,
		RecurrenceGate: func(ctx context.Context) error {
			gate := resolve().RecurrenceGate
			if gate == nil {
				return nil
			}
			return gate(ctx)
		},
		CareOfferingGuard: func(ctx context.Context, periodID int64, replacement *domain.CalendarPeriodFields) error {
			guard := resolve().CareOfferingGuard
			if guard == nil {
				return nil
			}
			var fields *schoolcalendar.CalendarPeriodFields
			if replacement != nil {
				value := periodFieldsToPublic(*replacement)
				fields = &value
			}
			return guard(ctx, periodID, fields)
		},
		Today: today,
	}
}

// berlinLocation is the school's wall clock; the calendar day is the day in
// Europe/Berlin, never the server's zone.
var berlinLocation = func() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic("school calendar compose: Europe/Berlin location is required: " + err.Error())
	}
	return location
}()

func berlinToday() string { return time.Now().In(berlinLocation).Format(schoolcalendar.DateLayout) }

// writeTransaction joins the ambient tenant transaction (tenant middleware,
// recurrence gate, scheduler loops) and otherwise opens one for the tenant
// in context, so the recurrence gate always holds a transaction-scoped lock.
func writeTransaction(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return fn(ctx)
	}
	return tenant.WithinCurrentTenant(ctx, fn)
}

type engine struct {
	service      *application.Service
	federalState func() AdministrationRuntime
}

// FederalState resolves the tenant's federal state through the runtime's
// settings port; an unbound runtime is a configuration error.
func (e engine) FederalState(ctx context.Context) (string, error) {
	federalState := e.federalState().FederalState
	if federalState == nil {
		return "", errors.New("school calendar: federal state resolver is not bound")
	}
	return federalState(ctx)
}

func (e engine) AddCalendarPeriod(ctx context.Context, input schoolcalendar.CreateCalendarPeriod) (schoolcalendar.CalendarPeriod, error) {
	value, err := e.service.AddCalendarPeriod(ctx, periodFieldsToDomain(input.CalendarPeriodFields))
	return periodToPublic(value), mapError(err)
}

func (e engine) ChangeCalendarPeriod(ctx context.Context, input schoolcalendar.UpdateCalendarPeriod) (schoolcalendar.CalendarPeriod, error) {
	value, err := e.service.ChangeCalendarPeriod(ctx, input.ID, periodFieldsToDomain(input.CalendarPeriodFields))
	return periodToPublic(value), mapError(err)
}

func (e engine) RemoveCalendarPeriod(ctx context.Context, id int64) error {
	return mapError(e.service.RemoveCalendarPeriod(ctx, id))
}

func (e engine) EnsureDefaultSchoolYear(ctx context.Context) ([]schoolcalendar.CalendarPeriod, bool, error) {
	values, created, err := e.service.EnsureDefaultSchoolYear(ctx, func(today string) domain.CalendarPeriodFields {
		name, start, end := schoolcalendar.DefaultSchoolYear(today)
		return domain.CalendarPeriodFields{
			Name: name, PeriodType: schoolcalendar.PeriodTypeSchoolYear, StartDate: start, EndDate: end,
			WeekCycleLength: 1, IsActive: true,
		}
	})
	if err != nil {
		return nil, false, mapError(err)
	}
	result := make([]schoolcalendar.CalendarPeriod, 0, len(values))
	for _, value := range values {
		result = append(result, periodToPublic(value))
	}
	return result, created, nil
}

func (e engine) FindCalendarPeriod(ctx context.Context, id int64) (schoolcalendar.CalendarPeriod, error) {
	value, err := e.service.FindCalendarPeriod(ctx, id)
	return periodToPublic(value), mapError(err)
}

func (e engine) ListCalendarPeriods(ctx context.Context, filter schoolcalendar.CalendarPeriodFilter) ([]schoolcalendar.CalendarPeriod, error) {
	values, err := e.service.ListCalendarPeriods(ctx, domain.CalendarPeriodFilter{
		IDs: filter.IDs, Name: filter.Name, PeriodType: filter.PeriodType, ActiveOnly: filter.ActiveOnly,
		OverlappingFrom: filter.OverlappingFrom, OverlappingTo: filter.OverlappingTo, ExcludeID: filter.ExcludeID,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]schoolcalendar.CalendarPeriod, 0, len(values))
	for _, value := range values {
		result = append(result, periodToPublic(value))
	}
	return result, nil
}

func (e engine) CreateCalendarPeriod(ctx context.Context, input schoolcalendar.CreateCalendarPeriod, ifAbsent bool) (schoolcalendar.CalendarPeriod, bool, error) {
	value, created, err := e.service.CreateCalendarPeriod(ctx, periodFieldsToDomain(input.CalendarPeriodFields), ifAbsent)
	return periodToPublic(value), created, mapError(err)
}

func (e engine) UpdateCalendarPeriod(ctx context.Context, input schoolcalendar.UpdateCalendarPeriod) (schoolcalendar.CalendarPeriod, error) {
	value, err := e.service.UpdateCalendarPeriod(ctx, input.ID, periodFieldsToDomain(input.CalendarPeriodFields))
	return periodToPublic(value), mapError(err)
}

func (e engine) DeleteCalendarPeriod(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteCalendarPeriod(ctx, id))
}

func (e engine) FindClosingDay(ctx context.Context, id int64) (schoolcalendar.ClosingDay, error) {
	value, err := e.service.FindClosingDay(ctx, id)
	return closingDayToPublic(value), mapError(err)
}

func (e engine) ListClosingDays(ctx context.Context, filter schoolcalendar.ClosingDayFilter) ([]schoolcalendar.ClosingDay, error) {
	values, err := e.service.ListClosingDays(ctx, domain.ClosingDayFilter{
		IDs: filter.IDs, OverlappingFrom: filter.OverlappingFrom, OverlappingTo: filter.OverlappingTo,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]schoolcalendar.ClosingDay, 0, len(values))
	for _, value := range values {
		result = append(result, closingDayToPublic(value))
	}
	return result, nil
}

func (e engine) CreateClosingDay(ctx context.Context, input schoolcalendar.CreateClosingDay) (schoolcalendar.ClosingDay, error) {
	value, err := e.service.CreateClosingDay(ctx, closingDayFieldsToDomain(input.ClosingDayFields))
	return closingDayToPublic(value), mapError(err)
}

func (e engine) UpdateClosingDay(ctx context.Context, input schoolcalendar.UpdateClosingDay) (schoolcalendar.ClosingDay, error) {
	value, err := e.service.UpdateClosingDay(ctx, input.ID, closingDayFieldsToDomain(input.ClosingDayFields))
	return closingDayToPublic(value), mapError(err)
}

func (e engine) DeleteClosingDay(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteClosingDay(ctx, id))
}

func (e engine) FindDateframe(ctx context.Context, id int64) (schoolcalendar.Dateframe, error) {
	value, err := e.service.FindDateframe(ctx, id)
	return dateframeToPublic(value), mapError(err)
}

func (e engine) ListDateframes(ctx context.Context, filter schoolcalendar.DateframeFilter) ([]schoolcalendar.Dateframe, error) {
	sort := make([]domain.DateframeSort, 0, len(filter.Sort))
	for _, field := range filter.Sort {
		sort = append(sort, domain.DateframeSort{Field: field.Field, Descending: field.Descending})
	}
	values, err := e.service.ListDateframes(ctx, domain.DateframeFilter{
		IDs: filter.IDs, Name: filter.Name, NameFold: filter.NameFold, NamePattern: filter.NamePattern, Contains: filter.Contains,
		OverlappingFrom: filter.OverlappingFrom, OverlappingTo: filter.OverlappingTo,
		Sort: sort, Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]schoolcalendar.Dateframe, 0, len(values))
	for _, value := range values {
		result = append(result, dateframeToPublic(value))
	}
	return result, nil
}

func (e engine) CreateDateframe(ctx context.Context, input schoolcalendar.CreateDateframe) (schoolcalendar.Dateframe, error) {
	value, err := e.service.CreateDateframe(ctx, dateframeFieldsToDomain(input.DateframeFields))
	return dateframeToPublic(value), mapError(err)
}

func (e engine) UpdateDateframe(ctx context.Context, input schoolcalendar.UpdateDateframe) (schoolcalendar.Dateframe, error) {
	value, err := e.service.UpdateDateframe(ctx, input.ID, dateframeFieldsToDomain(input.DateframeFields))
	return dateframeToPublic(value), mapError(err)
}

func (e engine) DeleteDateframe(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteDateframe(ctx, id))
}

func (e engine) ValidHolidayRegion(region string) bool {
	return e.service.ValidHolidayRegion(region)
}

func (e engine) ListHolidays(ctx context.Context, region, from, to string) ([]schoolcalendar.Holiday, error) {
	values, err := e.service.ListHolidays(ctx, region, from, to)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]schoolcalendar.Holiday, 0, len(values))
	for _, value := range values {
		result = append(result, schoolcalendar.Holiday{Date: value.Date, Name: value.Name})
	}
	return result, nil
}

func (e engine) HolidayDates(ctx context.Context, region, from, to string) (map[string]bool, error) {
	values, err := e.service.HolidayDates(ctx, region, from, to)
	return values, mapError(err)
}

func (e engine) RenderCalendar(ctx context.Context, name string, events []schoolcalendar.CalendarEvent) (string, error) {
	values := make([]domain.CalendarEvent, 0, len(events))
	for _, event := range events {
		values = append(values, calendarEventToDomain(event))
	}
	return e.service.RenderCalendar(ctx, name, values)
}

func (e engine) RenderCalendarObject(ctx context.Context, event schoolcalendar.CalendarEvent) (string, error) {
	return e.service.RenderCalendarObject(ctx, calendarEventToDomain(event))
}

func calendarEventToDomain(event schoolcalendar.CalendarEvent) domain.CalendarEvent {
	var recurrence *domain.CalendarRecurrence
	if event.Recurrence != nil {
		recurrence = &domain.CalendarRecurrence{
			Frequency: event.Recurrence.Frequency, Interval: event.Recurrence.Interval,
			Weekdays: event.Recurrence.Weekdays, MonthDays: event.Recurrence.MonthDays,
			Until: event.Recurrence.Until, Count: event.Recurrence.Count,
		}
	}
	return domain.CalendarEvent{
		UID: event.UID, Summary: event.Summary, Description: event.Description, Location: event.Location,
		StartDate: event.StartDate, EndDate: event.EndDate, StartClock: event.StartClock, EndClock: event.EndClock,
		AllDay: event.AllDay, Cancelled: event.Cancelled, Sequence: event.Sequence, Stamp: event.Stamp,
		LastModified: event.LastModified, Recurrence: recurrence, ExDates: event.ExDates,
	}
}

func dateframeFieldsToDomain(fields schoolcalendar.DateframeFields) domain.DateframeFields {
	return domain.DateframeFields{
		StartDate: fields.StartDate, EndDate: fields.EndDate, Name: fields.Name, Description: fields.Description,
	}
}

func dateframeToPublic(value domain.Dateframe) schoolcalendar.Dateframe {
	return schoolcalendar.Dateframe{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StartDate: value.StartDate, EndDate: value.EndDate, Name: value.Name, Description: value.Description,
	}
}

func periodFieldsToDomain(fields schoolcalendar.CalendarPeriodFields) domain.CalendarPeriodFields {
	return domain.CalendarPeriodFields{
		Name: fields.Name, PeriodType: fields.PeriodType, StartDate: fields.StartDate, EndDate: fields.EndDate,
		WeekCycleLength: fields.WeekCycleLength, WeekCycleAnchor: fields.WeekCycleAnchor, IsActive: fields.IsActive,
	}
}

func periodFieldsToPublic(fields domain.CalendarPeriodFields) schoolcalendar.CalendarPeriodFields {
	return schoolcalendar.CalendarPeriodFields{
		Name: fields.Name, PeriodType: fields.PeriodType, StartDate: fields.StartDate, EndDate: fields.EndDate,
		WeekCycleLength: fields.WeekCycleLength, WeekCycleAnchor: fields.WeekCycleAnchor, IsActive: fields.IsActive,
	}
}

func periodToPublic(value domain.CalendarPeriod) schoolcalendar.CalendarPeriod {
	return schoolcalendar.CalendarPeriod{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Name: value.Name, PeriodType: value.PeriodType, StartDate: value.StartDate, EndDate: value.EndDate,
		WeekCycleLength: value.WeekCycleLength, WeekCycleAnchor: value.WeekCycleAnchor, IsActive: value.IsActive,
	}
}

func closingDayFieldsToDomain(fields schoolcalendar.ClosingDayFields) domain.ClosingDayFields {
	return domain.ClosingDayFields{StartDate: fields.StartDate, EndDate: fields.EndDate, Reason: fields.Reason}
}

func closingDayToPublic(value domain.ClosingDay) schoolcalendar.ClosingDay {
	return schoolcalendar.ClosingDay{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StartDate: value.StartDate, EndDate: value.EndDate, Reason: value.Reason,
	}
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrCalendarPeriodNotFound):
		return schoolcalendar.ErrCalendarPeriodNotFound
	case errors.Is(err, domain.ErrClosingDayNotFound):
		return schoolcalendar.ErrClosingDayNotFound
	case errors.Is(err, domain.ErrDateframeNotFound):
		return schoolcalendar.ErrDateframeNotFound
	case errors.Is(err, domain.ErrCalendarPeriodNameConflict):
		// The cause stays in the chain on purpose: callers that classify the
		// collision by constraint name keep working.
		return fmt.Errorf("%w: %w", schoolcalendar.ErrCalendarPeriodNameConflict, err)
	case errors.Is(err, domain.ErrCalendarPeriodOverlapConflict):
		var overlap *domain.CalendarPeriodOverlapError
		if errors.As(err, &overlap) {
			overlaps := make([]schoolcalendar.CalendarPeriod, 0, len(overlap.Overlaps))
			for _, value := range overlap.Overlaps {
				overlaps = append(overlaps, periodToPublic(value))
			}
			return &schoolcalendar.CalendarPeriodOverlapError{Overlaps: overlaps}
		}
		return schoolcalendar.ErrCalendarPeriodOverlapConflict
	case errors.Is(err, domain.ErrCalendarPeriodRequiredByCareOffering):
		return fmt.Errorf("%w: %w", schoolcalendar.ErrCalendarPeriodRequiredByCareOffering, err)
	default:
		return err
	}
}
