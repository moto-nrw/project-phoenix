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

func (e engine) FindReminderCandidatesForUpdate(ctx context.Context, ids []int64) ([]*appointments.Appointment, error) {
	values, err := e.service.FindReminderCandidatesForUpdate(ctx, ids)
	return appointmentsToPublic(values), mapError(err)
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

func (e engine) FindRecurrenceRule(ctx context.Context, appointmentID int64) (*appointments.RecurrenceRule, error) {
	value, found, err := e.service.FindRecurrenceRule(ctx, appointmentID)
	if err != nil || !found {
		return nil, mapError(err)
	}
	return recurrenceRuleToPublic(value), nil
}

func (e engine) FindRecurrenceRules(ctx context.Context, appointmentIDs []int64) ([]*appointments.RecurrenceRule, error) {
	values, err := e.service.FindRecurrenceRules(ctx, appointmentIDs)
	return recurrenceRulesToPublic(values), mapError(err)
}

func (e engine) FindOccurrenceOverrides(ctx context.Context, appointmentIDs []int64, dates []appointments.Date) ([]*appointments.AppointmentOccurrenceOverride, error) {
	values, err := e.service.FindOccurrenceOverrides(ctx, appointmentIDs, datesToDomain(dates))
	return occurrenceOverridesToPublic(values), mapError(err)
}

func (e engine) FindOccurrenceOverridesByStartDates(ctx context.Context, appointmentIDs []int64, dates []appointments.Date) ([]*appointments.AppointmentOccurrenceOverride, error) {
	values, err := e.service.FindOccurrenceOverridesByStartDates(ctx, appointmentIDs, datesToDomain(dates))
	return occurrenceOverridesToPublic(values), mapError(err)
}

func (e engine) FindCancelledOccurrenceOverrides(ctx context.Context, appointmentIDs []int64) ([]*appointments.AppointmentOccurrenceOverride, error) {
	values, err := e.service.FindCancelledOccurrenceOverrides(ctx, appointmentIDs)
	return occurrenceOverridesToPublic(values), mapError(err)
}

func (e engine) FindAppointmentRecipient(ctx context.Context, recipientID int64) (*appointments.AppointmentRecipient, error) {
	value, found, err := e.service.FindAppointmentRecipient(ctx, recipientID)
	if err != nil || !found {
		return nil, mapError(err)
	}
	return appointmentRecipientToPublic(value), nil
}

func (e engine) FindAppointmentRecipients(ctx context.Context, appointmentID int64) ([]*appointments.AppointmentRecipient, error) {
	values, err := e.service.FindAppointmentRecipients(ctx, []int64{appointmentID})
	return appointmentRecipientsToPublic(values), mapError(err)
}

func (e engine) FindAppointmentRecipientsByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*appointments.AppointmentRecipient, error) {
	values, err := e.service.FindAppointmentRecipients(ctx, appointmentIDs)
	return appointmentRecipientsToPublic(values), mapError(err)
}

func (e engine) FindAppointmentRecipientStudents(ctx context.Context, recipientIDs []int64) ([]*appointments.AppointmentRecipientStudent, error) {
	values, err := e.service.FindAppointmentRecipientStudents(ctx, recipientIDs)
	return appointmentRecipientStudentsToPublic(values), mapError(err)
}

func (e engine) CountAppointmentRecipientStudents(ctx context.Context, studentID int64) (int, error) {
	value, err := e.service.CountAppointmentRecipientStudents(ctx, studentID)
	return value, mapError(err)
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

func (e engine) CreateRecurrenceRule(ctx context.Context, rule *appointments.RecurrenceRule) error {
	err := e.withinTenant(ctx, func(txCtx context.Context) error {
		value, commandErr := e.service.CreateRecurrenceRule(txCtx, recurrenceRuleToDomain(rule))
		if commandErr == nil {
			*rule = *recurrenceRuleToPublic(value)
		}
		return commandErr
	})
	return mapError(err)
}

func (e engine) DeleteRecurrenceRule(ctx context.Context, appointmentID int64) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.DeleteRecurrenceRule(txCtx, appointmentID)
	}))
}

