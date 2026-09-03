package calendar

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailbranding"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// parentCalendarDeepLink is the path the parents app is reachable at on the
// parents host. The portal's own routes carry no /parents prefix in the browser
// (the proxy adds it internally), and a push notification is opened by the
// service worker against that origin — so the link must be the browser path.
const parentCalendarDeepLink = "/calendar"

// OutboxEnqueuer is the slice of the shared email outbox this package needs.
// Nil (unwired) disables e-mail notification; the in-app calendar still works.
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, req platformService.EnqueueRequest) (*platformModels.EmailOutbox, error)
	CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error)
}

// PushOutboxCanceller removes pending durable pushes when an appointment
// lifecycle change makes their snapshot stale.
type PushOutboxCanceller interface {
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

	// The authorization scope the row is sent under, re-checked by the renderer
	// when the mail actually leaves. A reminder can wait in the outbox for the
	// school's whole lead time (24 hours by default), and a guardian's access to
	// the child that put them on the invitation can be revoked in that window.
	apptPayloadGuardianAccountID = "guardian_account_id"
	apptPayloadStudentIDs        = "student_ids"
)

// addGuardianDeliveryScope stamps the row with the account it is addressed to
// and the children of this appointment that account was let through by — the
// same pair the push carries into its delivery transaction. A recipient without
// a portal account contributes no scope; nothing about them can be re-checked,
// and no such recipient reaches this point (reachableGuardianRecipients drops
// them).
func addGuardianDeliveryScope(payload map[string]any, recipients guardianRecipients, guardianProfileID int64) {
	profile, ok := recipients.profiles[guardianProfileID]
	if !ok || profile.AccountID == nil || *profile.AccountID <= 0 {
		return
	}
	studentIDs := recipients.studentsByGuardian[guardianProfileID]
	if len(studentIDs) == 0 {
		return
	}
	payload[apptPayloadGuardianAccountID] = *profile.AccountID
	payload[apptPayloadStudentIDs] = append([]int64(nil), studentIDs...)
}

