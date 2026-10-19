package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/domain"
)

func NewPublicModule(service *Service) *appointments.Module {
	return appointments.NewModule(engine{service: service})
}

type engine struct{ service *Service }

func (e engine) FindAppointment(ctx context.Context, id int64) (*appointments.Appointment, error) {
	value, err := e.service.FindAppointment(ctx, id, false)
	return appointmentToPublic(value), MapPublicError(err)
}

func (e engine) FindAppointmentForUpdate(ctx context.Context, id int64) (*appointments.Appointment, error) {
	value, err := e.service.FindAppointment(ctx, id, true)
	return appointmentToPublic(value), MapPublicError(err)
}

func (e engine) FindReminderCandidateForUpdate(ctx context.Context, id int64) (*appointments.Appointment, error) {
	value, found, err := e.service.FindReminderCandidateForUpdate(ctx, id)
	if err != nil || !found {
		return nil, MapPublicError(err)
	}
	return appointmentToPublic(value), nil
}

func (e engine) FindReminderCandidatesForUpdate(ctx context.Context, ids []int64) ([]*appointments.Appointment, error) {
	values, err := e.service.FindReminderCandidatesForUpdate(ctx, ids)
	return appointmentsToPublic(values), MapPublicError(err)
}

func (e engine) ListAppointmentsVisibleToStaff(ctx context.Context, staffID int64, from, to appointments.Date) ([]*appointments.Appointment, error) {
	values, err := e.service.ListAppointmentsVisibleToStaff(ctx, staffID, domain.Date(from), domain.Date(to))
	return appointmentsToPublic(values), MapPublicError(err)
}

func (e engine) ListStaffCancellationTombstones(ctx context.Context, staffID int64, since time.Time) ([]*appointments.Appointment, error) {
	values, err := e.service.ListStaffCancellationTombstones(ctx, staffID, since)
	return appointmentsToPublic(values), MapPublicError(err)
}

func (e engine) ListAppointmentsVisibleToGuardians(ctx context.Context, guardianIDs, studentIDs []int64, from, to appointments.Date) ([]*appointments.Appointment, error) {
	values, err := e.service.ListAppointmentsVisibleToGuardians(ctx, guardianIDs, studentIDs, domain.Date(from), domain.Date(to))
	return appointmentsToPublic(values), MapPublicError(err)
}

func (e engine) ListGuardianCancellationTombstones(ctx context.Context, guardianIDs, studentIDs []int64, since time.Time) ([]*appointments.Appointment, error) {
	values, err := e.service.ListGuardianCancellationTombstones(ctx, guardianIDs, studentIDs, since)
	return appointmentsToPublic(values), MapPublicError(err)
}

func (e engine) ListGuardianReminderCandidates(ctx context.Context, from, to appointments.Date) ([]*appointments.Appointment, error) {
	values, err := e.service.ListGuardianReminderCandidates(ctx, domain.Date(from), domain.Date(to))
	return appointmentsToPublic(values), MapPublicError(err)
}

func (e engine) FindAppointmentTargets(ctx context.Context, appointmentID int64) ([]*appointments.AppointmentTarget, error) {
	values, err := e.service.FindAppointmentTargets(ctx, appointmentID)
	return targetsToPublic(values), MapPublicError(err)
}

func (e engine) FindRecurrenceRule(ctx context.Context, appointmentID int64) (*appointments.RecurrenceRule, error) {
	value, found, err := e.service.FindRecurrenceRule(ctx, appointmentID)
	if err != nil || !found {
		return nil, MapPublicError(err)
	}
	return recurrenceRuleToPublic(value), nil
}

func (e engine) FindRecurrenceRules(ctx context.Context, appointmentIDs []int64) ([]*appointments.RecurrenceRule, error) {
	values, err := e.service.FindRecurrenceRules(ctx, appointmentIDs)
	return recurrenceRulesToPublic(values), MapPublicError(err)
}

func (e engine) FindOccurrenceOverrides(ctx context.Context, appointmentIDs []int64, dates []appointments.Date) ([]*appointments.AppointmentOccurrenceOverride, error) {
	values, err := e.service.FindOccurrenceOverrides(ctx, appointmentIDs, datesToDomain(dates))
	return occurrenceOverridesToPublic(values), MapPublicError(err)
}

func (e engine) FindOccurrenceOverridesByStartDates(ctx context.Context, appointmentIDs []int64, dates []appointments.Date) ([]*appointments.AppointmentOccurrenceOverride, error) {
	values, err := e.service.FindOccurrenceOverridesByStartDates(ctx, appointmentIDs, datesToDomain(dates))
	return occurrenceOverridesToPublic(values), MapPublicError(err)
}

func (e engine) FindCancelledOccurrenceOverrides(ctx context.Context, appointmentIDs []int64) ([]*appointments.AppointmentOccurrenceOverride, error) {
	values, err := e.service.FindCancelledOccurrenceOverrides(ctx, appointmentIDs)
	return occurrenceOverridesToPublic(values), MapPublicError(err)
}

func (e engine) FindAppointmentRecipient(ctx context.Context, recipientID int64) (*appointments.AppointmentRecipient, error) {
	value, found, err := e.service.FindAppointmentRecipient(ctx, recipientID)
	if err != nil || !found {
		return nil, MapPublicError(err)
	}
	return appointmentRecipientToPublic(value), nil
}

func (e engine) FindAppointmentRecipients(ctx context.Context, appointmentID int64) ([]*appointments.AppointmentRecipient, error) {
	values, err := e.service.FindAppointmentRecipients(ctx, []int64{appointmentID})
	return appointmentRecipientsToPublic(values), MapPublicError(err)
}

func (e engine) FindAppointmentRecipientsByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*appointments.AppointmentRecipient, error) {
	values, err := e.service.FindAppointmentRecipients(ctx, appointmentIDs)
	return appointmentRecipientsToPublic(values), MapPublicError(err)
}

func (e engine) FindAppointmentRecipientStudents(ctx context.Context, recipientIDs []int64) ([]*appointments.AppointmentRecipientStudent, error) {
	values, err := e.service.FindAppointmentRecipientStudents(ctx, recipientIDs)
	return appointmentRecipientStudentsToPublic(values), MapPublicError(err)
}

func (e engine) CountAppointmentRecipientStudents(ctx context.Context, studentID int64) (int, error) {
	value, err := e.service.CountAppointmentRecipientStudents(ctx, studentID)
	return value, MapPublicError(err)
}

func (e engine) CreateAppointment(ctx context.Context, input appointments.CreateAppointment) (*appointments.Appointment, []*appointments.AppointmentTarget, error) {
	value, targets, err := e.service.CreateAppointment(ctx, fieldsToDomain(input.AppointmentFields), targetFieldsToDomain(input.Targets))
	if err != nil {
		return nil, nil, MapPublicError(err)
	}
	return appointmentToPublic(value), targetsToPublic(targets), nil
}
func (e engine) UpdateAppointment(ctx context.Context, input appointments.UpdateAppointment) (*appointments.Appointment, error) {
	value, err := e.service.UpdateAppointment(ctx, input.ID, fieldsToDomain(input.AppointmentFields))
	if err != nil {
		return nil, MapPublicError(err)
	}
	return appointmentToPublic(value), nil
}
func (e engine) DeleteAppointment(ctx context.Context, id int64) error {
	return MapPublicError(e.service.DeleteAppointment(ctx, id))
}
func (e engine) CancelAppointment(ctx context.Context, id int64) (bool, error) {
	changed, err := e.service.CancelAppointment(ctx, id)
	if err != nil {
		return false, MapPublicError(err)
	}
	return changed, nil
}
func (e engine) SoftDeleteAppointment(ctx context.Context, id int64) error {
	return MapPublicError(e.service.SoftDeleteAppointment(ctx, id))
}
func (e engine) DeleteFeedTombstonesBefore(ctx context.Context, before time.Time) (int, error) {
	count, err := e.service.DeleteFeedTombstonesBefore(ctx, before)
	if err != nil {
		return 0, MapPublicError(err)
	}
	return count, nil
}
func (e engine) ReplaceAppointmentTargets(ctx context.Context, appointmentID int64, fields []appointments.AppointmentTargetFields) ([]*appointments.AppointmentTarget, error) {
	values, err := e.service.ReplaceAppointmentTargets(ctx, appointmentID, targetFieldsToDomain(fields))
	if err != nil {
		return nil, MapPublicError(err)
	}
	return targetsToPublic(values), nil
}
func (e engine) CreateRecurrenceRule(ctx context.Context, rule *appointments.RecurrenceRule) error {
	value, err := e.service.CreateRecurrenceRule(ctx, recurrenceRuleToDomain(rule))
	if err == nil {
		*rule = *recurrenceRuleToPublic(value)
	}
	return MapPublicError(err)
}
func (e engine) DeleteRecurrenceRule(ctx context.Context, appointmentID int64) error {
	return MapPublicError(e.service.DeleteRecurrenceRule(ctx, appointmentID))
}
func (e engine) CreateOccurrenceOverride(ctx context.Context, override *appointments.AppointmentOccurrenceOverride) error {
	value, err := e.service.CreateOccurrenceOverride(ctx, occurrenceOverrideToDomain(override))
	if err == nil {
		*override = *occurrenceOverrideToPublic(value)
	}
	return MapPublicError(err)
}
func (e engine) DeleteOccurrenceOverrides(ctx context.Context, appointmentID int64) error {
	return MapPublicError(e.service.DeleteOccurrenceOverrides(ctx, appointmentID))
}
func (e engine) CancelAppointmentOccurrence(ctx context.Context, appointmentID int64, date appointments.Date) (bool, error) {
	changed, err := e.service.CancelAppointmentOccurrence(ctx, appointmentID, domain.Date(date))
	if err != nil {
		return false, MapPublicError(err)
	}
	return changed, nil
}
func (e engine) CreateAppointmentRecipients(ctx context.Context, appointmentID int64, fields []appointments.AppointmentRecipientFields) ([]*appointments.AppointmentRecipient, []*appointments.AppointmentRecipientStudent, error) {
	values, links, err := e.service.CreateAppointmentRecipients(ctx, appointmentID, recipientFieldsToDomain(fields))
	if err != nil {
		return nil, nil, MapPublicError(err)
	}
	return appointmentRecipientsToPublic(values), appointmentRecipientStudentsToPublic(links), nil
}
func (e engine) UpdateAppointmentRecipientResponse(ctx context.Context, recipientID int64, status string) error {
	return MapPublicError(e.service.UpdateAppointmentRecipientResponse(ctx, recipientID, status))
}
func (e engine) ClaimReminderPushDelivery(ctx context.Context, appointmentID int64, revision int, date appointments.Date, guardianID int64) (bool, error) {
	claimed, err := e.service.ClaimReminderPushDelivery(ctx, appointmentID, revision, domain.Date(date), guardianID)
	if err != nil {
		return false, MapPublicError(err)
	}
	return claimed, nil
}
func (e engine) ReleaseReminderPushDelivery(ctx context.Context, appointmentID int64, revision int, date appointments.Date, guardianID int64) error {
	return MapPublicError(e.service.ReleaseReminderPushDelivery(ctx, appointmentID, revision, domain.Date(date), guardianID))
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

func MapPublicError(err error) error {
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
