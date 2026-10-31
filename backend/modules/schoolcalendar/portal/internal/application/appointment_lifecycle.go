package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
)

func (s *service) CreateStaffAppointment(ctx context.Context, req CreateAppointmentRequest) (*AppointmentDetail, error) {
	var result *AppointmentDetail
	err := s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		var commandErr error
		result, commandErr = s.createAppointment(txCtx, req, false)
		return commandErr
	})
	return result, err
}

// createAppointment writes the appointment the current staff member
// organizes. allowEmptyParents keeps a bulk parent target without reachable
// guardians from failing the write; Schultermine use it (#4798).
func (s *service) createAppointment(ctx context.Context, req CreateAppointmentRequest, allowEmptyParents bool) (*AppointmentDetail, error) {
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	return s.createAppointmentFor(ctx, staff.ID, req, allowEmptyParents)
}

// createAppointmentFor writes an appointment organized by organizerStaffID.
// The Elternsprechtag uses it for a family's booking, which the Lehrkraft
// organizes (#4801).
func (s *service) createAppointmentFor(ctx context.Context, organizerStaffID int64, req CreateAppointmentRequest, allowEmptyParents bool) (*AppointmentDetail, error) {
	appointment, recurrence, err := newStaffAppointment(organizerStaffID, req)
	if err != nil {
		return nil, err
	}

	recipientFields, targets, err := s.resolveTargets(ctx, appointment.DeliveryMode, req.Targets, allowEmptyParents)
	if err != nil {
		return nil, err
	}

	created, persistedTargets, err := s.cfg.Appointments.CreateAppointment(ctx, appointmentcap.CreateAppointment{
		AppointmentFields: appointmentCapabilityFields(appointment),
		Targets:           appointmentTargetFields(targets),
	})
	if err != nil {
		return nil, err
	}
	appointment = created
	targets = persistedTargets

	if recurrence != nil {
		recurrence.AppointmentID = appointment.ID
		if err := s.cfg.Appointments.CreateRecurrenceRule(ctx, recurrence); err != nil {
			return nil, err
		}
	}

	recipients, _, err := s.cfg.Appointments.CreateAppointmentRecipients(ctx, appointment.ID, recipientFields)
	if err != nil {
		return nil, err
	}

	if req.SendEmail {
		if err := s.notifyGuardianLifecycle(ctx, appointment, portal.EmailKindAppointmentPublished); err != nil {
			return nil, err
		}
	}

	return &AppointmentDetail{
		Appointment: appointment,
		Recurrence:  recurrence,
		Recipients:  recipients,
		Targets:     targets,
	}, nil
}

// newStaffAppointment builds and validates the appointment (with its optional
// recurrence) a staff member requested, applying the creation defaults. The
// returned recurrence carries no appointment ID yet.
func newStaffAppointment(organizerStaffID int64, req CreateAppointmentRequest) (*appointmentcap.Appointment, *appointmentcap.RecurrenceRule, error) {
	if req.DeliveryMode == "" {
		req.DeliveryMode = appointmentcap.DeliveryModeRSVPRequired
	}
	if req.OverviewVisibility == "" {
		req.OverviewVisibility = appointmentcap.OverviewVisibilityOrganizer
	}
	if req.EndDate.IsZero() {
		req.EndDate = req.StartDate
	}

	appointment := &appointmentcap.Appointment{
		OrganizerStaffID:   organizerStaffID,
		Title:              req.Title,
		Description:        req.Description,
		Location:           req.Location,
		StartDate:          toCalendarDate(req.StartDate),
		EndDate:            toCalendarDate(req.EndDate),
		StartTime:          normalizeWallClock(req.StartTime),
		EndTime:            normalizeWallClock(req.EndTime),
		AllDay:             req.AllDay,
		DeliveryMode:       req.DeliveryMode,
		OverviewVisibility: req.OverviewVisibility,
		// Persist the notification opt-in so a later cancellation honours it: an
		// appointment created without send_email never mails guardians.
		NotifyGuardians: req.SendEmail,
		SchoolEventType: req.SchoolEventType,
	}
	if err := appointment.Validate(); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	recurrence := recurrenceRuleFromRequest(req.Recurrence)
	if recurrence != nil {
		recurrence.AppointmentID = 1
		if err := validateRecurrence(appointment, recurrence); err != nil {
			return nil, nil, err
		}
		recurrence.AppointmentID = 0
	}
	return appointment, recurrence, nil
}

