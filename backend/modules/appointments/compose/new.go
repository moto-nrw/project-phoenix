// Package compose wires the Appointments module over the shared tenant
// UnitOfWork and Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

func New(dependencies Dependencies) (*appointments.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("appointments compose: all dependencies are required")
	}
	store := postgres.New(database(dependencies.DB))
	service := application.New(store, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	})
	return appointments.NewModule(engine{service: service}), nil
}

func database(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID := tenant.FromContext(ctx)
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return db, tenantID, nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID, nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID, nil
			}
			return db, tenantID, nil
		default:
			return nil, 0, fmt.Errorf("appointments postgres: unsupported transaction %T", transaction)
		}
	}
}

type engine struct{ service *application.Service }

func (e engine) FindAppointment(ctx context.Context, id int64) (*appointments.Appointment, error) {
	value, err := e.service.FindAppointment(ctx, id, false)
	return appointmentToPublic(value), mapError(err)
}

func (e engine) FindAppointmentForUpdate(ctx context.Context, id int64) (*appointments.Appointment, error) {
	value, err := e.service.FindAppointment(ctx, id, true)
	return appointmentToPublic(value), mapError(err)
}

func (e engine) FindReminderCandidateForUpdate(ctx context.Context, id int64) (*appointments.Appointment, error) {
	value, found, err := e.service.FindReminderCandidateForUpdate(ctx, id)
	if err != nil || !found {
		return nil, mapError(err)
	}
	return appointmentToPublic(value), nil
}

func (e engine) ListAppointmentsVisibleToStaff(ctx context.Context, staffID int64, from, to appointments.Date) ([]*appointments.Appointment, error) {
	values, err := e.service.ListAppointmentsVisibleToStaff(ctx, staffID, domain.Date(from), domain.Date(to))
	return appointmentsToPublic(values), mapError(err)
}

func (e engine) ListStaffCancellationTombstones(ctx context.Context, staffID int64, since time.Time) ([]*appointments.Appointment, error) {
	values, err := e.service.ListStaffCancellationTombstones(ctx, staffID, since)
	return appointmentsToPublic(values), mapError(err)
}

func (e engine) ListAppointmentsVisibleToGuardians(ctx context.Context, guardianIDs, studentIDs []int64, from, to appointments.Date) ([]*appointments.Appointment, error) {
	values, err := e.service.ListAppointmentsVisibleToGuardians(ctx, guardianIDs, studentIDs, domain.Date(from), domain.Date(to))
	return appointmentsToPublic(values), mapError(err)
}

func (e engine) ListGuardianCancellationTombstones(ctx context.Context, guardianIDs, studentIDs []int64, since time.Time) ([]*appointments.Appointment, error) {
	values, err := e.service.ListGuardianCancellationTombstones(ctx, guardianIDs, studentIDs, since)
	return appointmentsToPublic(values), mapError(err)
}

func (e engine) ListGuardianReminderCandidates(ctx context.Context, from, to appointments.Date) ([]*appointments.Appointment, error) {
	values, err := e.service.ListGuardianReminderCandidates(ctx, domain.Date(from), domain.Date(to))
	return appointmentsToPublic(values), mapError(err)
}

func (e engine) FindAppointmentTargets(ctx context.Context, appointmentID int64) ([]*appointments.AppointmentTarget, error) {
	values, err := e.service.FindAppointmentTargets(ctx, appointmentID)
	return targetsToPublic(values), mapError(err)
}

func (e engine) CreateAppointment(ctx context.Context, input appointments.CreateAppointment) (result *appointments.Appointment, targets []*appointments.AppointmentTarget, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		value, targetValues, commandErr := e.service.CreateAppointment(txCtx, fieldsToDomain(input.AppointmentFields), targetFieldsToDomain(input.Targets))
		result = appointmentToPublic(value)
		targets = targetsToPublic(targetValues)
		return commandErr
	})
	if err != nil {
		return nil, nil, mapError(err)
	}
	return result, targets, mapError(err)
}

func (e engine) UpdateAppointment(ctx context.Context, input appointments.UpdateAppointment) (result *appointments.Appointment, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		value, commandErr := e.service.UpdateAppointment(txCtx, input.ID, fieldsToDomain(input.AppointmentFields))
		result = appointmentToPublic(value)
		return commandErr
	})
	if err != nil {
		return nil, mapError(err)
	}
	return result, mapError(err)
}