// notifyGuardians enqueues one outbox e-mail per reachable guardian recipient of
// the appointment. Best-effort branding (school name / logo) never aborts the
// operation; a failed Enqueue does (it runs in the caller's tenant tx, so the
// whole appointment write rolls back cleanly rather than half-notifying).
func (s *service) notifyGuardians(ctx context.Context, appointment *calModels.Appointment, kind string) error {
	if s.cfg.Outbox == nil {
		return nil
	}
	recipients, err := s.reachableGuardianRecipients(ctx, appointment.ID)
	if err != nil {
		return err
	}
	guardianIDs, profiles := recipients.guardianIDs, recipients.profiles
	if len(guardianIDs) == 0 {
		return nil
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
		if first, ok := firstRecurrenceOccurrence(appointment, recurrence); ok && first != toTimezoneDate(appointment.StartDate) {
			adjusted := *appointment
			adjusted.EndDate = toCalendarDate(first.AddDays(appointment.StartDate.DaysUntil(appointment.EndDate)))
			adjusted.StartDate = toCalendarDate(first)
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
		if profile.AccountID != nil && *profile.AccountID > 0 && s.cfg.Preferences != nil {
			notOptedOut, err := s.cfg.Preferences.FilterNotOptedOut(ctx, notifications.TypeParentAppointment, []int64{*profile.AccountID})
			if err != nil {
				return fmt.Errorf("calendar: filter appointment preferences: %w", err)
			}
			if len(notOptedOut) == 0 {
				continue
			}
		}
		payload := map[string]any{
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
		}
		addGuardianDeliveryScope(payload, recipients, id)
		if _, err := s.cfg.Outbox.Enqueue(ctx, platformService.EnqueueRequest{
			Kind:              kind,
			Payload:           payload,
			RelatedEntityType: platformModels.EmailRelatedTypeAppointment,
			RelatedEntityID:   appointment.ID,
		}); err != nil {
			return fmt.Errorf("calendar: enqueue appointment e-mail: %w", err)
		}
	}
	return nil
}

// guardianRecipients is what an appointment's recipient rows resolve to for
// notification purposes: the reachable guardian profile IDs in row order
// (duplicates included — callers de-duplicate as they consume), the profiles
// keyed by ID, and the children each guardian was let through by.
type guardianRecipients struct {
	guardianIDs []int64
	profiles    map[int64]*userModels.GuardianProfile

	// studentsByGuardian records, per guardian profile, the children of this
	// appointment the guardian still holds parent_portal.access for. Push
	// dispatch carries them into its own transaction, which asks the same
	// question again immediately before the payload leaves the backend.
	studentsByGuardian map[int64][]int64
}

// reachableGuardianRecipients resolves the appointment's guardian recipients to
// the profiles that can actually sign in to the parents portal for this school.
//
// Shared by the e-mail path and the push path so both address the same active
// parents-portal profiles. Channel-specific reachability is handled by each
// caller: e-mail requires an address, while push requires an opted-in account.
func (s *service) reachableGuardianRecipients(ctx context.Context, appointmentID int64) (guardianRecipients, error) {
	empty := guardianRecipients{}
	recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(ctx, appointmentID)
	if err != nil {
		return empty, err
	}
	guardianIDs := make([]int64, 0, len(recipients))
	for _, recipient := range recipients {
		if recipient.RecipientType == calModels.RecipientTypeGuardianProfile && recipient.GuardianProfileID != nil {
			guardianIDs = append(guardianIDs, *recipient.GuardianProfileID)
		}
	}
	if len(guardianIDs) == 0 {
		return empty, nil
	}
	profiles, err := s.cfg.GuardianProfileRepo.FindActivePortalProfilesByIDs(ctx, guardianIDs)
	if err != nil {
		return empty, err
	}
	if s.cfg.RecipientStudentRepo == nil || s.cfg.StudentGuardianRepo == nil {
		return empty, errors.New("calendar: guardian permission repositories are required")
	}
	recipientIDs := make([]int64, 0, len(recipients))
	for _, recipient := range recipients {
		if recipient.GuardianProfileID != nil {
			recipientIDs = append(recipientIDs, recipient.ID)
		}
	}
	studentLinks, err := s.cfg.RecipientStudentRepo.FindByRecipientIDs(ctx, recipientIDs)
	if err != nil {
		return empty, err
	}
	studentsByRecipient := make(map[int64][]int64, len(recipientIDs))
	for _, link := range studentLinks {
		studentsByRecipient[link.RecipientID] = append(studentsByRecipient[link.RecipientID], link.StudentID)
	}

	reachable := make([]int64, 0, len(guardianIDs))
	studentsByGuardian := make(map[int64][]int64, len(guardianIDs))
	for _, recipient := range recipients {
		if recipient.GuardianProfileID == nil {
			continue
		}
		profile, ok := profiles[*recipient.GuardianProfileID]
		if !ok || profile.AccountID == nil || *profile.AccountID <= 0 {
			continue
		}
		allowedStudents := make([]int64, 0, len(studentsByRecipient[recipient.ID]))
		for _, studentID := range studentsByRecipient[recipient.ID] {
			allowed, err := s.cfg.StudentGuardianRepo.AccountHasStudentPermission(ctx, *profile.AccountID, studentID, profile.TenantID, authorize.GuardianPermissionPortalAccess)
			if err != nil {
				return empty, fmt.Errorf("calendar: check guardian appointment recipient permission: %w", err)
			}
			if allowed {
				allowedStudents = append(allowedStudents, studentID)
			}
		}
		if len(allowedStudents) == 0 {
			continue
		}
		reachable = append(reachable, *recipient.GuardianProfileID)
		studentsByGuardian[*recipient.GuardianProfileID] = append(studentsByGuardian[*recipient.GuardianProfileID], allowedStudents...)
	}
	return guardianRecipients{guardianIDs: reachable, profiles: profiles, studentsByGuardian: studentsByGuardian}, nil
}

// guardianAccountIDs reduces resolved profiles to their distinct account IDs —
// the address a notification audience is expressed in.
func guardianAccountIDs(guardianIDs []int64, profiles map[int64]*userModels.GuardianProfile) []int64 {
	accountIDs := make([]int64, 0, len(guardianIDs))
	seen := make(map[int64]struct{}, len(guardianIDs))
	for _, id := range guardianIDs {
		profile, ok := profiles[id]
		if !ok || profile.AccountID == nil || *profile.AccountID <= 0 {
			continue
		}
		if _, done := seen[*profile.AccountID]; done {
			continue
		}
		seen[*profile.AccountID] = struct{}{}
		accountIDs = append(accountIDs, *profile.AccountID)
	}
	return accountIDs
}

// guardianStudentGroup is one addressable slice of an appointment's guardian
// audience: the accounts that were let through by exactly the same children.
type guardianStudentGroup struct {
	accountIDs []int64
	studentIDs []int64
	locale     string
}

// guardianStudentGroups splits an addressed audience so every event carries the
// children of ITS recipients, never the appointment-wide union.
//
// The scope travels to the device lookup and the SSE fan-out, which keep an
// account that still sees at least one named child. With a union, an account
// that lost access to the child it was invited for would survive that check
// through a child of a different recipient it happens to be a guardian of —
// a push about an appointment the parents portal would then refuse to show.
// Grouping keeps the recheck as narrow as the resolution that produced it, and
// costs one extra event only where recipients genuinely differ.
func guardianStudentGroups(recipients guardianRecipients, accountIDs []int64) []guardianStudentGroup {
	groups := make([]guardianStudentGroup, 0, len(accountIDs))
	index := make(map[string]int, len(accountIDs))
	for _, accountID := range accountIDs {
		studentIDs := guardianStudentIDs(recipients, []int64{accountID})
		if len(studentIDs) == 0 {
			continue
		}
		locale := guardianAccountLocale(recipients, accountID)
		key := locale + ":" + guardianStudentGroupKey(studentIDs)
		if at, ok := index[key]; ok {
			groups[at].accountIDs = append(groups[at].accountIDs, accountID)
			continue
		}
		index[key] = len(groups)
		groups = append(groups, guardianStudentGroup{
			accountIDs: []int64{accountID},
			studentIDs: studentIDs,
			locale:     locale,
		})
	}
	return groups
}

func guardianAccountLocale(recipients guardianRecipients, accountID int64) string {
	for _, guardianID := range recipients.guardianIDs {
		profile, ok := recipients.profiles[guardianID]
		if !ok || profile.AccountID == nil || *profile.AccountID != accountID {
			continue
		}
		if profile.PortalLocale != nil {
			return *profile.PortalLocale
		}
		return ""
	}
	return ""
}

// guardianStudentGroupKey identifies a set of children independently of the
// order they were resolved in, so two accounts with the same children share one
// event instead of producing one each.
func guardianStudentGroupKey(studentIDs []int64) string {
	sorted := slices.Clone(studentIDs)
	slices.Sort(sorted)
	parts := make([]string, len(sorted))
	for i, studentID := range sorted {
		parts[i] = strconv.FormatInt(studentID, 10)
	}
	return strings.Join(parts, ",")
}

// guardianStudentIDs collects the children the addressed accounts were let
// through by — the authorization scope a push carries into its own delivery
// transaction, where the access question is asked one last time.
//
// Callers address one account at a time (the reminder) or group accounts by
// their children first (the lifecycle path), so the returned scope never
// exceeds what its own recipients were let through by.
func guardianStudentIDs(recipients guardianRecipients, accountIDs []int64) []int64 {
	addressed := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		addressed[accountID] = struct{}{}
	}
	studentIDs := make([]int64, 0, len(recipients.studentsByGuardian))
	seen := make(map[int64]struct{}, len(recipients.studentsByGuardian))
	for _, guardianID := range recipients.guardianIDs {
		profile, ok := recipients.profiles[guardianID]
		if !ok || profile.AccountID == nil {
			continue
		}
		if _, ok := addressed[*profile.AccountID]; !ok {
			continue
		}
		for _, studentID := range recipients.studentsByGuardian[guardianID] {
			if _, done := seen[studentID]; done {
				continue
			}
			seen[studentID] = struct{}{}
			studentIDs = append(studentIDs, studentID)
		}
	}
	return studentIDs
}