// validateRecurrence checks a requested recurrence rule against the
// appointment it repeats: the rule itself, an end on or after the start, and
// at least one occurrence.
func validateRecurrence(appointment *appointmentcap.Appointment, recurrence *appointmentcap.RecurrenceRule) error {
	if err := recurrence.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if recurrence.EndsOn != nil && recurrence.EndsOn.Before(appointment.StartDate) {
		return fmt.Errorf("%w: recurrence end must be on or after start date", ErrInvalidRequest)
	}
	if _, ok := firstRecurrenceOccurrence(appointment, recurrence); !ok {
		return fmt.Errorf("%w: recurrence produces no occurrences", ErrInvalidRequest)
	}
	return nil
}

// loadOrganizedAppointment fetches an appointment and asserts the current staff
// member is its organizer. Only the organizer may edit, cancel, or delete an
// appointment. Returns ErrNotFound for missing rows and ErrForbidden when the
// caller is not the organizer.
func (s *service) loadOrganizedAppointment(ctx context.Context, appointmentID int64) (*appointmentcap.Appointment, error) {
	if appointmentID <= 0 {
		return nil, fmt.Errorf("%w: appointment id is required", ErrInvalidRequest)
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	appointment, err := s.findAppointment(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	if appointment == nil {
		return nil, ErrNotFound
	}
	// A soft-deleted appointment lives on only as a feed tombstone; it is gone from
	// every interactive surface, so lifecycle operations (edit/cancel/delete/detail)
	// treat it as not found.
	if appointment.DeletedAt != nil {
		return nil, ErrNotFound
	}
	if appointment.OrganizerStaffID != staff.ID {
		return nil, fmt.Errorf("%w: only the organizer may modify this appointment", ErrForbidden)
	}
	// A booked Elternsprechtag slot changes only through its booking (#4801):
	// the family books, moves or cancels it, the Lehrkraft changes the day.
	if isParentConferenceAppointment(appointment) {
		return nil, fmt.Errorf("%w: an Elternsprechtag booking changes through the Sprechtag", ErrForbidden)
	}
	return appointment, nil
}

// loadAppointmentForWrite loads an appointment the current staff member
// organizes and locks it for a lifecycle write. A row that vanished between
// the two reads is not found.
func (s *service) loadAppointmentForWrite(ctx context.Context, appointmentID int64) (*appointmentcap.Appointment, error) {
	appointment, err := s.loadOrganizedAppointment(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	appointment, err = s.findAppointmentForUpdate(ctx, appointment.ID)
	if s.cfg.MissingRecord(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return appointment, nil
}

func (s *service) findAppointment(ctx context.Context, appointmentID int64) (*appointmentcap.Appointment, error) {
	appointment, err := s.cfg.Appointments.FindAppointment(ctx, appointmentID)
	if errors.Is(err, appointmentcap.ErrAppointmentNotFound) {
		return nil, nil
	}
	return appointment, err
}

func (s *service) findAppointmentForUpdate(ctx context.Context, appointmentID int64) (*appointmentcap.Appointment, error) {
	appointment, err := s.cfg.Appointments.FindAppointmentForUpdate(ctx, appointmentID)
	if errors.Is(err, appointmentcap.ErrAppointmentNotFound) {
		return nil, fmt.Errorf("appointment %d not found: %w", appointmentID, s.cfg.MissingRecordCause)
	}
	return appointment, err
}

func appointmentCapabilityFields(value *appointmentcap.Appointment) appointmentcap.AppointmentFields {
	return appointmentcap.AppointmentFields{
		OrganizerStaffID: value.OrganizerStaffID, Title: value.Title, Description: value.Description,
		Location: value.Location, StartDate: value.StartDate, EndDate: value.EndDate,
		StartTime: value.StartTime, EndTime: value.EndTime, AllDay: value.AllDay,
		DeliveryMode: value.DeliveryMode, OverviewVisibility: value.OverviewVisibility,
		NotifyGuardians: value.NotifyGuardians, SchoolEventType: value.SchoolEventType,
	}
}

func appointmentTargetFields(values []*appointmentcap.AppointmentTarget) []appointmentcap.AppointmentTargetFields {
	result := make([]appointmentcap.AppointmentTargetFields, 0, len(values))
	for _, value := range values {
		result = append(result, appointmentcap.AppointmentTargetFields{
			TargetType: value.TargetType, TargetID: value.TargetID, TargetValue: value.TargetValue,
		})
	}
	return result
}

// appointmentDetail reloads the full detail (recurrence, recipients, targets)
// for an appointment. Used by the lifecycle operations so callers (and the
// notification layer in Phase B) get the same shape as CreateStaffAppointment.
func (s *service) appointmentDetail(ctx context.Context, appointment *appointmentcap.Appointment) (*AppointmentDetail, error) {
	recurrence, err := s.cfg.Appointments.FindRecurrenceRule(ctx, appointment.ID)
	if err != nil {
		return nil, err
	}
	recipients, err := s.cfg.Appointments.FindAppointmentRecipients(ctx, appointment.ID)
	if err != nil {
		return nil, err
	}
	targets, err := s.cfg.Appointments.FindAppointmentTargets(ctx, appointment.ID)
	if err != nil {
		return nil, err
	}
	return &AppointmentDetail{
		Appointment: appointment,
		Recurrence:  recurrence,
		Recipients:  recipients,
		Targets:     targets,
	}, nil
}

func (s *service) GetStaffAppointmentDetail(ctx context.Context, appointmentID int64) (*AppointmentDetail, error) {
	appointment, err := s.loadOrganizedAppointment(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	return s.appointmentDetail(ctx, appointment)
}

func (s *service) UpdateStaffAppointment(ctx context.Context, appointmentID int64, req UpdateAppointmentRequest) (*AppointmentDetail, error) {
	var result *AppointmentDetail
	err := s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		var commandErr error
		result, commandErr = s.updateStaffAppointment(txCtx, appointmentID, req)
		return commandErr
	})
	return result, err
}

func (s *service) updateStaffAppointment(ctx context.Context, appointmentID int64, req UpdateAppointmentRequest) (*AppointmentDetail, error) {
	appointment, err := s.loadAppointmentForWrite(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	return s.updateLoadedAppointment(ctx, appointment, req)
}

// updateLoadedAppointment applies req to an appointment locked for write
// and replaces its pending notices.
func (s *service) updateLoadedAppointment(ctx context.Context, appointment *appointmentcap.Appointment, req UpdateAppointmentRequest) (*AppointmentDetail, error) {
	// A cancelled appointment is terminal: there is no reactivation flow, so
	// editing it (which would also fire a "Termin geändert" notice while the
	// appointment stays cancelled) is rejected outright.
	if appointment.CancelledAt != nil {
		return nil, fmt.Errorf("%w: appointment is cancelled", ErrInvalidRequest)
	}
	recurrence, err := applyAppointmentUpdate(appointment, req)
	if err != nil {
		return nil, err
	}
	appointment, err = s.persistAppointmentUpdate(ctx, appointment, recurrence)
	if err != nil {
		return nil, err
	}

	// Kill every not-yet-sent appointment e-mail so the worker cannot deliver a
	// stale title/date/location or a reminder after guardian delivery was turned
	// off. The update notice below replaces the lifecycle communication.
	if err := s.cancelPendingNotifications(ctx, appointment.ID, "appointment updated"); err != nil {
		return nil, err
	}
	if appointment.NotifyGuardians {
		if err := s.notifyGuardianLifecycle(ctx, appointment, portal.EmailKindAppointmentUpdated); err != nil {
			return nil, err
		}
	}

	return s.appointmentDetail(ctx, appointment)
}

// applyAppointmentUpdate writes the editable fields of req onto the loaded
// appointment and validates the result together with the requested
// recurrence, which it returns bound to the appointment.
func applyAppointmentUpdate(appointment *appointmentcap.Appointment, req UpdateAppointmentRequest) (*appointmentcap.RecurrenceRule, error) {
	if req.OverviewVisibility == "" {
		req.OverviewVisibility = appointment.OverviewVisibility
	}
	if req.EndDate.IsZero() {
		req.EndDate = req.StartDate
	}

	appointment.Title = req.Title
	appointment.Description = req.Description
	appointment.Location = req.Location
	appointment.StartDate = toCalendarDate(req.StartDate)
	appointment.EndDate = toCalendarDate(req.EndDate)
	appointment.StartTime = normalizeWallClock(req.StartTime)
	appointment.EndTime = normalizeWallClock(req.EndTime)
	appointment.AllDay = req.AllDay
	appointment.OverviewVisibility = req.OverviewVisibility
	if req.SendEmailSet || req.SendEmail {
		appointment.NotifyGuardians = req.SendEmail
	}
	if err := appointment.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	recurrence := recurrenceRuleFromRequest(req.Recurrence)
	if recurrence != nil {
		recurrence.AppointmentID = appointment.ID
		if err := validateRecurrence(appointment, recurrence); err != nil {
			return nil, err
		}
	}
	return recurrence, nil
}

// persistAppointmentUpdate writes the edited appointment and replaces its
// recurrence rule and occurrence overrides. It returns the stored appointment.
func (s *service) persistAppointmentUpdate(ctx context.Context, appointment *appointmentcap.Appointment, recurrence *appointmentcap.RecurrenceRule) (*appointmentcap.Appointment, error) {
	updated, err := s.cfg.Appointments.UpdateAppointment(ctx, appointmentcap.UpdateAppointment{
		ID: appointment.ID, AppointmentFields: appointmentCapabilityFields(appointment),
	})
	if err != nil {
		// A concurrent cancel/delete transitioned the appointment between load and
		// write, so the conditional update matched nothing. Abort before touching
		// recurrence/overrides or sending an "updated" notice; the tenant tx rolls
		// back. Surface a conflict rather than a bogus success.
		if errors.Is(err, appointmentcap.ErrAppointmentLifecycleConflict) {
			return nil, fmt.Errorf("%w: appointment was cancelled or deleted", ErrConflict)
		}
		return nil, err
	}
	// Replace the recurrence rule wholesale: the DB enforces one rule per
	// appointment, so drop the old row and recreate if the edit still recurs.
	if err := s.cfg.Appointments.DeleteRecurrenceRule(ctx, updated.ID); err != nil {
		return nil, err
	}
	if recurrence != nil {
		if err := s.cfg.Appointments.CreateRecurrenceRule(ctx, recurrence); err != nil {
			return nil, err
		}
	}
	// Editing the series is a whole-series operation, so per-occurrence
	// cancellations ("Nur diesen Termin") from the old cadence no longer apply.
	// Drop them; otherwise a date reused by the new recurrence would be silently
	// suppressed (and stale EXDATEs would leak into the subscription feed/ICS).
	if err := s.cfg.Appointments.DeleteOccurrenceOverrides(ctx, updated.ID); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *service) CancelStaffAppointment(ctx context.Context, appointmentID int64) (*AppointmentDetail, error) {
	var result *AppointmentDetail
	err := s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		var commandErr error
		result, commandErr = s.cancelStaffAppointment(txCtx, appointmentID)
		return commandErr
	})
	return result, err
}

func (s *service) cancelStaffAppointment(ctx context.Context, appointmentID int64) (*AppointmentDetail, error) {
	appointment, err := s.loadAppointmentForWrite(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	if appointment.CancelledAt != nil {
		return s.appointmentDetail(ctx, appointment)
	}
	// Cancel via the dedicated conditional update (not Update): it only writes
	// cancelled_at/revision, so it can't clobber a concurrent edit and — being
	// WHERE cancelled_at IS NULL AND deleted_at IS NULL — matches nothing once a
	// concurrent cancel or delete has won. `transitioned` is true only for the
	// caller that actually flipped the row.
	transitioned, err := s.cfg.Appointments.CancelAppointment(ctx, appointment.ID)
	if err != nil {
		return nil, err
	}
	if !transitioned {
		// A concurrent cancel or delete won between our load and this update, so
		// the row did NOT transition. Do NOT mark the stale in-memory object
		// cancelled (that would return a "cancelled" view of a possibly-deleted
		// appointment). Reload for the true state: loadOrganizedAppointment
		// surfaces a concurrent delete as not-found and returns the (already)
		// cancelled detail for a concurrent cancel — idempotent, no double notice.
		reloaded, err := s.loadOrganizedAppointment(ctx, appointmentID)
		if err != nil {
			return nil, err
		}
		return s.appointmentDetail(ctx, reloaded)
	}
	now := time.Now()
	appointment.CancelledAt = &now
	// Only the caller that performed the transition does the notification work,
	// so two concurrent cancels can't send duplicate guardian e-mails. And even
	// then, honour the persisted opt-in: an appointment created without
	// send_email never mails guardians on cancellation.
	if err := s.cancelPendingNotifications(ctx, appointment.ID, "appointment cancelled"); err != nil {
		return nil, err
	}
	if appointment.NotifyGuardians {
		if err := s.notifyGuardianLifecycle(ctx, appointment, portal.EmailKindAppointmentCancelled); err != nil {
			return nil, err
		}
	}
	return s.appointmentDetail(ctx, appointment)
}

func (s *service) DeleteStaffAppointment(ctx context.Context, appointmentID int64) error {
	return s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		return s.deleteStaffAppointment(txCtx, appointmentID)
	})
}

func (s *service) deleteStaffAppointment(ctx context.Context, appointmentID int64) error {
	appointment, err := s.loadAppointmentForWrite(ctx, appointmentID)
	if err != nil {
		return err
	}
	return s.softDeleteLoadedAppointment(ctx, appointment)
}

// softDeleteLoadedAppointment removes an appointment locked for write.
func (s *service) softDeleteLoadedAppointment(ctx context.Context, appointment *appointmentcap.Appointment) error {
	// Deleting is silent (no notice); kill any queued e-mails first so the worker
	// doesn't send a mail for a row we're about to remove or tombstone.
	if err := s.cancelPendingNotifications(ctx, appointment.ID, "appointment deleted"); err != nil {
		return err
	}
	// A subscribed external calendar only drops an
	// appointment when it re-receives the SAME UID with STATUS:CANCELLED at a
	// higher SEQUENCE. Hard-deleting an appointment makes it vanish
	// from the feed, which many clients keep rather than purge — the stale event
	// then lingers in the subscribed calendar forever. Every appointment can appear
	// in its organizer's staff feed, so it is SOFT-deleted: it disappears from every
	// interactive staff/parent calendar (those queries filter deleted_at IS NULL),
	// yet the subscription feed re-exports it as a durable STATUS:CANCELLED
	// tombstone (retained by deletion time, independent of the date lookback) with
	// a bumped SEQUENCE so even long-offline subscribers eventually purge it.
	return s.cfg.Appointments.SoftDeleteAppointment(ctx, appointment.ID)
}

func (s *service) CancelStaffAppointmentOccurrence(ctx context.Context, appointmentID int64, occurrenceDate appointmentcap.Date) error {
	return s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		return s.cancelStaffAppointmentOccurrence(txCtx, appointmentID, occurrenceDate)
	})
}

