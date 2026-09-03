// Package compose wires the School Calendar module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"

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
	service := application.New(store, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	})
	return schoolcalendar.NewModule(engine{service: service}), nil
}

type engine struct{ service *application.Service }

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
	default:
		return err
	}
}