func localizedAppointmentNotificationCopy(kind, locale string) (title, body string) {
	notificationKind := notifications.ParentAppointmentPublished
	switch kind {
	case platformModels.EmailKindAppointmentUpdated:
		notificationKind = notifications.ParentAppointmentUpdated
	case platformModels.EmailKindAppointmentCancelled:
		notificationKind = notifications.ParentAppointmentCancelled
	case platformModels.EmailKindAppointmentReminder:
		notificationKind = notifications.ParentAppointmentReminder
	}
	return notifications.ParentAppointmentCopy(locale, notificationKind)
}

// notifyGuardianDevices is the push/in-app counterpart of notifyGuardians: same
// audience, same trigger, different channel. It is called only where the e-mail
// is also sent, so the per-appointment "Eltern benachrichtigen" opt-in governs
// both — an organizer who chose not to notify does not get to notify anyway
// through a second channel.
//
// Never fatal. The appointment write is the operation the user asked for; a
// notification that could not be dispatched is logged and dropped, exactly like
// SSE broadcasting elsewhere in the codebase.
func (s *service) notifyGuardianDevices(ctx context.Context, appointment *calModels.Appointment, kind string) {
	if s.cfg.Notifier == nil {
		return
	}
	appointmentCopy := *appointment
	postCommitCtx := tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
	tenant.RegisterAfterCommit(ctx, func() {
		s.dispatchGuardianDevicesAfterCommit(postCommitCtx, &appointmentCopy, kind)
	})
}