func (s *service) cancelStaffAppointmentOccurrence(ctx context.Context, appointmentID int64, occurrenceDate appointmentcap.Date) error {
	appointment, err := s.loadAppointmentForWrite(ctx, appointmentID)
	if err != nil {
		return err
	}
	if occurrenceDate.IsZero() {
		return fmt.Errorf("%w: occurrence date is required", ErrInvalidRequest)
	}
	// A single-occurrence cancellation only makes sense for a date the series
	// actually generates. Without this guard, cancelling a non-recurring
	// appointment (or a date not in the series) would persist a useless override
	// while the appointment stayed fully visible.
	recurrence, err := s.cfg.Appointments.FindRecurrenceRule(ctx, appointment.ID)
	if err != nil {
		return err
	}
	if recurrence == nil {
		return fmt.Errorf("%w: appointment is not recurring", ErrInvalidRequest)
	}
	if !occurrenceExists(appointment, recurrence, occurrenceDate) {
		return fmt.Errorf("%w: occurrence date is not part of the series", ErrInvalidRequest)
	}
	// Reuse an existing override for this date (e.g. from a prior single-occurrence
	// edit) so cancelling stays idempotent and respects the (appointment, date)
	// uniqueness constraint.
	existing, err := s.cfg.Appointments.FindOccurrenceOverrides(ctx, []int64{appointment.ID}, []appointmentcap.Date{toCalendarDate(occurrenceDate)})
	if err != nil {
		return err
	}
	if len(existing) > 0 && existing[0].Cancelled {
		// Already cancelled — idempotent no-op, no revision bump.
		return nil
	}
	// Conflict-safe upsert: a concurrent request cancelling the same occurrence
	// converges on cancelled=true instead of one hitting the unique constraint
	// and returning a 500.
	if _, err := s.cfg.Appointments.CancelAppointmentOccurrence(ctx, appointment.ID, toCalendarDate(occurrenceDate)); err != nil {
		return err
	}
	// A queued create/update notice announces the appointment's first occurrence;
	// removing an occurrence (possibly that first one) could otherwise deliver a
	// mail for a date parents will no longer see. Kill any pending notice, matching
	// the cancel/delete/update stale-notification cleanup.
	if err := s.cancelPendingNotifications(ctx, appointment.ID, "occurrence cancelled"); err != nil {
		return err
	}
	return nil
}

