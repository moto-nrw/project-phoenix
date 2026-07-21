package calendar

import (
	"context"
	"fmt"

	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/emailbranding"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
)

// OutboxEnqueuer is the slice of the shared email outbox this package needs.
// Nil (unwired) disables e-mail notification; the in-app calendar still works.
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, req platformService.EnqueueRequest) (*platformModels.EmailOutbox, error)
	CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error)
}

// LogoResolver returns the school login image used as e-mail header branding.
// Optional: a nil resolver or a lookup error degrades to the plain fallback.
type LogoResolver interface {
	GetLoginImageURL(ctx context.Context, tenantID int64) (string, error)
}

// Outbox payload keys for the appointment notification e-mails. The mails are
// pointers into the parent portal (title + when + link), never the full body.
const (
	apptPayloadRecipient   = "recipient_email"
	apptPayloadFirstName   = "first_name"
	apptPayloadLastName    = "last_name"
	apptPayloadTitle       = "title"
	apptPayloadWhen        = "when_text"
	apptPayloadLocation    = "location"
	apptPayloadSchoolName  = "school_name"
	apptPayloadPortalURL   = "portal_url"
	apptPayloadLogoURL     = "logo_url"
	apptPayloadMotoLogoURL = "moto_logo_url"
)

// notifyGuardians enqueues one outbox e-mail per reachable guardian recipient of
// the appointment. Best-effort branding (school name / logo) never aborts the
// operation; a failed Enqueue does (it runs in the caller's tenant tx, so the
// whole appointment write rolls back cleanly rather than half-notifying).
func (s *service) notifyGuardians(ctx context.Context, appointment *calModels.Appointment, kind string) error {
	if s.cfg.Outbox == nil {
		return nil
	}
	recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(ctx, appointment.ID)
	if err != nil {
		return err
	}
	guardianIDs := make([]int64, 0, len(recipients))
	for _, recipient := range recipients {
		if recipient.RecipientType == calModels.RecipientTypeGuardianProfile && recipient.GuardianProfileID != nil {
			guardianIDs = append(guardianIDs, *recipient.GuardianProfileID)
		}
	}
	if len(guardianIDs) == 0 {
		return nil
	}
	profiles, err := s.cfg.GuardianProfileRepo.FindActivePortalProfilesByIDs(ctx, guardianIDs)
	if err != nil {
		return err
	}

	schoolName := s.resolveSchoolName(ctx, appointment.TenantID)
	logoURL := s.resolveSchoolLogo(ctx, appointment.TenantID)
	motoLogoURL := emailbranding.MotoLogoURL(s.cfg.ParentsURL)

	// The e-mail advertises the appointment's FIRST occurrence. For a weekly rule
	// whose weekdays exclude the StartDate weekday, that is a later date than
	// StartDate — use the same first-occurrence anchor as the calendar/ICS export
	// so the mail and the calendar never disagree.
	whenAppointment := appointment
	recurrence, err := s.cfg.RecurrenceRepo.FindByAppointmentID(ctx, appointment.ID)
	if err != nil {
		return err
	}
	if recurrence != nil {
		if first, ok := firstRecurrenceOccurrence(appointment, recurrence); ok && first != appointment.StartDate {
			adjusted := *appointment
			adjusted.EndDate = first.AddDays(appointment.StartDate.DaysUntil(appointment.EndDate))
			adjusted.StartDate = first
			whenAppointment = &adjusted
		}
	}
	whenText := appointmentWhenText(whenAppointment)
	location := ""
	if appointment.Location != nil {
		location = *appointment.Location
	}

	seen := make(map[int64]struct{}, len(guardianIDs))
	for _, id := range guardianIDs {
		if _, done := seen[id]; done {
			continue
		}
		seen[id] = struct{}{}
		profile, ok := profiles[id]
		if !ok || profile.Email == nil || *profile.Email == "" {
			continue
		}
		if _, err := s.cfg.Outbox.Enqueue(ctx, platformService.EnqueueRequest{
			Kind: kind,
			Payload: map[string]any{
				apptPayloadRecipient:   *profile.Email,
				apptPayloadFirstName:   profile.FirstName,
				apptPayloadLastName:    profile.LastName,
				apptPayloadTitle:       appointment.Title,
				apptPayloadWhen:        whenText,
				apptPayloadLocation:    location,
				apptPayloadSchoolName:  schoolName,
				apptPayloadPortalURL:   s.cfg.ParentsURL,
				apptPayloadLogoURL:     logoURL,
				apptPayloadMotoLogoURL: motoLogoURL,
			},
			RelatedEntityType: platformModels.EmailRelatedTypeAppointment,
			RelatedEntityID:   appointment.ID,
		}); err != nil {
			return fmt.Errorf("calendar: enqueue appointment e-mail: %w", err)
		}
	}
	return nil
}

// cancelPendingNotifications marks every not-yet-sent appointment e-mail
// (published notice and queued reminders) for this appointment as failed so the
// worker never sends them — used on cancel and delete.
func (s *service) cancelPendingNotifications(ctx context.Context, appointmentID int64, reason string) error {
	if s.cfg.Outbox == nil {
		return nil
	}
	_, err := s.cfg.Outbox.CancelPendingByRelatedEntity(ctx, platformModels.EmailRelatedTypeAppointment, appointmentID, reason)
	return err
}

func (s *service) resolveSchoolName(ctx context.Context, tenantID int64) string {
	if s.cfg.SchoolRepo == nil {
		return ""
	}
	school, err := s.cfg.SchoolRepo.FindByID(ctx, tenantID)
	if err != nil || school == nil {
		return ""
	}
	return school.Name
}

func (s *service) resolveSchoolLogo(ctx context.Context, tenantID int64) string {
	if s.cfg.Settings == nil {
		return ""
	}
	raw, err := s.cfg.Settings.GetLoginImageURL(ctx, tenantID)
	if err != nil {
		return ""
	}
	return emailbranding.SchoolLogoURL(s.cfg.ParentsURL, raw)
}

// appointmentWhenText renders a human-readable German date/time line for the
// e-mail (the appointment's first occurrence).
func appointmentWhenText(appointment *calModels.Appointment) string {
	day := appointment.StartDate.Format("02.01.2006")
	if appointment.AllDay {
		if appointment.EndDate.After(appointment.StartDate) {
			return fmt.Sprintf("%s – %s (ganztägig)", day, appointment.EndDate.Format("02.01.2006"))
		}
		return fmt.Sprintf("%s (ganztägig)", day)
	}
	start := formatClock(appointment.StartTime)
	end := formatClock(appointment.EndTime)
	if appointment.EndDate.After(appointment.StartDate) {
		// Multi-day timed appointment: show the end date too, or the times read as
		// a single-day range (e.g. "18:00–09:00" instead of overnight).
		return fmt.Sprintf("%s, %s Uhr – %s, %s Uhr", day, start, appointment.EndDate.Format("02.01.2006"), end)
	}
	return fmt.Sprintf("%s, %s–%s Uhr", day, start, end)
}