// dispatchGuardianDevicesAfterCommit rechecks the relationship-scoped parent
// permission after the lifecycle write commits, immediately before notification
// dispatch. Notifications use account IDs on the wire, so retaining an earlier
// account-only audience would otherwise bypass a just-revoked student link.
func (s *service) dispatchGuardianDevicesAfterCommit(ctx context.Context, appointment *calModels.Appointment, kind string) {
	var accountIDs []int64
	err := tenant.WithTenantTx(ctx, s.cfg.DB, appointment.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		current, err := s.cfg.AppointmentRepo.FindByID(txCtx, appointment.ID)
		if err != nil {
			return err
		}
		if current == nil || current.DeletedAt != nil {
			return nil
		}
		if kind != platformModels.EmailKindAppointmentCancelled && (current.CancelledAt != nil || !current.NotifyGuardians) {
			return nil
		}
		recipients, err := s.reachableGuardianRecipients(txCtx, current.ID)
		if err != nil {
			return err
		}
		accountIDs = guardianAccountIDs(recipients.guardianIDs, recipients.profiles)
		for _, group := range guardianStudentGroups(recipients, accountIDs) {
			if _, err := s.dispatchGuardianAccountDevicesLocalized(txCtx, current, kind, group.accountIDs, group.studentIDs, group.locale); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, notifications.ErrDisabled) || errors.Is(err, notifications.ErrOutsideActiveWindow) {
		s.logger().Info("calendar: appointment push suppressed by tenant notification gate",
			slog.Int64("appointment_id", appointment.ID),
			slog.Int("recipient_count", len(accountIDs)),
			slog.String("reason", err.Error()),
		)
	} else if err != nil {
		s.logger().Warn("calendar: queue guardian appointment push failed",
			slog.Int64("appointment_id", appointment.ID),
			slog.String("error", err.Error()),
		)
	}
}

func (s *service) dispatchGuardianAccountDevicesLocalized(ctx context.Context, appointment *calModels.Appointment, kind string, accountIDs, studentIDs []int64, locale string) (bool, error) {
	if s.cfg.Notifier == nil || len(accountIDs) == 0 {
		return false, nil
	}
	notificationType := notifications.TypeParentAppointment
	if kind == platformModels.EmailKindAppointmentReminder {
		notificationType = notifications.TypeParentAppointmentReminder
	}

	// Consent narrows the audience the appointment already defined, never widens
	// it. A missing preference service means the dependency was not wired; the
	// factory always wires it, so treat that as "do not push" rather than
	// pushing to people who never agreed.
	if s.cfg.Preferences == nil {
		return false, nil
	}
	accountIDs, err := s.cfg.Preferences.FilterOptedIn(ctx, notificationType, accountIDs)
	if err != nil {
		return false, fmt.Errorf("filter opted-in guardians: %w", err)
	}
	if len(accountIDs) == 0 {
		return false, nil
	}

	title, body := localizedAppointmentNotificationCopy(kind, locale)
	priority := notifications.PriorityNormal
	if kind == platformModels.EmailKindAppointmentCancelled {
		priority = notifications.PriorityHigh
	}

	err = s.cfg.Notifier.Notify(ctx, notifications.Event{
		Type:           notificationType,
		IdempotencyKey: fmt.Sprintf("appointment:%d:%d:%s", appointment.ID, appointment.Revision, kind),
		RelatedType:    "appointment", RelatedID: appointment.ID,
		Title:    title,
		Body:     body,
		DeepLink: parentCalendarDeepLink,
		Priority: priority,
		Audience: notifications.Audience{
			TenantID:           appointment.TenantID,
			Scope:              notifications.ScopeGuardian,
			GuardianAccountIDs: accountIDs,
			StudentIDs:         studentIDs,
		},
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *service) dispatchGuardianAccountReminderDevicesLocalized(ctx context.Context, appointment *calModels.Appointment, accountIDs, studentIDs []int64, locale string) (bool, error) {
	if s.cfg.ReminderNotifier == nil || len(accountIDs) == 0 {
		return false, nil
	}
	title, body := localizedAppointmentNotificationCopy(platformModels.EmailKindAppointmentReminder, locale)
	if err := s.cfg.ReminderNotifier.NotifySynchronously(ctx, notifications.Event{
		Type: notifications.TypeParentAppointmentReminder, Title: title, Body: body,
		DeepLink: parentCalendarDeepLink, Priority: notifications.PriorityNormal,
		Audience: notifications.Audience{
			TenantID: appointment.TenantID, Scope: notifications.ScopeGuardian,
			GuardianAccountIDs: accountIDs, StudentIDs: studentIDs,
		},
	}); err != nil {
		return false, err
	}
	return true, nil
}

func (s *service) logger() *slog.Logger {
	if s.cfg.Logger == nil {
		return slog.Default()
	}
	return s.cfg.Logger
}

// cancelPendingNotifications removes every not-yet-sent appointment delivery
// whose snapshot has become stale after an appointment lifecycle change.
func (s *service) cancelPendingNotifications(ctx context.Context, appointmentID int64, reason string) error {
	if s.cfg.Outbox != nil {
		if _, err := s.cfg.Outbox.CancelPendingByRelatedEntity(ctx, platformModels.EmailRelatedTypeAppointment, appointmentID, reason); err != nil {
			return err
		}
	}
	if s.cfg.PushOutbox != nil {
		_, err := s.cfg.PushOutbox.CancelPendingByRelatedEntity(ctx, "appointment", appointmentID, reason)
		return err
	}
	return nil
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