func (s *service) resolveTargets(ctx context.Context, deliveryMode string, targets []AppointmentTarget, allowEmptyParents bool) ([]appointmentcap.AppointmentRecipientFields, []*appointmentcap.AppointmentTarget, error) {
	status := appointmentcap.ResponseStatusPending
	if deliveryMode == appointmentcap.DeliveryModeInformational {
		status = appointmentcap.ResponseStatusInfo
	}
	readSet, err := s.loadTargetResolutionReadSet(ctx, targets)
	if err != nil {
		return nil, nil, err
	}

	recipients := targetRecipients{
		staffIDs:          map[int64]struct{}{},
		guardianStudents:  map[int64]map[int64]struct{}{},
		allowEmptyParents: allowEmptyParents,
	}
	targetRows := make([]*appointmentcap.AppointmentTarget, 0, len(targets))
	for _, target := range targets {
		targetRows = append(targetRows, &appointmentcap.AppointmentTarget{
			TargetType:  target.Type,
			TargetID:    target.ID,
			TargetValue: target.Value,
		})
		if err := recipients.addTarget(readSet, target); err != nil {
			return nil, nil, err
		}
	}
	return recipients.fields(status), targetRows, nil
}

// activeStudentIDs returns the IDs of the students that are currently active. A
// bulk parent target (whole-school, a group, a class) must not fan out to the
// guardians of pending or inactive (e.g. former) students, who would otherwise
// receive appointment details and notifications for a school they left.
func activeStudentIDs(students []*ports.Student) []int64 {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		if student.Status == "active" {
			ids = append(ids, student.ID)
		}
	}
	return ids
}

func normalizeWeekdays(days []string) []string {
	out := make([]string, 0, len(days))
	seen := make(map[string]bool, len(days))
	for _, day := range days {
		normalized := strings.ToLower(strings.TrimSpace(day))
		// De-duplicate: a repeated weekday would make the count-bounded expansion
		// emit that date twice and exhaust occurrence_count early.
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out
}

func recurrenceRuleFromRequest(req *RecurrenceRequest) *appointmentcap.RecurrenceRule {
	if req == nil {
		return nil
	}
	rule := &appointmentcap.RecurrenceRule{
		Frequency:       req.Frequency,
		IntervalCount:   req.IntervalCount,
		Weekdays:        normalizeWeekdays(req.Weekdays),
		MonthDays:       req.MonthDays,
		EndsOn:          toCalendarDatePtr(req.EndsOn),
		OccurrenceCount: req.OccurrenceCount,
	}
	if rule.IntervalCount == 0 {
		rule.IntervalCount = 1
	}
	return rule
}