func (e engine) CreateOccurrenceOverride(ctx context.Context, override *appointments.AppointmentOccurrenceOverride) error {
	err := e.withinTenant(ctx, func(txCtx context.Context) error {
		value, commandErr := e.service.CreateOccurrenceOverride(txCtx, occurrenceOverrideToDomain(override))
		if commandErr == nil {
			*override = *occurrenceOverrideToPublic(value)
		}
		return commandErr
	})
	return mapError(err)
}

func (e engine) DeleteOccurrenceOverrides(ctx context.Context, appointmentID int64) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.DeleteOccurrenceOverrides(txCtx, appointmentID)
	}))
}

func (e engine) CancelAppointmentOccurrence(ctx context.Context, appointmentID int64, occurrenceDate appointments.Date) (transitioned bool, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		var commandErr error
		transitioned, commandErr = e.service.CancelAppointmentOccurrence(txCtx, appointmentID, domain.Date(occurrenceDate))
		return commandErr
	})
	if err != nil {
		return false, mapError(err)
	}
	return transitioned, nil
}

func (e engine) CreateAppointmentRecipients(ctx context.Context, appointmentID int64, fields []appointments.AppointmentRecipientFields) (recipients []*appointments.AppointmentRecipient, links []*appointments.AppointmentRecipientStudent, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		values, linkValues, commandErr := e.service.CreateAppointmentRecipients(txCtx, appointmentID, recipientFieldsToDomain(fields))
		recipients = appointmentRecipientsToPublic(values)
		links = appointmentRecipientStudentsToPublic(linkValues)
		return commandErr
	})
	if err != nil {
		return nil, nil, mapError(err)
	}
	return recipients, links, nil
}

func (e engine) UpdateAppointmentRecipientResponse(ctx context.Context, recipientID int64, status string) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.UpdateAppointmentRecipientResponse(txCtx, recipientID, status)
	}))
}

func (e engine) ClaimReminderPushDelivery(ctx context.Context, appointmentID int64, revision int, occurrenceDate appointments.Date, guardianProfileID int64) (claimed bool, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		var commandErr error
		claimed, commandErr = e.service.ClaimReminderPushDelivery(txCtx, appointmentID, revision, domain.Date(occurrenceDate), guardianProfileID)
		return commandErr
	})
	if err != nil {
		return false, mapError(err)
	}
	return claimed, nil
}

func (e engine) ReleaseReminderPushDelivery(ctx context.Context, appointmentID int64, revision int, occurrenceDate appointments.Date, guardianProfileID int64) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.ReleaseReminderPushDelivery(txCtx, appointmentID, revision, domain.Date(occurrenceDate), guardianProfileID)
	}))
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

func recipientFieldsToDomain(values []appointments.AppointmentRecipientFields) []domain.AppointmentRecipientFields {
	result := make([]domain.AppointmentRecipientFields, 0, len(values))
	for _, value := range values {
		result = append(result, domain.AppointmentRecipientFields{
			RecipientType: value.RecipientType, StaffID: value.StaffID,
			GuardianProfileID: value.GuardianProfileID, Status: value.Status,
			StudentIDs: value.StudentIDs,
		})
	}
	return result
}

func datesToDomain(values []appointments.Date) []domain.Date {
	result := make([]domain.Date, 0, len(values))
	for _, value := range values {
		result = append(result, domain.Date(value))
	}
	return result
}

func recurrenceRuleToDomain(value *appointments.RecurrenceRule) domain.RecurrenceRule {
	return domain.RecurrenceRule{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		AppointmentID: value.AppointmentID, Frequency: value.Frequency, IntervalCount: value.IntervalCount,
		Weekdays: value.Weekdays, MonthDays: value.MonthDays, EndsOn: publicDateToDomain(value.EndsOn),
		OccurrenceCount: value.OccurrenceCount,
	}
}

func recurrenceRuleToPublic(value domain.RecurrenceRule) *appointments.RecurrenceRule {
	return &appointments.RecurrenceRule{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		AppointmentID: value.AppointmentID, Frequency: value.Frequency, IntervalCount: value.IntervalCount,
		Weekdays: value.Weekdays, MonthDays: value.MonthDays, EndsOn: domainDateToPublic(value.EndsOn),
		OccurrenceCount: value.OccurrenceCount,
	}
}

