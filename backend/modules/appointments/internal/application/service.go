package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/ports"
)

// Service is the Appointments owner. It validates every input at the owner
// edge before the operation is observed, observes each store operation and
// runs a standalone operation in its own tenant unit of work. The embedded
// Retention deletes the ended appointments (#4045).
type Service struct {
	*Retention
	store   ports.Store
	observe ports.Observer
	RunInTx func(context.Context, func(context.Context) error) error
}

var _ appointments.Owner = (*Service)(nil)

func New(store ports.Store, retention *Retention, observe ports.Observer, runInTx func(context.Context, func(context.Context) error) error) *Service {
	if store == nil || retention == nil || observe == nil || runInTx == nil {
		panic("appointments application: store, retention and observer are required")
	}
	return &Service{Retention: retention, store: store, observe: observe, RunInTx: runInTx}
}

func (s *Service) FindAppointment(ctx context.Context, id int64) (*appointments.Appointment, error) {
	return s.findAppointment(ctx, id, false)
}

func (s *Service) FindAppointmentForUpdate(ctx context.Context, id int64) (*appointments.Appointment, error) {
	return s.findAppointment(ctx, id, true)
}

func (s *Service) findAppointment(ctx context.Context, id int64, lock bool) (result *appointments.Appointment, err error) {
	if id <= 0 {
		return nil, invalid("appointment ID is required")
	}
	operation, run := "find_appointment", s.runUnit
	if lock {
		operation, run = "lock_appointment", s.run
	}
	err = run(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindAppointment(ctx, id, lock)
		stats.Add(queryStats)
		if err == nil && result == nil {
			return appointments.ErrAppointmentNotFound
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) FindReminderCandidateForUpdate(ctx context.Context, id int64) (result *appointments.Appointment, err error) {
	if id <= 0 {
		return nil, invalid("appointment ID is required")
	}
	err = s.run(ctx, "lock_reminder_candidate", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindReminderCandidateForUpdate(ctx, id)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindReminderCandidatesForUpdate(ctx context.Context, ids []int64) (result []*appointments.Appointment, err error) {
	ids = positiveIDs(ids)
	if len(ids) == 0 {
		return []*appointments.Appointment{}, nil
	}
	err = s.run(ctx, "lock_reminder_candidates", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindReminderCandidatesForUpdate(ctx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListAppointmentsVisibleToStaff(ctx context.Context, staffID int64, from, to appointments.Date) (result []*appointments.Appointment, err error) {
	if staffID <= 0 || from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, invalid("staff ID and a valid appointment window are required")
	}
	err = s.runUnit(ctx, "list_staff_appointments", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListAppointmentsVisibleToStaff(ctx, staffID, from, to)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListStaffCancellationTombstones(ctx context.Context, staffID int64, since time.Time) (result []*appointments.Appointment, err error) {
	if staffID <= 0 || since.IsZero() {
		return nil, invalid("staff ID and tombstone cutoff are required")
	}
	err = s.runUnit(ctx, "list_staff_cancellation_tombstones", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffCancellationTombstones(ctx, staffID, since)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListAppointmentsVisibleToGuardians(ctx context.Context, guardianIDs, studentIDs []int64, from, to appointments.Date) (result []*appointments.Appointment, err error) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, invalid("a valid appointment window is required")
	}
	guardianIDs, studentIDs = positiveIDs(guardianIDs), positiveIDs(studentIDs)
	err = s.runUnit(ctx, "list_guardian_appointments", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListAppointmentsVisibleToGuardians(ctx, guardianIDs, studentIDs, from, to)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListGuardianCancellationTombstones(ctx context.Context, guardianIDs, studentIDs []int64, since time.Time) (result []*appointments.Appointment, err error) {
	if since.IsZero() {
		return nil, invalid("tombstone cutoff is required")
	}
	guardianIDs, studentIDs = positiveIDs(guardianIDs), positiveIDs(studentIDs)
	err = s.runUnit(ctx, "list_guardian_cancellation_tombstones", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListGuardianCancellationTombstones(ctx, guardianIDs, studentIDs, since)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListGuardianReminderCandidates(ctx context.Context, from, to appointments.Date) (result []*appointments.Appointment, err error) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, invalid("a valid reminder window is required")
	}
	err = s.runUnit(ctx, "list_guardian_reminder_candidates", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListGuardianReminderCandidates(ctx, from, to)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindAppointmentTargets(ctx context.Context, appointmentID int64) (result []*appointments.AppointmentTarget, err error) {
	if appointmentID <= 0 {
		return nil, invalid("appointment ID is required")
	}
	err = s.runUnit(ctx, "find_appointment_targets", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindAppointmentTargets(ctx, appointmentID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindRecurrenceRule(ctx context.Context, appointmentID int64) (result *appointments.RecurrenceRule, err error) {
	if appointmentID <= 0 {
		return nil, invalid("appointment ID is required")
	}
	err = s.runUnit(ctx, "find_recurrence_rule", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindRecurrenceRule(ctx, appointmentID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindRecurrenceRules(ctx context.Context, appointmentIDs []int64) (result []*appointments.RecurrenceRule, err error) {
	appointmentIDs = positiveIDs(appointmentIDs)
	if len(appointmentIDs) == 0 {
		return []*appointments.RecurrenceRule{}, nil
	}
	err = s.runUnit(ctx, "find_recurrence_rules", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindRecurrenceRules(ctx, appointmentIDs)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindOccurrenceOverrides(ctx context.Context, appointmentIDs []int64, dates []appointments.Date) (result []*appointments.AppointmentOccurrenceOverride, err error) {
	appointmentIDs = positiveIDs(appointmentIDs)
	err = s.runUnit(ctx, "find_occurrence_overrides", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindOccurrenceOverrides(ctx, appointmentIDs, dates)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindOccurrenceOverridesByStartDates(ctx context.Context, appointmentIDs []int64, dates []appointments.Date) (result []*appointments.AppointmentOccurrenceOverride, err error) {
	appointmentIDs = positiveIDs(appointmentIDs)
	if len(appointmentIDs) == 0 || len(dates) == 0 {
		return []*appointments.AppointmentOccurrenceOverride{}, nil
	}
	err = s.runUnit(ctx, "find_occurrence_overrides_by_start_date", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindOccurrenceOverridesByStartDates(ctx, appointmentIDs, dates)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindCancelledOccurrenceOverrides(ctx context.Context, appointmentIDs []int64) (result []*appointments.AppointmentOccurrenceOverride, err error) {
	appointmentIDs = positiveIDs(appointmentIDs)
	err = s.runUnit(ctx, "find_cancelled_occurrence_overrides", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindCancelledOccurrenceOverrides(ctx, appointmentIDs)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindAppointmentRecipient(ctx context.Context, recipientID int64) (result *appointments.AppointmentRecipient, err error) {
	if recipientID <= 0 {
		return nil, invalid("appointment recipient ID is required")
	}
	err = s.runUnit(ctx, "find_appointment_recipient", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindAppointmentRecipient(ctx, recipientID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindAppointmentRecipients(ctx context.Context, appointmentID int64) ([]*appointments.AppointmentRecipient, error) {
	if appointmentID <= 0 {
		return nil, invalid("appointment ID is required")
	}
	return s.findAppointmentRecipients(ctx, []int64{appointmentID})
}

func (s *Service) FindAppointmentRecipientsByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*appointments.AppointmentRecipient, error) {
	appointmentIDs = positiveIDs(appointmentIDs)
	if len(appointmentIDs) == 0 {
		return []*appointments.AppointmentRecipient{}, nil
	}
	return s.findAppointmentRecipients(ctx, appointmentIDs)
}

func (s *Service) findAppointmentRecipients(ctx context.Context, appointmentIDs []int64) (result []*appointments.AppointmentRecipient, err error) {
	err = s.runUnit(ctx, "find_appointment_recipients", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindAppointmentRecipients(ctx, appointmentIDs)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindAppointmentRecipientStudents(ctx context.Context, recipientIDs []int64) (result []*appointments.AppointmentRecipientStudent, err error) {
	recipientIDs = positiveIDs(recipientIDs)
	if len(recipientIDs) == 0 {
		return []*appointments.AppointmentRecipientStudent{}, nil
	}
	err = s.runUnit(ctx, "find_appointment_recipient_students", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindAppointmentRecipientStudents(ctx, recipientIDs)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CountAppointmentRecipientStudents(ctx context.Context, studentID int64) (result int, err error) {
	if studentID <= 0 {
		return 0, invalid("student ID is required")
	}
	err = s.runUnit(ctx, "count_appointment_recipient_students", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountAppointmentRecipientStudents(ctx, studentID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateAppointment(ctx context.Context, input appointments.CreateAppointment) (result *appointments.Appointment, targetRows []*appointments.AppointmentTarget, err error) {
	fields, err := appointments.ValidateAppointmentFields(input.AppointmentFields)
	if err != nil {
		return nil, nil, err
	}
	targetRows = []*appointments.AppointmentTarget{}
	err = s.runUnit(ctx, "create_appointment", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CreateAppointment(ctx, fields)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if len(input.Targets) == 0 {
			return nil
		}
		targetRows, queryStats, err = s.store.InsertAppointmentTargets(ctx, result.ID, input.Targets)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return result, targetRows, nil
}

func (s *Service) UpdateAppointment(ctx context.Context, input appointments.UpdateAppointment) (result *appointments.Appointment, err error) {
	if input.ID <= 0 {
		return nil, invalid("appointment ID is required")
	}
	fields, err := appointments.ValidateAppointmentFields(input.AppointmentFields)
	if err != nil {
		return nil, err
	}
	err = s.runUnit(ctx, "update_appointment", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.UpdateAppointment(ctx, input.ID, fields)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) DeleteAppointment(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("appointment ID is required")
	}
	return s.runUnit(ctx, "delete_appointment", func(ctx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteAppointment(ctx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) CancelAppointment(ctx context.Context, id int64) (transitioned bool, err error) {
	if id <= 0 {
		return false, invalid("appointment ID is required")
	}
	err = s.runUnit(ctx, "cancel_appointment", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		transitioned, queryStats, err = s.store.CancelAppointment(ctx, id)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return false, err
	}
	return transitioned, nil
}

func (s *Service) SoftDeleteAppointment(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("appointment ID is required")
	}
	return s.runUnit(ctx, "soft_delete_appointment", func(ctx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.SoftDeleteAppointment(ctx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteFeedTombstonesBefore(ctx context.Context, before time.Time) (rows int, err error) {
	if before.IsZero() {
		return 0, invalid("tombstone cutoff is required")
	}
	err = s.runUnit(ctx, "delete_appointment_tombstones", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		rows, queryStats, err = s.store.DeleteFeedTombstonesBefore(ctx, before)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (s *Service) ReplaceAppointmentTargets(ctx context.Context, appointmentID int64, targets []appointments.AppointmentTargetFields) (result []*appointments.AppointmentTarget, err error) {
	if appointmentID <= 0 {
		return nil, invalid("appointment ID is required")
	}
	result = []*appointments.AppointmentTarget{}
	err = s.runUnit(ctx, "replace_appointment_targets", func(ctx context.Context, stats *domain.OperationStats) error {
		deleteStats, deleteErr := s.store.DeleteAppointmentTargets(ctx, appointmentID)
		stats.Add(deleteStats)
		if deleteErr != nil {
			return deleteErr
		}
		if len(targets) == 0 {
			return nil
		}
		var insertStats domain.OperationStats
		result, insertStats, err = s.store.InsertAppointmentTargets(ctx, appointmentID, targets)
		stats.Add(insertStats)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) CreateRecurrenceRule(ctx context.Context, rule *appointments.RecurrenceRule) error {
	if err := rule.Validate(); err != nil {
		return err
	}
	var created *appointments.RecurrenceRule
	err := s.runUnit(ctx, "create_recurrence_rule", func(ctx context.Context, stats *domain.OperationStats) (err error) {
		var queryStats domain.OperationStats
		created, queryStats, err = s.store.CreateRecurrenceRule(ctx, *rule)
		stats.Add(queryStats)
		return err
	})
	if err == nil {
		*rule = *created
	}
	return err
}

func (s *Service) DeleteRecurrenceRule(ctx context.Context, appointmentID int64) error {
	if appointmentID <= 0 {
		return invalid("appointment ID is required")
	}
	return s.runUnit(ctx, "delete_recurrence_rule", func(ctx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteRecurrenceRule(ctx, appointmentID)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) CreateOccurrenceOverride(ctx context.Context, override *appointments.AppointmentOccurrenceOverride) error {
	if override == nil || override.AppointmentID <= 0 || override.OccurrenceDate.IsZero() {
		return invalid("appointment ID and occurrence date are required")
	}
	var created *appointments.AppointmentOccurrenceOverride
	err := s.runUnit(ctx, "create_occurrence_override", func(ctx context.Context, stats *domain.OperationStats) (err error) {
		var queryStats domain.OperationStats
		created, queryStats, err = s.store.CreateOccurrenceOverride(ctx, *override)
		stats.Add(queryStats)
		return err
	})
	if err == nil {
		*override = *created
	}
	return err
}

func (s *Service) DeleteOccurrenceOverrides(ctx context.Context, appointmentID int64) error {
	if appointmentID <= 0 {
		return invalid("appointment ID is required")
	}
	return s.runUnit(ctx, "delete_occurrence_overrides", func(ctx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteOccurrenceOverrides(ctx, appointmentID)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) CancelAppointmentOccurrence(ctx context.Context, appointmentID int64, occurrenceDate appointments.Date) (transitioned bool, err error) {
	if appointmentID <= 0 || occurrenceDate.IsZero() {
		return false, invalid("appointment ID and occurrence date are required")
	}
	err = s.runUnit(ctx, "cancel_appointment_occurrence", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		transitioned, queryStats, err = s.store.CancelOccurrence(ctx, appointmentID, occurrenceDate)
		stats.Add(queryStats)
		if err != nil || !transitioned {
			return err
		}
		queryStats, err = s.store.BumpAppointmentRevision(ctx, appointmentID)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return false, err
	}
	return transitioned, nil
}

func (s *Service) CreateAppointmentRecipients(ctx context.Context, appointmentID int64, fields []appointments.AppointmentRecipientFields) (result []*appointments.AppointmentRecipient, links []*appointments.AppointmentRecipientStudent, err error) {
	if appointmentID <= 0 {
		return nil, nil, invalid("appointment ID is required")
	}
	normalized := make([]appointments.AppointmentRecipientFields, 0, len(fields))
	for _, value := range fields {
		for _, studentID := range value.StudentIDs {
			if studentID <= 0 {
				return nil, nil, invalid("student IDs must be positive")
			}
		}
		value.StudentIDs = positiveIDs(value.StudentIDs)
		if err := appointments.ValidateRecipientFields(value); err != nil {
			return nil, nil, err
		}
		normalized = append(normalized, value)
	}
	links = []*appointments.AppointmentRecipientStudent{}
	err = s.runUnit(ctx, "create_appointment_recipients", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.InsertAppointmentRecipients(ctx, appointmentID, normalized)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		pending, linkErr := recipientStudentLinks(normalized, result)
		if linkErr != nil || len(pending) == 0 {
			return linkErr
		}
		links, queryStats, err = s.store.InsertAppointmentRecipientStudents(ctx, pending)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return result, links, nil
}

func (s *Service) UpdateAppointmentRecipientResponse(ctx context.Context, recipientID int64, status string) error {
	if recipientID <= 0 {
		return invalid("appointment recipient ID is required")
	}
	if !appointments.ValidResponseStatus(status) {
		return invalid("invalid recipient status")
	}
	return s.runUnit(ctx, "update_appointment_recipient_response", func(ctx context.Context, stats *domain.OperationStats) error {
		updated, queryStats, err := s.store.UpdateAppointmentRecipientResponse(ctx, recipientID, status)
		stats.Add(queryStats)
		if err == nil && !updated {
			return appointments.ErrAppointmentRecipientNotFound
		}
		return err
	})
}

func (s *Service) ClaimReminderPushDelivery(ctx context.Context, appointmentID int64, revision int, occurrenceDate appointments.Date, guardianProfileID int64) (claimed bool, err error) {
	if appointmentID <= 0 || revision < 0 || occurrenceDate.IsZero() || guardianProfileID <= 0 {
		return false, invalid("appointment ID, revision, occurrence date, and guardian profile ID are required")
	}
	err = s.runUnit(ctx, "claim_reminder_push_delivery", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		claimed, queryStats, err = s.store.ClaimReminderPushDelivery(ctx, appointmentID, revision, occurrenceDate, guardianProfileID)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return false, err
	}
	return claimed, nil
}

func (s *Service) ReleaseReminderPushDelivery(ctx context.Context, appointmentID int64, revision int, occurrenceDate appointments.Date, guardianProfileID int64) error {
	if appointmentID <= 0 || revision < 0 || occurrenceDate.IsZero() || guardianProfileID <= 0 {
		return invalid("appointment ID, revision, occurrence date, and guardian profile ID are required")
	}
	return s.runUnit(ctx, "release_reminder_push_delivery", func(ctx context.Context, stats *domain.OperationStats) error {
		_, queryStats, err := s.store.ReleaseReminderPushDelivery(ctx, appointmentID, revision, occurrenceDate, guardianProfileID)
		stats.Add(queryStats)
		return err
	})
}

type recipientSubject struct {
	recipientType string
	subjectID     int64
}

func recipientStudentLinks(fields []appointments.AppointmentRecipientFields, recipients []*appointments.AppointmentRecipient) ([]appointments.AppointmentRecipientStudent, error) {
	recipientBySubject := make(map[recipientSubject]int64, len(recipients))
	for _, recipient := range recipients {
		recipientBySubject[subjectForRecipient(recipient.RecipientType, recipient.StaffID, recipient.GuardianProfileID)] = recipient.ID
	}
	links := make([]appointments.AppointmentRecipientStudent, 0)
	for _, field := range fields {
		if len(field.StudentIDs) == 0 {
			continue
		}
		recipientID, ok := recipientBySubject[subjectForRecipient(field.RecipientType, field.StaffID, field.GuardianProfileID)]
		if !ok {
			return nil, fmt.Errorf("appointments application: inserted recipient is missing")
		}
		for _, studentID := range field.StudentIDs {
			links = append(links, appointments.AppointmentRecipientStudent{RecipientID: recipientID, StudentID: studentID})
		}
	}
	return links, nil
}

func subjectForRecipient(recipientType string, staffID, guardianProfileID *int64) recipientSubject {
	result := recipientSubject{recipientType: recipientType}
	if staffID != nil {
		result.subjectID = *staffID
	}
	if guardianProfileID != nil {
		result.subjectID = *guardianProfileID
	}
	return result
}

func invalid(reason string) error { return &appointments.InvalidError{Reason: reason} }

func positiveIDs(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (s *Service) run(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = fn(ctx, &stats)
	return err
}

// runUnit owns the standalone operation's unit; locking reads use run instead.
func (s *Service) runUnit(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) error {
	return s.run(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
		return s.RunInTx(ctx, func(txCtx context.Context) error { return fn(txCtx, stats) })
	})
}