func (e engine) DeleteAppointment(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.DeleteAppointment(txCtx, id)
	}))
}

func (e engine) BumpAppointmentRevision(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.BumpAppointmentRevision(txCtx, id)
	}))
}

func (e engine) CancelAppointment(ctx context.Context, id int64) (transitioned bool, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		var commandErr error
		transitioned, commandErr = e.service.CancelAppointment(txCtx, id)
		return commandErr
	})
	if err != nil {
		return false, mapError(err)
	}
	return transitioned, mapError(err)
}

func (e engine) SoftDeleteAppointment(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.SoftDeleteAppointment(txCtx, id)
	}))
}

func (e engine) DeleteFeedTombstonesBefore(ctx context.Context, before time.Time) (rows int, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		var commandErr error
		rows, commandErr = e.service.DeleteFeedTombstonesBefore(txCtx, before)
		return commandErr
	})
	if err != nil {
		return 0, mapError(err)
	}
	return rows, mapError(err)
}

func (e engine) ReplaceAppointmentTargets(ctx context.Context, appointmentID int64, values []appointments.AppointmentTargetFields) (result []*appointments.AppointmentTarget, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		targets, commandErr := e.service.ReplaceAppointmentTargets(txCtx, appointmentID, targetFieldsToDomain(values))
		result = targetsToPublic(targets)
		return commandErr
	})
	if err != nil {
		return nil, mapError(err)
	}
	return result, mapError(err)
}

func (e engine) withinTenant(ctx context.Context, command func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return command(ctx)
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return errors.New("appointments: tenant is required for commands")
	}
	return tenant.WithinTenant(ctx, tenantID, command)
}

func fieldsToDomain(fields appointments.AppointmentFields) domain.AppointmentFields {
	return domain.AppointmentFields{
		OrganizerStaffID: fields.OrganizerStaffID, Title: fields.Title, Description: fields.Description,
		Location: fields.Location, StartDate: domain.Date(fields.StartDate), EndDate: domain.Date(fields.EndDate),
		StartTime: fields.StartTime, EndTime: fields.EndTime, AllDay: fields.AllDay,
		DeliveryMode: fields.DeliveryMode, OverviewVisibility: fields.OverviewVisibility,
		NotifyGuardians: fields.NotifyGuardians,
	}
}

func targetFieldsToDomain(values []appointments.AppointmentTargetFields) []domain.AppointmentTargetFields {
	result := make([]domain.AppointmentTargetFields, 0, len(values))
	for _, value := range values {
		result = append(result, domain.AppointmentTargetFields{
			TargetType: value.TargetType, TargetID: value.TargetID, TargetValue: value.TargetValue,
		})
	}
	return result
}

func appointmentToPublic(value domain.Appointment) *appointments.Appointment {
	if value.ID == 0 {
		return nil
	}
	return &appointments.Appointment{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		OrganizerStaffID: value.OrganizerStaffID, Title: value.Title, Description: value.Description,
		Location: value.Location, StartDate: appointments.Date(value.StartDate), EndDate: appointments.Date(value.EndDate),
		StartTime: value.StartTime, EndTime: value.EndTime, AllDay: value.AllDay,
		DeliveryMode: value.DeliveryMode, OverviewVisibility: value.OverviewVisibility,
		CancelledAt: value.CancelledAt, DeletedAt: value.DeletedAt,
		NotifyGuardians: value.NotifyGuardians, Revision: value.Revision,
	}
}

func appointmentsToPublic(values []domain.Appointment) []*appointments.Appointment {
	result := make([]*appointments.Appointment, 0, len(values))
	for _, value := range values {
		result = append(result, appointmentToPublic(value))
	}
	return result
}

func targetsToPublic(values []domain.AppointmentTarget) []*appointments.AppointmentTarget {
	result := make([]*appointments.AppointmentTarget, 0, len(values))
	for _, value := range values {
		result = append(result, &appointments.AppointmentTarget{
			ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
			AppointmentID: value.AppointmentID, TargetType: value.TargetType,
			TargetID: value.TargetID, TargetValue: value.TargetValue,
		})
	}
	return result
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrAppointmentNotFound):
		return fmt.Errorf("%w: %w", appointments.ErrAppointmentNotFound, err)
	case errors.Is(err, domain.ErrAppointmentLifecycleConflict):
		return fmt.Errorf("%w: %w", appointments.ErrAppointmentLifecycleConflict, err)
	default:
		return err
	}
}