func recurrenceRulesToPublic(values []domain.RecurrenceRule) []*appointments.RecurrenceRule {
	result := make([]*appointments.RecurrenceRule, 0, len(values))
	for _, value := range values {
		result = append(result, recurrenceRuleToPublic(value))
	}
	return result
}

func occurrenceOverrideToDomain(value *appointments.AppointmentOccurrenceOverride) domain.AppointmentOccurrenceOverride {
	return domain.AppointmentOccurrenceOverride{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		AppointmentID: value.AppointmentID, OccurrenceDate: domain.Date(value.OccurrenceDate), Cancelled: value.Cancelled,
		Title: value.Title, Description: value.Description, Location: value.Location,
		StartDate: publicDateToDomain(value.StartDate), EndDate: publicDateToDomain(value.EndDate),
		StartTime: value.StartTime, EndTime: value.EndTime, AllDay: value.AllDay,
	}
}

func occurrenceOverrideToPublic(value domain.AppointmentOccurrenceOverride) *appointments.AppointmentOccurrenceOverride {
	return &appointments.AppointmentOccurrenceOverride{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		AppointmentID: value.AppointmentID, OccurrenceDate: appointments.Date(value.OccurrenceDate), Cancelled: value.Cancelled,
		Title: value.Title, Description: value.Description, Location: value.Location,
		StartDate: domainDateToPublic(value.StartDate), EndDate: domainDateToPublic(value.EndDate),
		StartTime: value.StartTime, EndTime: value.EndTime, AllDay: value.AllDay,
	}
}

func occurrenceOverridesToPublic(values []domain.AppointmentOccurrenceOverride) []*appointments.AppointmentOccurrenceOverride {
	result := make([]*appointments.AppointmentOccurrenceOverride, 0, len(values))
	for _, value := range values {
		result = append(result, occurrenceOverrideToPublic(value))
	}
	return result
}

func appointmentRecipientToPublic(value domain.AppointmentRecipient) *appointments.AppointmentRecipient {
	return &appointments.AppointmentRecipient{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		AppointmentID: value.AppointmentID, RecipientType: value.RecipientType,
		StaffID: value.StaffID, GuardianProfileID: value.GuardianProfileID,
		Status: value.Status, RespondedAt: value.RespondedAt,
	}
}

func appointmentRecipientsToPublic(values []domain.AppointmentRecipient) []*appointments.AppointmentRecipient {
	result := make([]*appointments.AppointmentRecipient, 0, len(values))
	for _, value := range values {
		result = append(result, appointmentRecipientToPublic(value))
	}
	return result
}

func appointmentRecipientStudentToPublic(value domain.AppointmentRecipientStudent) *appointments.AppointmentRecipientStudent {
	return &appointments.AppointmentRecipientStudent{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		RecipientID: value.RecipientID, StudentID: value.StudentID,
	}
}

func appointmentRecipientStudentsToPublic(values []domain.AppointmentRecipientStudent) []*appointments.AppointmentRecipientStudent {
	result := make([]*appointments.AppointmentRecipientStudent, 0, len(values))
	for _, value := range values {
		result = append(result, appointmentRecipientStudentToPublic(value))
	}
	return result
}

func publicDateToDomain(value *appointments.Date) *domain.Date {
	if value == nil {
		return nil
	}
	converted := domain.Date(*value)
	return &converted
}

func domainDateToPublic(value *domain.Date) *appointments.Date {
	if value == nil {
		return nil
	}
	converted := appointments.Date(*value)
	return &converted
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
	case errors.Is(err, domain.ErrAppointmentRecipientNotFound):
		return fmt.Errorf("%w: %w", appointments.ErrAppointmentRecipientNotFound, err)
	case errors.Is(err, domain.ErrAppointmentLifecycleConflict):
		return fmt.Errorf("%w: %w", appointments.ErrAppointmentLifecycleConflict, err)
	default:
		return err
	}
}
