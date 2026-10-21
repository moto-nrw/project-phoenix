package compose_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/appointments"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	calendarSvc "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func deactivateAccountTenant(t *testing.T, db *bun.DB, accountID, tenantID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("auth.account_tenants").
		Set("status = ?", "inactive").
		Set("deactivated_at = NOW()").
		Set("staff_calendar_feed_token = NULL").
		Set("updated_at = NOW()").
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", tenantID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

func calendarTestConfig(t *testing.T, db *bun.DB) calendarCompose.Config {
	t.Helper()
	_, reposPeople, reposCalendar, reposTimetable, reposAppointments, reposParents, reposRooms := calendarTestRepositories(t, db)
	identity, err := api.NewUserContextTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	userContext := identity.Caller()

	cfg := owners.NewCalendarFacts(reposPeople.Staff, reposPeople.Student,
		owners.NewCalendarGuardianDirectory(reposPeople.GuardianProfile, owners.MustNewPeopleDirectory(testpkg.MustDatabase(db))),
		reposPeople.StudentGuardian, reposParents.ParentChildLinks(), owners.NewSchoolStructureSuiteRepositories(testpkg.NewDatabase(t, db)).Group)
	cfg = owners.BindCalendarScheduleFacts(cfg, reposTimetable.InstanceStaff, reposTimetable.ActivityInstance.Reads, reposRooms.Room, reposTimetable.StaffShift, reposTimetable.ShiftType)
	cfg = owners.BindCalendarCaller(cfg, userContext)
	cfg.Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	cfg.Today = func() appointments.Date { return appointments.Date(calendar.NewDate(2026, time.January, 1)) }
	cfg.Appointments = reposAppointments.Appointments()
	cfg.Runtime = calendarCompose.NewTenantRuntime(db)
	cfg.CalendarRenderer = calendarCompose.NewCalendarRenderer(reposCalendar.SchoolCalendar())
	return cfg
}
func setupCalendarService(t *testing.T, db *bun.DB) calendarSvc.Service {
	t.Helper()
	return owners.NewCalendarPortal(calendarTestConfig(t, db))
}

func setupCalendarServiceWithOutbox(t *testing.T, db *bun.DB, outbox owners.CalendarOutbox) calendarCompose.Application {
	t.Helper()
	cfg := calendarTestConfig(t, db)
	cfg.Outbox = outbox
	cfg.ParentsURL = "https://parents.test"
	return owners.NewCalendarPortal(cfg)
}

var errInjectedRecurrenceCreate = errors.New("injected recurrence create failure")
var errInjectedOverrideDelete = errors.New("injected occurrence override delete failure")

type failingRecurrenceCreate struct {
	appointments.Capability
}

func (failingRecurrenceCreate) CreateRecurrenceRule(context.Context, *appointments.RecurrenceRule) error {
	return errInjectedRecurrenceCreate
}

type failingOverrideDelete struct {
	appointments.Capability
}

func (failingOverrideDelete) DeleteOccurrenceOverrides(context.Context, int64) error {
	return errInjectedOverrideDelete
}

type recordingAppointmentCreate struct {
	appointments.Capability
	appointmentID int64
	targets       int
}

func (r *recordingAppointmentCreate) CreateAppointment(ctx context.Context, input appointments.CreateAppointment) (*appointments.Appointment, []*appointments.AppointmentTarget, error) {
	appointment, targets, err := r.Capability.CreateAppointment(ctx, input)
	if err == nil {
		r.appointmentID = appointment.ID
		r.targets = len(targets)
	}
	return appointment, targets, err
}

type recordingOutbox struct {
	enqueued  []calendarCompose.EnqueueRequest
	cancelled int
	nextID    int64
	keys      map[string]struct{}
}

func (r *recordingOutbox) Enqueue(_ context.Context, req calendarCompose.EnqueueRequest) (*calendarCompose.EmailOutbox, error) {
	if req.IdempotencyKey != "" {
		if r.keys == nil {
			r.keys = make(map[string]struct{})
		}
		if _, exists := r.keys[req.IdempotencyKey]; exists {
			return &calendarCompose.EmailOutbox{}, nil
		}
		r.keys[req.IdempotencyKey] = struct{}{}
	}
	r.enqueued = append(r.enqueued, req)
	r.nextID++
	return &calendarCompose.EmailOutbox{ID: r.nextID}, nil
}

func (r *recordingOutbox) CancelPendingByRelatedEntity(context.Context, string, int64, string) (int64, error) {
	r.cancelled++
	return 0, nil
}

func calendarContext(tb testing.TB, accountID int64) context.Context {
	tb.Helper()
	return context.WithValue(testpkg.Ctx(tb), sessiontoken.CtxClaims, sessiontoken.AppClaims{
		ID:       int(accountID),
		TenantID: testpkg.Tenant(tb),
	})
}

type fixedCalDAVSettings struct {
	enabled bool
	err     error
}

func (s fixedCalDAVSettings) Enabled(context.Context) (bool, error) {
	return s.enabled, s.err
}

func (s fixedCalDAVSettings) EnabledForTenant(context.Context, int64) (bool, error) {
	return s.enabled, s.err
}

func wallClock(h, m int) time.Time {
	return calendar.NormalizeWallClock(time.Date(2024, 1, 1, h, m, 0, 0, time.UTC))
}

func eventDates(events []calendarSvc.Event, source string) []string {
	dates := make([]string, 0, len(events))
	for _, event := range events {
		if event.Source == source {
			dates = append(dates, event.StartDate)
		}
	}
	sort.Strings(dates)
	return dates
}

func findRecipientByStaff(t *testing.T, detail *calendarSvc.AppointmentDetail, staffID int64) int64 {
	t.Helper()
	for _, recipient := range detail.Recipients {
		if recipient.StaffID != nil && *recipient.StaffID == staffID {
			return recipient.ID
		}
	}
	t.Fatalf("staff recipient %d not found", staffID)
	return 0
}

func findRecipientByGuardian(t *testing.T, detail *calendarSvc.AppointmentDetail, guardianID int64) int64 {
	t.Helper()
	id := findOptionalRecipientByGuardian(detail, guardianID)
	require.NotZero(t, id)
	return id
}

func findOptionalRecipientByGuardian(detail *calendarSvc.AppointmentDetail, guardianID int64) int64 {
	for _, recipient := range detail.Recipients {
		if recipient.GuardianProfileID != nil && *recipient.GuardianProfileID == guardianID {
			return recipient.ID
		}
	}
	return 0
}

func TestCalendarServiceIntegration_CreateRecurringAppointmentAndResponses(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Calendar", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Calendar", "Invitee")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	endsOn := portalDate(2026, 1, 12)
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Recurring planning",
		Description:  testpkg.StrPtr("Weekly planning with staff and parents"),
		Location:     testpkg.StrPtr("Room 1"),
		StartDate:    portalDate(2026, 1, 5),
		EndDate:      portalDate(2026, 1, 5),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeRSVPRequired,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday", "wednesday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, detail)

	assert.Equal(t, organizer.ID, detail.Appointment.OrganizerStaffID)
	require.Len(t, detail.Recipients, 2)
	require.Len(t, detail.Targets, 2)
	require.NotNil(t, detail.Recurrence)
	assert.Equal(t, []string{"monday", "wednesday"}, detail.Recurrence.Weekdays)

	staffRecipientID := findRecipientByStaff(t, detail, invitedStaff.ID)
	guardianRecipientID := findRecipientByGuardian(t, detail, parentChain.GuardianProfileID)

	staffEvents, err := testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 1, 5), portalDate(2026, 1, 12))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-07", "2026-01-12"}, eventDates(staffEvents, calendarSvc.EventSourceAppointment))
	require.NotEmpty(t, staffEvents)
	assert.Equal(t, appointments.ResponseStatusPending, *staffEvents[0].ResponseStatus)
	assert.True(t, staffEvents[0].CanRespond)

	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, invitedAccount.ID), db, service.RespondToStaffInvitation, staffRecipientID, appointments.ResponseStatusAccepted))
	staffEvents, err = testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 1, 5), portalDate(2026, 1, 5))
	require.NoError(t, err)
	require.NotEmpty(t, staffEvents)
	assert.Equal(t, appointments.ResponseStatusAccepted, *staffEvents[0].ResponseStatus)

	parentEvents, err := service.ListMyParentEvents(testpkg.Ctx(t), parentChain.AccountID, portalDate(2026, 1, 5), portalDate(2026, 1, 12))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-07", "2026-01-12"}, eventDates(parentEvents, calendarSvc.EventSourceAppointment))
	require.NotEmpty(t, parentEvents)
	assert.Equal(t, appointments.ResponseStatusPending, *parentEvents[0].ResponseStatus)
	assert.True(t, parentEvents[0].CanRespond)

	require.NoError(t, service.RespondToParentInvitation(testpkg.Ctx(t), parentChain.AccountID, guardianRecipientID, appointments.ResponseStatusDeclined))
	parentEvents, err = service.ListMyParentEvents(testpkg.Ctx(t), parentChain.AccountID, portalDate(2026, 1, 5), portalDate(2026, 1, 5))
	require.NoError(t, err)
	require.NotEmpty(t, parentEvents)
	assert.Equal(t, appointments.ResponseStatusDeclined, *parentEvents[0].ResponseStatus)
}

func TestCalendarServiceIntegration_InformationalAppointmentCannotBeAnswered(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Info", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Info", "Invitee")

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Information only",
		StartDate:    portalDate(2026, 2, 2),
		EndDate:      portalDate(2026, 2, 2),
		StartTime:    wallClock(12, 0),
		EndTime:      wallClock(12, 30),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)

	recipientID := findRecipientByStaff(t, detail, invitedStaff.ID)
	err = testpkg.TenantCommand2(t, calendarContext(t, invitedAccount.ID), db, service.RespondToStaffInvitation, recipientID, appointments.ResponseStatusAccepted)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
}

func TestCalendarServiceIntegration_UpdateCancelDeleteLifecycle(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Lifecycle", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Lifecycle", "Invitee")

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Original title",
		Location:     testpkg.StrPtr("Room 1"),
		StartDate:    portalDate(2026, 3, 2),
		EndDate:      portalDate(2026, 3, 2),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeRSVPRequired,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)

	// A non-organizer may not edit the appointment.
	_, err = testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.UpdateStaffAppointment, detail.Appointment.ID, calendarSvc.UpdateAppointmentRequest{
		Title:     "Hijack",
		StartDate: portalDate(2026, 3, 2),
		EndDate:   portalDate(2026, 3, 2),
		StartTime: wallClock(9, 0),
		EndTime:   wallClock(10, 0),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrForbidden))

	// The organizer edits event details; recipients (RSVP audience) are preserved.
	updated, err := testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.UpdateStaffAppointment, detail.Appointment.ID, calendarSvc.UpdateAppointmentRequest{
		Title:     "Updated title",
		Location:  testpkg.StrPtr("Room 2"),
		StartDate: portalDate(2026, 3, 3),
		EndDate:   portalDate(2026, 3, 3),
		StartTime: wallClock(11, 0),
		EndTime:   wallClock(12, 0),
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated title", updated.Appointment.Title)
	require.Len(t, updated.Recipients, 1)

	// The organizer can re-read the full detail (used to prefill the edit modal).
	detailAfter, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.GetStaffAppointmentDetail, detail.Appointment.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated title", detailAfter.Appointment.Title)
	assert.Equal(t, "Room 2", *detailAfter.Appointment.Location)
	// A non-organizer is refused.
	_, err = testpkg.TenantQuery1(t, calendarContext(t, invitedAccount.ID), db, service.GetStaffAppointmentDetail, detail.Appointment.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrForbidden))

	events, err := testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 3, 1), portalDate(2026, 3, 10))
	require.NoError(t, err)
	require.NotEmpty(t, events)
	assert.Equal(t, "Updated title", events[0].Title)
	assert.Equal(t, "2026-03-03", events[0].StartDate)
	assert.False(t, events[0].Cancelled)

	// Cancelling marks the event but keeps it visible so parents/staff see it.
	_, err = testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointment, detail.Appointment.ID)
	require.NoError(t, err)
	events, err = testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 3, 1), portalDate(2026, 3, 10))
	require.NoError(t, err)
	require.NotEmpty(t, events)
	assert.True(t, events[0].Cancelled)

	// Deleting removes it from every reader.
	require.NoError(t, testpkg.TenantCommand1(t, calendarContext(t, organizerAccount.ID), db, service.DeleteStaffAppointment, detail.Appointment.ID))
	events, err = testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 3, 1), portalDate(2026, 3, 10))
	require.NoError(t, err)
	assert.Empty(t, eventDates(events, calendarSvc.EventSourceAppointment))
}

func TestCalendarServiceIntegration_GuardianNotifications(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Notify", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	// Create WITHOUT send_email: no mail is queued.
	_, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Silent",
		StartDate:    portalDate(2026, 4, 1),
		EndDate:      portalDate(2026, 4, 1),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	assert.Empty(t, outbox.enqueued, "no email should be queued without send_email")

	// Create WITH send_email: one published mail to the guardian.
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Elternabend",
		Location:     testpkg.StrPtr("Aula"),
		StartDate:    portalDate(2026, 4, 2),
		EndDate:      portalDate(2026, 4, 2),
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(19, 30),
		DeliveryMode: appointments.DeliveryModeInformational,
		SendEmail:    true,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	require.Len(t, outbox.enqueued, 1)
	published := outbox.enqueued[0]
	assert.Equal(t, emailKindAppointmentPublished, published.Kind)
	assert.Equal(t, emailRelatedTypeAppointment, published.RelatedEntityType)
	assert.Equal(t, detail.Appointment.ID, published.RelatedEntityID)
	assert.Equal(t, "Elternabend", published.Payload["title"])
	assert.NotEmpty(t, published.Payload["recipient_email"])

	// Cancelling queues a cancellation notice and clears pending mail.
	_, err = testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointment, detail.Appointment.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, outbox.cancelled, "cancel should clear pending mail once")
	require.Len(t, outbox.enqueued, 2)
	assert.Equal(t, emailKindAppointmentCancelled, outbox.enqueued[1].Kind)
}

func TestCalendarServiceIntegration_CancelHonoursEmailOptOutAndTransition(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	_, _, _, _, reposAppointments, _, _ := calendarTestRepositories(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "OptOut", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	newSilentAppointment := func(title string, day int) *calendarSvc.AppointmentDetail {
		detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
			Title:        title,
			StartDate:    portalDate(2026, 4, day),
			EndDate:      portalDate(2026, 4, day),
			StartTime:    wallClock(9, 0),
			EndTime:      wallClock(10, 0),
			DeliveryMode: appointments.DeliveryModeInformational,
			// No send_email: the guardian opted out of mail for this appointment.
			Targets: []calendarSvc.AppointmentTarget{
				{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
			},
		})
		require.NoError(t, err)
		return detail
	}

	// Cancelling a send_email=false appointment must not mail guardians.
	silent := newSilentAppointment("Stiller Termin", 1)
	require.Empty(t, outbox.enqueued)
	_, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointment, silent.Appointment.ID)
	require.NoError(t, err)
	assert.Empty(t, outbox.enqueued, "a send_email=false appointment must stay silent on cancellation")

	// The repo Cancel reports the transition exactly once — the mechanism that lets
	// concurrent callers gate notifications so no duplicate cancellation e-mail is
	// sent.
	fresh := newSilentAppointment("Zweiter", 2)
	first, err := reposAppointments.Appointments().CancelAppointment(testpkg.Ctx(t), fresh.Appointment.ID)
	require.NoError(t, err)
	assert.True(t, first, "first cancel performs the transition")
	second, err := reposAppointments.Appointments().CancelAppointment(testpkg.Ctx(t), fresh.Appointment.ID)
	require.NoError(t, err)
	assert.False(t, second, "a second (concurrent) cancel does not re-transition, so it must not notify")
}

func TestCalendarServiceIntegration_CancelAfterDeleteDoesNotTransition(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, _, _, _, reposAppointments, _, _ := calendarTestRepositories(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "DeleteRace", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	// send_email=true, so a successful cancellation WOULD notify guardians.
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Wird gelöscht",
		StartDate:    portalDate(2026, 5, 1),
		EndDate:      portalDate(2026, 5, 1),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		SendEmail:    true,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	// Simulate a delete landing after a cancel request loaded the still-live
	// appointment: soft-delete it directly.
	require.NoError(t, reposAppointments.Appointments().SoftDeleteAppointment(testpkg.Ctx(t), detail.Appointment.ID))

	// The cancel transition must now match zero rows (deleted_at guard) and report
	// no transition, so CancelStaffAppointment skips the guardian notice.
	transitioned, err := reposAppointments.Appointments().CancelAppointment(testpkg.Ctx(t), detail.Appointment.ID)
	require.NoError(t, err)
	assert.False(t, transitioned, "cancel must not transition an appointment a concurrent delete removed")

	after, err := reposAppointments.Appointments().FindAppointment(testpkg.Ctx(t), detail.Appointment.ID)
	require.NoError(t, err)
	require.NotNil(t, after)
	assert.NotNil(t, after.DeletedAt, "the appointment stays deleted")
	assert.Nil(t, after.CancelledAt, "and was not silently flipped to cancelled")
}

func TestCalendarServiceIntegration_AppointmentICS(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "ICS", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	endsOn := portalDate(2026, 5, 25)
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Wöchentliche AG",
		Location:     testpkg.StrPtr("Turnhalle"),
		StartDate:    portalDate(2026, 5, 4), // Monday
		EndDate:      portalDate(2026, 5, 4),
		StartTime:    wallClock(14, 0),
		EndTime:      wallClock(15, 30),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	// Organizer can export.
	filename, content, err := testpkg.TenantQueryPair1(t, calendarContext(t, organizerAccount.ID), db, service.StaffAppointmentICS, detail.Appointment.ID)
	require.NoError(t, err)
	assert.Equal(t, "Wchentliche-AG.ics", filename)
	assert.Contains(t, content, "BEGIN:VEVENT")
	assert.Contains(t, content, "SUMMARY:Wöchentliche AG")
	assert.Contains(t, content, "LOCATION:Turnhalle")
	assert.Contains(t, content, "DTSTART;TZID=Europe/Berlin:20260504T140000")
	assert.Contains(t, content, "RRULE:FREQ=WEEKLY;BYDAY=MO;UNTIL=20260525T")
	assert.NotContains(t, content, "EXDATE", "no exclusions before any occurrence is cancelled")

	// Cancelling a single occurrence emits an EXDATE so subscribed calendars
	// drop that date (matching the in-app view).
	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointmentOccurrence, detail.Appointment.ID, portalDate(2026, 5, 11)))
	_, contentAfter, err := testpkg.TenantQueryPair1(t, calendarContext(t, organizerAccount.ID), db, service.StaffAppointmentICS, detail.Appointment.ID)
	require.NoError(t, err)
	assert.Contains(t, contentAfter, "EXDATE;TZID=Europe/Berlin:20260511T140000")

	// The invited guardian can export, and also sees the exclusion.
	_, parentContent, err := service.ParentAppointmentICS(testpkg.Ctx(t), parentChain.AccountID, detail.Appointment.ID)
	require.NoError(t, err)
	assert.Contains(t, parentContent, "SUMMARY:Wöchentliche AG")
	assert.Contains(t, parentContent, "EXDATE;TZID=Europe/Berlin:20260511T140000")

	// A stranger staff member cannot.
	_, otherAccount := testpkg.CreateTestCalendarStaff(t, db, "ICS", "Stranger")
	_, _, err = testpkg.TenantQueryPair1(t, calendarContext(t, otherAccount.ID), db, service.StaffAppointmentICS, detail.Appointment.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))
}

func TestCalendarServiceIntegration_SubscriptionFeed(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	cfg := calendarTestConfig(t, db)
	repos, _, _, _, _, _, _ := calendarTestRepositories(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, nil, nil)
	cfg.ParentsURL = "https://parents.test"
	// The feed counts its window from today; the appointments below sit on
	// fixed dates, so today is pinned to them instead of the wall clock.
	cfg.Today = func() appointments.Date { return portalDate(2026, 8, 24) }
	service := owners.NewCalendarPortal(cfg)

	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Feed", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Sommerfest",
		StartDate:    portalDate(2026, 8, 24).AddDays(7),
		EndDate:      portalDate(2026, 8, 24).AddDays(7),
		StartTime:    wallClock(15, 0),
		EndTime:      wallClock(18, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	// First request generates the token and returns the subscription URLs once.
	httpsURL, webcalURL, err := service.ParentCalendarFeedURL(testpkg.Ctx(t), parentChain.AccountID)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(httpsURL, "https://parents.test/api/calendar-feed/"))
	assert.True(t, strings.HasPrefix(webcalURL, "webcal://parents.test/api/calendar-feed/"))

	// Show-once: only the token hash is stored, so a second request cannot
	// re-display the raw URL — it returns empty (the UI then offers a rotate).
	httpsURL2, webcalURL2, err := service.ParentCalendarFeedURL(testpkg.Ctx(t), parentChain.AccountID)
	require.NoError(t, err)
	assert.Empty(t, httpsURL2)
	assert.Empty(t, webcalURL2)

	token := strings.TrimPrefix(httpsURL, "https://parents.test/api/calendar-feed/")

	// Security: the DB stores only the SHA-256 hash of the token, never the raw
	// capability token — a DB read/backup exposes no replayable feed URL.
	var storedFeedToken string
	require.NoError(t, db.NewSelect().
		Table("auth.accounts").
		Column("calendar_feed_token").
		Where("id = ?", parentChain.AccountID).
		Scan(context.Background(), &storedFeedToken))
	rawSum := sha256.Sum256([]byte(token))
	assert.Equal(t, hex.EncodeToString(rawSum[:]), storedFeedToken, "only the token hash is persisted")
	assert.NotEqual(t, token, storedFeedToken, "the raw token must never be stored")

	filename, content, err := service.ParentCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.Equal(t, "moto-kalender.ics", filename)
	assert.Contains(t, content, "BEGIN:VCALENDAR")
	assert.Contains(t, content, "SUMMARY:Sommerfest")
	assert.NotContains(t, content, "EXDATE")

	// A recurring appointment with a cancelled single occurrence emits an EXDATE
	// in the feed, so subscribers drop that date.
	recurStart := portalDate(2026, 8, 24).AddDays(7)
	recurEnd := recurStart.AddDays(28)
	recurring, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Wöchentliches Treffen",
		StartDate:    recurStart,
		EndDate:      recurStart,
		StartTime:    wallClock(16, 0),
		EndTime:      wallClock(17, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{strings.ToLower(recurStart.Weekday().String())},
			EndsOn:        &recurEnd,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointmentOccurrence, recurring.Appointment.ID, recurStart.AddDays(7)))

	_, contentWithExdate, err := service.ParentCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.Contains(t, contentWithExdate, "SUMMARY:Wöchentliches Treffen")
	assert.Contains(t, contentWithExdate, "EXDATE;TZID=Europe/Berlin:")

	// An unknown token is a plain not-found.
	_, _, err = service.ParentCalendarFeedByToken(testpkg.Ctx(t), "nope-not-a-real-token")
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	// Rotation invalidates the old token.
	httpsURL3, _, err := service.RotateParentCalendarFeed(testpkg.Ctx(t), parentChain.AccountID)
	require.NoError(t, err)
	assert.NotEqual(t, httpsURL, httpsURL3)
	_, _, err = service.ParentCalendarFeedByToken(testpkg.Ctx(t), token)
	require.Error(t, err)
}

func TestCalendarServiceIntegration_StaffSubscriptionFeed(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos, reposPeople, reposCalendar, _, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, repos.StaffCalendarFeedToken, reposPeople.Person)
	cfg.StaffFeedTombstoneRepo = reposCalendar.CalendarStaffFeedTombstone
	cfg.FrontendURL = "https://moto.test"
	service := owners.NewCalendarPortal(cfg)

	_, account := testpkg.CreateTestCalendarStaff(t, db, "Feed", "Mitarbeiter")
	_, err := testpkg.TenantQuery1(t, calendarContext(t, account.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Teamsitzung",
		StartDate:    calendarSvc.Date(calendar.NewDate(2026, time.January, 1)).AddDays(7),
		EndDate:      calendarSvc.Date(calendar.NewDate(2026, time.January, 1)).AddDays(7),
		StartTime:    wallClock(14, 0),
		EndTime:      wallClock(15, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
	})
	require.NoError(t, err)

	httpsURL, webcalURL, err := testpkg.TenantQueryPair0(t, calendarContext(t, account.ID), db, service.StaffCalendarFeedURL)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(httpsURL, "https://moto.test/api/calendar-feed/"))
	assert.True(t, strings.HasPrefix(webcalURL, "webcal://moto.test/api/calendar-feed/"))

	httpsURLAgain, webcalURLAgain, err := testpkg.TenantQueryPair0(t, calendarContext(t, account.ID), db, service.StaffCalendarFeedURL)
	require.NoError(t, err)
	assert.Empty(t, httpsURLAgain)
	assert.Empty(t, webcalURLAgain)

	token := strings.TrimPrefix(httpsURL, "https://moto.test/api/calendar-feed/")
	var storedHash string
	require.NoError(t, db.NewSelect().
		Table("auth.account_tenants").
		Column("staff_calendar_feed_token").
		Where("account_id = ?", account.ID).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Scan(context.Background(), &storedHash))
	rawSum := sha256.Sum256([]byte(token))
	assert.Equal(t, hex.EncodeToString(rawSum[:]), storedHash)
	assert.NotEqual(t, token, storedHash)

	filename, content, err := service.StaffCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.Equal(t, "moto-kalender.ics", filename)
	assert.Contains(t, content, "SUMMARY:Teamsitzung")
}

func TestCalendarServiceIntegration_StaffCalDAVUsesSharedReadOnlyProjectionAndToken(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos, reposPeople, reposCalendar, _, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, repos.StaffCalendarFeedToken, reposPeople.Person)
	cfg.StaffFeedTombstoneRepo = reposCalendar.CalendarStaffFeedTombstone
	cfg.CalDAVPolicy = fixedCalDAVSettings{enabled: true}
	cfg.FrontendURL = "https://moto.test"
	cfg.CalDAVURL = "https://api.moto.test"
	service := owners.NewCalendarPortal(cfg)

	_, account := testpkg.CreateTestCalendarStaff(t, db, "CalDAV", "Mitarbeiter")
	_, err := testpkg.TenantQuery1(t, calendarContext(t, account.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "CalDAV Teamsitzung",
		StartDate:    calendarSvc.Date(calendar.NewDate(2026, time.January, 1)).AddDays(7),
		EndDate:      calendarSvc.Date(calendar.NewDate(2026, time.January, 1)).AddDays(7),
		StartTime:    wallClock(14, 0),
		EndTime:      wallClock(15, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
	})
	require.NoError(t, err)

	access, err := testpkg.TenantQuery0(t, calendarContext(t, account.ID), db, service.StaffCalendarAccess)
	require.NoError(t, err)
	require.NotNil(t, access.CalDAV)
	assert.Equal(t, "https://api.moto.test/api/caldav/", access.CalDAV.ServerURL)
	assert.Equal(t, account.Email, access.CalDAV.Username)
	assert.NotEmpty(t, access.CalDAV.AppPassword)
	assert.Equal(t, strings.TrimPrefix(access.URL, "https://moto.test/api/calendar-feed/"), access.CalDAV.AppPassword)

	secondAccess, err := testpkg.TenantQuery0(t, calendarContext(t, account.ID), db, service.StaffCalendarAccess)
	require.NoError(t, err)
	require.NotNil(t, secondAccess.CalDAV)
	assert.Empty(t, secondAccess.URL)
	assert.Empty(t, secondAccess.CalDAV.AppPassword, "the raw capability must only be returned once")

	_, err = service.AuthenticateStaffCalDAV(testpkg.Ctx(t), "other@example.test", access.CalDAV.AppPassword)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)

	queryCounter := testpkg.CaptureQueriesForContext(t, db)
	calendar, err := service.AuthenticateStaffCalDAV(queryCounter.Context(testpkg.Ctx(t)), strings.ToUpper(account.Email), access.CalDAV.AppPassword)
	require.NoError(t, err)
	testpkg.AssertQueryBudget(t, "services.calendar.caldav_snapshot", queryCounter.Queries())
	require.NotEmpty(t, calendar.Items)
	assert.Equal(t, testpkg.Tenant(t), calendar.TenantID)

	_, feedContent, err := service.StaffCalendarFeedByToken(testpkg.Ctx(t), access.CalDAV.AppPassword)
	require.NoError(t, err)
	for _, item := range calendar.Items {
		content := string(item.Content)
		assert.Contains(t, content, "BEGIN:VCALENDAR")
		assert.Contains(t, content, "BEGIN:VEVENT")
		assert.Contains(t, content, "UID:"+item.UID)
		assert.NotContains(t, content, "METHOD:PUBLISH")
		assert.Contains(t, feedContent, "UID:"+item.UID)
	}

	disabledConfig := cfg
	disabledConfig.CalDAVPolicy = fixedCalDAVSettings{enabled: false}
	disabledService := owners.NewCalendarPortal(disabledConfig)
	_, err = disabledService.AuthenticateStaffCalDAV(testpkg.Ctx(t), account.Email, access.CalDAV.AppPassword)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)
	_, _, err = disabledService.StaffCalendarFeedByToken(testpkg.Ctx(t), access.CalDAV.AppPassword)
	require.NoError(t, err, "the CalDAV feature gate must not disable the existing iCalendar feed")

	rotated, err := testpkg.TenantQuery0(t, calendarContext(t, account.ID), db, service.RotateStaffCalendarAccess)
	require.NoError(t, err)
	require.NotNil(t, rotated.CalDAV)
	assert.NotEqual(t, access.CalDAV.AppPassword, rotated.CalDAV.AppPassword)
	_, err = service.AuthenticateStaffCalDAV(testpkg.Ctx(t), account.Email, access.CalDAV.AppPassword)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound, "rotation must revoke the old CalDAV capability")
	_, err = service.AuthenticateStaffCalDAV(testpkg.Ctx(t), account.Email, rotated.CalDAV.AppPassword)
	require.NoError(t, err)

	deactivateAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	_, err = service.AuthenticateStaffCalDAV(testpkg.Ctx(t), account.Email, rotated.CalDAV.AppPassword)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound, "an inactive school mapping must receive no calendar data")
}

func TestCalendarServiceIntegration_StaffCalDAVSettingFailureLeavesICalendarAvailable(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos, reposPeople, reposCalendar, _, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, repos.StaffCalendarFeedToken, reposPeople.Person)
	cfg.StaffFeedTombstoneRepo = reposCalendar.CalendarStaffFeedTombstone
	cfg.CalDAVPolicy = fixedCalDAVSettings{err: errors.New("setting unavailable")}
	cfg.FrontendURL = "https://moto.test"
	service := owners.NewCalendarPortal(cfg)

	_, account := testpkg.CreateTestCalendarStaff(t, db, "CalDAV", "Fail Closed")
	access, err := testpkg.TenantQuery0(t, calendarContext(t, account.ID), db, service.StaffCalendarAccess)
	require.NoError(t, err)
	assert.Nil(t, access.CalDAV)
	require.NotEmpty(t, access.URL)

	token := strings.TrimPrefix(access.URL, "https://moto.test/api/calendar-feed/")
	_, _, err = service.StaffCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
}

func TestCalendarServiceIntegration_StaffFeedPreservesIdentityDatabaseErrors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos, reposPeople, _, _, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, repos.StaffCalendarFeedToken, reposPeople.Person)
	cfg.FrontendURL = "https://moto.test"
	service := owners.NewCalendarPortal(cfg)

	_, account := testpkg.CreateTestCalendarStaff(t, db, "Feed", "Fehler")
	ctx, cancel := context.WithCancel(calendarContext(t, account.ID))
	cancel()

	_, _, err := testpkg.TenantQueryPair0(t, ctx, db, service.StaffCalendarFeedURL)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, calendarSvc.ErrForbidden)
}

func TestCalendarServiceIntegration_StaffSubscriptionLifecycleKeepsParentFeedIndependent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos, reposPeople, reposCalendar, _, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, repos.StaffCalendarFeedToken, reposPeople.Person)
	cfg.StaffFeedTombstoneRepo = reposCalendar.CalendarStaffFeedTombstone
	cfg.ParentsURL = "https://parents.test"
	cfg.FrontendURL = "https://moto.test"
	service := owners.NewCalendarPortal(cfg)

	_, account := testpkg.CreateTestCalendarStaff(t, db, "Dual", "Role")
	staffURL, _, err := testpkg.TenantQueryPair0(t, calendarContext(t, account.ID), db, service.StaffCalendarFeedURL)
	require.NoError(t, err)
	staffToken := strings.TrimPrefix(staffURL, "https://moto.test/api/calendar-feed/")

	parentURL, _, err := service.ParentCalendarFeedURL(testpkg.Ctx(t), account.ID)
	require.NoError(t, err)
	parentToken := strings.TrimPrefix(parentURL, "https://parents.test/api/calendar-feed/")
	_, _, err = service.ParentCalendarFeedByToken(testpkg.Ctx(t), parentToken)
	require.NoError(t, err)

	rotatedURL, _, err := testpkg.TenantQueryPair0(t, calendarContext(t, account.ID), db, service.RotateStaffCalendarFeed)
	require.NoError(t, err)
	rotatedToken := strings.TrimPrefix(rotatedURL, "https://moto.test/api/calendar-feed/")
	assert.NotEqual(t, staffToken, rotatedToken)
	_, _, err = service.StaffCalendarFeedByToken(testpkg.Ctx(t), staffToken)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)
	_, _, err = service.StaffCalendarFeedByToken(testpkg.Ctx(t), rotatedToken)
	require.NoError(t, err)
	_, _, err = service.ParentCalendarFeedByToken(testpkg.Ctx(t), parentToken)
	require.NoError(t, err, "staff rotation must not invalidate the independent parent feed")

	deactivateAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	_, _, err = service.StaffCalendarFeedByToken(testpkg.Ctx(t), rotatedToken)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)
	_, err = db.NewRaw(`UPDATE auth.account_tenants SET status = 'active', activated_at = NOW(),
		deactivated_at = NULL, updated_at = NOW() WHERE account_id = ? AND tenant_id = ?`, account.ID, testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	_, _, err = service.StaffCalendarFeedByToken(testpkg.Ctx(t), rotatedToken)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound, "reactivation must not resurrect a capability issued before offboarding")
}

func TestCalendarServiceIntegration_StaffSubscriptionMatchesPersonalCalendarWithoutPrivateDescriptions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos, reposPeople, reposCalendar, _, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, repos.StaffCalendarFeedToken, reposPeople.Person)
	cfg.StaffFeedTombstoneRepo = reposCalendar.CalendarStaffFeedTombstone
	cfg.FrontendURL = "https://moto.test"
	service := owners.NewCalendarPortal(cfg)

	staff, account := testpkg.CreateTestCalendarStaff(t, db, "Feed", "Quellen")
	day := calendarSvc.Date(calendar.NewDate(2026, time.January, 1)).AddDays(7)
	privateAppointmentDescription := "Besprechung zu Kind Mia Muster"
	appointmentLocation := "Teamraum"
	_, err := testpkg.TenantQuery1(t, calendarContext(t, account.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Teamsitzung",
		Description:  &privateAppointmentDescription,
		Location:     &appointmentLocation,
		StartDate:    day,
		EndDate:      day,
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
	})
	require.NoError(t, err)

	room := testpkg.CreateTestRoom(t, db, "Werkraum")
	instance := testpkg.CreateTestActivityInstance(t, db, day, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "11:00",
		EndHHMM:   "12:00",
		Title:     "Kreativangebot",
	})
	testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
	testpkg.CreateTestStaffShift(t, db, staff.ID, day, testpkg.StaffShiftOpts{
		StartHHMM: "13:00",
		EndHHMM:   "16:00",
		Notes:     "Abholung Mia Muster beachten",
	})

	staffURL, _, err := testpkg.TenantQueryPair0(t, calendarContext(t, account.ID), db, service.StaffCalendarFeedURL)
	require.NoError(t, err)
	token := strings.TrimPrefix(staffURL, "https://moto.test/api/calendar-feed/")
	_, content, err := service.StaffCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)

	assert.Contains(t, content, "SUMMARY:Teamsitzung")
	assert.Contains(t, content, "LOCATION:Teamraum")
	assert.Contains(t, content, "SUMMARY:Kreativangebot")
	assert.Contains(t, content, "LOCATION:Werkraum")
	assert.Contains(t, content, "SUMMARY:Dienst")
	assert.NotContains(t, content, "Mia Muster")
	assert.NotContains(t, content, "DESCRIPTION:")
}

func TestCalendarServiceIntegration_StaffSubscriptionRetainsRemovedScheduleEvents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos, reposPeople, reposCalendar, reposTimetable, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, repos.StaffCalendarFeedToken, reposPeople.Person)
	cfg.StaffFeedTombstoneRepo = reposCalendar.CalendarStaffFeedTombstone
	cfg.FrontendURL = "https://moto.test"
	service := owners.NewCalendarPortal(cfg)

	staff, account := testpkg.CreateTestCalendarStaff(t, db, "Feed", "Entfernt")
	otherStaff, _ := testpkg.CreateTestCalendarStaff(t, db, "Andere", "Person")
	day := calendarSvc.Date(calendar.NewDate(2026, time.January, 1)).AddDays(7)
	room := testpkg.CreateTestRoom(t, db, "Tombstone-Raum")
	instance := testpkg.CreateTestActivityInstance(t, db, day, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "10:00",
		EndHHMM:   "11:00",
		Title:     "Alte Zuordnung",
	})
	assignment := testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
	shift := testpkg.CreateTestStaffShift(t, db, staff.ID, day, testpkg.StaffShiftOpts{
		StartHHMM: "12:00",
		EndHHMM:   "14:00",
	})

	staffURL, _, err := testpkg.TenantQueryPair0(t, calendarContext(t, account.ID), db, service.StaffCalendarFeedURL)
	require.NoError(t, err)
	token := strings.TrimPrefix(staffURL, "https://moto.test/api/calendar-feed/")
	_, before, err := service.StaffCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.Contains(t, before, fmt.Sprintf("UID:timetable-%d-%d@moto-app.de", instance.ID, testpkg.Tenant(t)))
	assert.Contains(t, before, fmt.Sprintf("UID:shift-%d-%d@moto-app.de", shift.ID, testpkg.Tenant(t)))

	require.NoError(t, api.ReassignInstanceStaffForTest(testpkg.Ctx(t), reposTimetable.InstanceStaff, assignment.ID, otherStaff.ID))
	require.NoError(t, reposTimetable.StaffShift.Delete(testpkg.Ctx(t), shift.ID))

	_, after, err := service.StaffCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.Contains(t, after, fmt.Sprintf("UID:timetable-%d-%d@moto-app.de", instance.ID, testpkg.Tenant(t)))
	assert.Contains(t, after, fmt.Sprintf("UID:shift-%d-%d@moto-app.de", shift.ID, testpkg.Tenant(t)))
	assert.GreaterOrEqual(t, strings.Count(after, "STATUS:CANCELLED"), 2)
}

func TestCalendarServiceIntegration_StaffSubscriptionRetainsCancelledScheduleEventsOutsideLookback(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos, reposPeople, reposCalendar, reposTimetable, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, repos.StaffCalendarFeedToken, reposPeople.Person)
	cfg.StaffFeedTombstoneRepo = reposCalendar.CalendarStaffFeedTombstone
	cfg.FrontendURL = "https://moto.test"
	service := owners.NewCalendarPortal(cfg)

	staff, account := testpkg.CreateTestCalendarStaff(t, db, "Feed", "Alte Absage")
	day := calendarSvc.Date(calendar.NewDate(2026, time.January, 1)).AddDays(-60)
	room := testpkg.CreateTestRoom(t, db, "Alter Tombstone-Raum")
	instance := testpkg.CreateTestActivityInstance(t, db, day, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "10:00",
		EndHHMM:   "11:00",
		Title:     "Altes Kreativangebot",
	})
	testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
	shift := testpkg.CreateTestStaffShift(t, db, staff.ID, day, testpkg.StaffShiftOpts{
		StartHHMM: "12:00",
		EndHHMM:   "14:00",
	})

	staffURL, _, err := testpkg.TenantQueryPair0(t, calendarContext(t, account.ID), db, service.StaffCalendarFeedURL)
	require.NoError(t, err)
	token := strings.TrimPrefix(staffURL, "https://moto.test/api/calendar-feed/")
	_, before, err := service.StaffCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.NotContains(t, before, fmt.Sprintf("UID:timetable-%d-%d@moto-app.de", instance.ID, testpkg.Tenant(t)))
	assert.NotContains(t, before, fmt.Sprintf("UID:shift-%d-%d@moto-app.de", shift.ID, testpkg.Tenant(t)))

	_, err = db.NewUpdate().
		Table("schedule.activity_instances").
		Set("status = ?", testpkg.ScheduleInstanceStatusCancelled).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("id = ?", instance.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	storedShift, err := reposTimetable.StaffShift.FindByID(testpkg.Ctx(t), shift.ID)
	require.NoError(t, err)
	storedShift.Cancelled = true
	require.NoError(t, reposTimetable.StaffShift.Update(testpkg.Ctx(t), storedShift))

	_, after, err := service.StaffCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.Contains(t, after, fmt.Sprintf("UID:timetable-%d-%d@moto-app.de", instance.ID, testpkg.Tenant(t)))
	assert.Contains(t, after, fmt.Sprintf("UID:shift-%d-%d@moto-app.de", shift.ID, testpkg.Tenant(t)))
	assert.GreaterOrEqual(t, strings.Count(after, "STATUS:CANCELLED"), 2)
}

func TestCalendarServiceIntegration_StaffSubscriptionPublishesOccurrenceAndDeletionCancellations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos, reposPeople, reposCalendar, _, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, repos.StaffCalendarFeedToken, reposPeople.Person)
	cfg.StaffFeedTombstoneRepo = reposCalendar.CalendarStaffFeedTombstone
	cfg.FrontendURL = "https://moto.test"
	service := owners.NewCalendarPortal(cfg)

	_, account := testpkg.CreateTestCalendarStaff(t, db, "Feed", "Absage")
	start := portalDate(2026, 8, 24).AddDays(7)
	endsOn := start.AddDays(28)
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, account.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Wöchentliche Runde",
		StartDate:    start,
		EndDate:      start,
		StartTime:    wallClock(14, 0),
		EndTime:      wallClock(15, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{strings.ToLower(start.Weekday().String())},
			EndsOn:        &endsOn,
		},
	})
	require.NoError(t, err)

	staffURL, _, err := testpkg.TenantQueryPair0(t, calendarContext(t, account.ID), db, service.StaffCalendarFeedURL)
	require.NoError(t, err)
	token := strings.TrimPrefix(staffURL, "https://moto.test/api/calendar-feed/")

	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, account.ID), db, service.CancelStaffAppointmentOccurrence, detail.Appointment.ID, start.AddDays(7)))
	_, withExdate, err := service.StaffCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.Contains(t, withExdate, "EXDATE;TZID=Europe/Berlin:")

	require.NoError(t, testpkg.TenantCommand1(t, calendarContext(t, account.ID), db, service.DeleteStaffAppointment, detail.Appointment.ID))
	_, afterDelete, err := service.StaffCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.Contains(t, afterDelete, "SUMMARY:Wöchentliche Runde")
	assert.Contains(t, afterDelete, "STATUS:CANCELLED")
	assert.Contains(t, afterDelete, "SEQUENCE:")
}

func TestCalendarServiceIntegration_CleanupExpiredFeedTombstonesDeletesStaffTombstones(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupIsolatedTestDB(t)
	_, _, reposCalendar, reposTimetable, _, _, _ := calendarTestRepositories(t, db)
	cfg := calendarTestConfig(t, db)
	cfg.StaffFeedTombstoneRepo = reposCalendar.CalendarStaffFeedTombstone
	service := owners.NewCalendarPortal(cfg)

	staff, account := testpkg.CreateTestCalendarStaff(t, db, "Cleanup", "Feed")
	day := portalDate(2026, 8, 24).AddDays(7)
	endsOn := day.AddDays(14)
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, account.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Alter Feed-Termin",
		StartDate:    day,
		EndDate:      day,
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{strings.ToLower(day.Weekday().String())},
			EndsOn:        &endsOn,
		},
	})
	require.NoError(t, err)
	require.NoError(t, testpkg.TenantCommand1(t, calendarContext(t, account.ID), db, service.DeleteStaffAppointment, detail.Appointment.ID))
	cancelledDetail, err := testpkg.TenantQuery1(t, calendarContext(t, account.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Alter abgesagter Feed-Termin",
		StartDate:    day.AddDays(1),
		EndDate:      day.AddDays(1),
		StartTime:    wallClock(11, 0),
		EndTime:      wallClock(12, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{strings.ToLower(day.AddDays(1).Weekday().String())},
			EndsOn:        &endsOn,
		},
	})
	require.NoError(t, err)
	_, err = testpkg.TenantQuery1(t, calendarContext(t, account.ID), db, service.CancelStaffAppointment, cancelledDetail.Appointment.ID)
	require.NoError(t, err)

	room := testpkg.CreateTestRoom(t, db, "Cleanup-Raum")
	instance := testpkg.CreateTestActivityInstance(t, db, day, room.ID, testpkg.ActivityInstanceOpts{})
	assignment := testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
	require.NoError(t, reposTimetable.InstanceStaff.DeleteInstanceStaff(testpkg.Ctx(t), assignment.ID))

	expired := time.Now().AddDate(0, 0, -91)
	_, err = db.NewUpdate().Table("calendar.appointments").
		Set("deleted_at = ?", expired).
		Where("id = ?", detail.Appointment.ID).
		Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewUpdate().Table("calendar.appointments").
		Set("cancelled_at = ?", expired).
		Where("id = ?", cancelledDetail.Appointment.ID).
		Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewUpdate().Table("calendar.staff_feed_tombstones").
		Set("cancelled_at = ?", expired).
		Where("source = ?", calendarSvc.EventSourceTimetable).
		Where("source_id = ?", instance.ID).
		Exec(context.Background())
	require.NoError(t, err)

	deleted, err := testpkg.TenantQuery0(t, testpkg.Ctx(t), db, service.CleanupExpiredFeedTombstones)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted, "the feed cleanup deletes the staff feed tombstone only")

	var staffTombstones int
	require.NoError(t, db.NewSelect().Table("calendar.staff_feed_tombstones").ColumnExpr("COUNT(*)").
		Where("source_id = ?", instance.ID).Scan(context.Background(), &staffTombstones))
	assert.Zero(t, staffTombstones)
	// Cancelled and deleted appointments go in the nightly retention of the
	// Appointments owner, which logs them (#4045,
	// TestNightlyRetentionDeletesCancelledAppointmentsAfterTheFeedWindow).
	for _, id := range []int64{detail.Appointment.ID, cancelledDetail.Appointment.ID} {
		var appointments int
		require.NoError(t, db.NewSelect().Table("calendar.appointments").ColumnExpr("COUNT(*)").
			Where("id = ?", id).Scan(context.Background(), &appointments))
		assert.Equal(t, 1, appointments, "the feed cleanup leaves appointment %d to the nightly retention", id)
	}
}

func TestCalendarServiceIntegration_DeleteFeedVisibleLeavesTombstone(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	cfg := calendarTestConfig(t, db)
	repos, _, _, _, _, _, _ := calendarTestRepositories(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, nil, nil)
	cfg.ParentsURL = "https://parents.test"
	// The feed counts its window from today; the appointments below sit on
	// fixed dates, so today is pinned to them instead of the wall clock.
	cfg.Today = func() appointments.Date { return portalDate(2026, 8, 24) }
	service := owners.NewCalendarPortal(cfg)

	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Tombstone", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Elternabend",
		StartDate:    portalDate(2026, 8, 24).AddDays(7),
		EndDate:      portalDate(2026, 8, 24).AddDays(7),
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(19, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	// The appointment is tombstoned, not hard-deleted, so clean it up explicitly.

	httpsURL, _, err := service.ParentCalendarFeedURL(testpkg.Ctx(t), parentChain.AccountID)
	require.NoError(t, err)
	token := strings.TrimPrefix(httpsURL, "https://parents.test/api/calendar-feed/")

	// The interactive calendar window is capped (maxCalendarWindowDays); use a
	// small one that still spans the appointment (today + 7).
	viewFrom := portalDate(2026, 8, 24)
	viewTo := portalDate(2026, 8, 24).AddDays(30)

	// Before deletion the appointment is a normal (non-cancelled) feed event and
	// shows up in the parent's interactive calendar.
	_, content, err := service.ParentCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.Contains(t, content, "SUMMARY:Elternabend")
	assert.NotContains(t, content, "STATUS:CANCELLED")

	before, err := service.ListMyParentEvents(testpkg.Ctx(t), parentChain.AccountID, viewFrom, viewTo)
	require.NoError(t, err)
	assert.NotEmpty(t, eventDates(before, calendarSvc.EventSourceAppointment))

	require.NoError(t, testpkg.TenantCommand1(t, calendarContext(t, organizerAccount.ID), db, service.DeleteStaffAppointment, detail.Appointment.ID))

	// The subscription feed keeps re-exporting the same UID as a durable
	// STATUS:CANCELLED tombstone with a bumped SEQUENCE, so subscribers purge it
	// instead of retaining a stale event.
	_, afterDelete, err := service.ParentCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.Contains(t, afterDelete, "SUMMARY:Elternabend")
	assert.Contains(t, afterDelete, "STATUS:CANCELLED")
	assert.Contains(t, afterDelete, "SEQUENCE:")

	// But the delete is a real delete for every interactive surface: the organizer
	// can no longer open it, and it is gone from the parent's calendar.
	_, err = testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.GetStaffAppointmentDetail, detail.Appointment.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	afterParent, err := service.ListMyParentEvents(testpkg.Ctx(t), parentChain.AccountID, viewFrom, viewTo)
	require.NoError(t, err)
	assert.Empty(t, eventDates(afterParent, calendarSvc.EventSourceAppointment))
}

func TestCalendarServiceIntegration_EditRacingCancellationConflicts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, _, _, _, reposAppointments, _, _ := calendarTestRepositories(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Reactivate", "Organizer")
	invitedStaff, _ := testpkg.CreateTestCalendarStaff(t, db, "Reactivate", "Teacher")

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Original",
		StartDate:    portalDate(2026, 3, 2),
		EndDate:      portalDate(2026, 3, 2),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeRSVPRequired,
		Targets:      []calendarSvc.AppointmentTarget{{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID}},
	})
	require.NoError(t, err)
	// The snapshot returned by create carries CancelledAt == nil — it stands in for
	// a content edit loaded BEFORE the cancellation below.
	require.Nil(t, detail.Appointment.CancelledAt)

	// Cancel it (persists cancelled_at via the dedicated conditional update).
	_, err = testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointment, detail.Appointment.ID)
	require.NoError(t, err)

	// Flushing the stale edit must NOT touch the terminal row: the conditional
	// Update matches zero rows and reports a lifecycle conflict, so the edit
	// neither reactivates the appointment nor overwrites its content.
	stale := *detail.Appointment
	stale.Title = "Edited concurrently"
	_, err = reposAppointments.Appointments().UpdateAppointment(testpkg.Ctx(t), appointments.UpdateAppointment{
		ID: stale.ID,
		AppointmentFields: appointments.AppointmentFields{
			OrganizerStaffID: stale.OrganizerStaffID, Title: stale.Title, Description: stale.Description,
			Location: stale.Location, StartDate: stale.StartDate, EndDate: stale.EndDate,
			StartTime: stale.StartTime, EndTime: stale.EndTime, AllDay: stale.AllDay,
			DeliveryMode: stale.DeliveryMode, OverviewVisibility: stale.OverviewVisibility,
			NotifyGuardians: stale.NotifyGuardians,
		},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, appointments.ErrAppointmentLifecycleConflict)

	after, err := reposAppointments.Appointments().FindAppointment(testpkg.Ctx(t), detail.Appointment.ID)
	require.NoError(t, err)
	require.NotNil(t, after)
	assert.NotNil(t, after.CancelledAt, "the cancellation stands")
	assert.Equal(t, "Original", after.Title, "the racing edit did not apply")
}

func TestCalendarServiceIntegration_CancelledTombstoneSurvivesLookbackWindow(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	cfg := calendarTestConfig(t, db)
	repos, _, _, _, _, _, _ := calendarTestRepositories(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, nil, nil)
	cfg.ParentsURL = "https://parents.test"
	service := owners.NewCalendarPortal(cfg)

	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "OldCancel", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	// A guardian-facing appointment whose date is already far beyond the feed's
	// 30-day lookback, so the live feed query never returns it.
	pastDate := calendarSvc.Date(calendar.NewDate(2026, time.January, 1)).AddDays(-60)
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Vergangener Termin",
		StartDate:    pastDate,
		EndDate:      pastDate,
		StartTime:    wallClock(15, 0),
		EndTime:      wallClock(16, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets:      []calendarSvc.AppointmentTarget{{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID}},
	})
	require.NoError(t, err)

	httpsURL, _, err := service.ParentCalendarFeedURL(testpkg.Ctx(t), parentChain.AccountID)
	require.NoError(t, err)
	token := strings.TrimPrefix(httpsURL, "https://parents.test/api/calendar-feed/")

	// Before cancellation: outside the lookback, so absent from the feed.
	_, content, err := service.ParentCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.NotContains(t, content, "SUMMARY:Vergangener Termin")

	// Cancel it. Even though its date aged out of the lookback, the feed must
	// re-export it as a durable STATUS:CANCELLED tombstone (retained by cancel
	// time), so a subscriber who was offline still receives the cancellation.
	_, err = testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointment, detail.Appointment.ID)
	require.NoError(t, err)

	_, afterCancel, err := service.ParentCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.Contains(t, afterCancel, "SUMMARY:Vergangener Termin")
	assert.Contains(t, afterCancel, "STATUS:CANCELLED")
}

func TestCalendarServiceIntegration_AllSchoolParentsTarget(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "School", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Schulfest",
		StartDate:    portalDate(2026, 6, 20),
		EndDate:      portalDate(2026, 6, 20),
		StartTime:    wallClock(10, 0),
		EndTime:      wallClock(14, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeAllSchoolParents},
		},
	})
	require.NoError(t, err)

	// The whole-school target resolved to the seeded guardian.
	require.Len(t, detail.Targets, 1)
	assert.Equal(t, appointments.TargetTypeAllSchoolParents, detail.Targets[0].TargetType)
	id := findOptionalRecipientByGuardian(detail, parentChain.GuardianProfileID)
	assert.NotZero(t, id, "all_school_parents should reach the school's guardian")

	// The parent sees it in their calendar.
	events, err := service.ListMyParentEvents(testpkg.Ctx(t), parentChain.AccountID, portalDate(2026, 6, 20), portalDate(2026, 6, 20))
	require.NoError(t, err)
	require.NotEmpty(t, events)
	assert.Equal(t, "Schulfest", events[0].Title)
}

func TestCalendarServiceIntegration_AllSchoolParentsExcludesInactiveStudents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "SchoolActive", "Organizer")
	activeChain := testpkg.CreateTestParentGuardianChain(t, db)
	formerChain := testpkg.CreateTestParentGuardianChain(t, db)

	// Mark the second family's child as a former (inactive) student.
	_, err := db.NewUpdate().
		Table("users.student_school_memberships").
		Set("status = ?", testpkg.StudentStatusInactive).
		Where("student_profile_id = ? AND deleted_at IS NULL", formerChain.StudentID).
		Exec(context.Background())
	require.NoError(t, err)

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Schulfest",
		StartDate:    portalDate(2026, 6, 20),
		EndDate:      portalDate(2026, 6, 20),
		StartTime:    wallClock(10, 0),
		EndTime:      wallClock(14, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeAllSchoolParents},
		},
	})
	require.NoError(t, err)

	// The active family's guardian is reached; the former family's is not.
	assert.NotZero(t, findOptionalRecipientByGuardian(detail, activeChain.GuardianProfileID),
		"an active student's guardian must receive the whole-school appointment")
	assert.Zero(t, findOptionalRecipientByGuardian(detail, formerChain.GuardianProfileID),
		"a former (inactive) student's guardian must NOT receive it")
}

func TestCalendarServiceIntegration_CancelSingleOccurrence(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Occurrence", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Occurrence", "Invitee")

	endsOn := portalDate(2026, 1, 19)
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Weekly standup",
		StartDate:    portalDate(2026, 1, 5), // Monday
		EndDate:      portalDate(2026, 1, 5),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(9, 30),
		DeliveryMode: appointments.DeliveryModeRSVPRequired,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)

	events, err := testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 1, 5), portalDate(2026, 1, 19))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-12", "2026-01-19"}, eventDates(events, calendarSvc.EventSourceAppointment))

	// Cancel the middle occurrence.
	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointmentOccurrence, detail.Appointment.ID, portalDate(2026, 1, 12)))
	events, err = testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 1, 5), portalDate(2026, 1, 19))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-19"}, eventDates(events, calendarSvc.EventSourceAppointment))

	// Cancelling the same occurrence again is idempotent.
	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointmentOccurrence, detail.Appointment.ID, portalDate(2026, 1, 12)))
}

func TestCalendarServiceIntegration_AttendeeOverviewVisibility(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Overview", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Overview", "Teacher")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:              "Overview visible to all",
		StartDate:          portalDate(2026, 2, 10),
		EndDate:            portalDate(2026, 2, 10),
		StartTime:          wallClock(9, 0),
		EndTime:            wallClock(10, 0),
		DeliveryMode:       appointments.DeliveryModeRSVPRequired,
		OverviewVisibility: appointments.OverviewVisibilityAll,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	staffRecipientID := findRecipientByStaff(t, detail, invitedStaff.ID)
	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, invitedAccount.ID), db, service.RespondToStaffInvitation, staffRecipientID, appointments.ResponseStatusAccepted))

	staffOverview, err := testpkg.TenantQuery1(t, calendarContext(t, invitedAccount.ID), db, service.GetStaffAppointmentOverview, detail.Appointment.ID)
	require.NoError(t, err)
	require.NotNil(t, staffOverview)
	assert.Equal(t, appointments.OverviewVisibilityAll, staffOverview.OverviewVisibility)
	assert.Equal(t, appointments.DeliveryModeRSVPRequired, staffOverview.DeliveryMode)
	require.Len(t, staffOverview.Attendees, 2)
	assert.Equal(t, appointments.RecipientTypeGuardianProfile, staffOverview.Attendees[0].RecipientType)
	assert.Contains(t, staffOverview.Attendees[0].Name, "Schneider")
	assert.Equal(t, appointments.ResponseStatusPending, staffOverview.Attendees[0].Status)
	assert.Equal(t, appointments.RecipientTypeStaff, staffOverview.Attendees[1].RecipientType)
	assert.Contains(t, staffOverview.Attendees[1].Name, "Teacher")
	assert.Equal(t, appointments.ResponseStatusAccepted, staffOverview.Attendees[1].Status)
	require.NotNil(t, staffOverview.Attendees[1].RespondedAt)

	parentOverview, err := service.GetParentAppointmentOverview(testpkg.Ctx(t), parentChain.AccountID, detail.Appointment.ID)
	require.NoError(t, err)
	require.Len(t, parentOverview.Attendees, 2)

	staffEvents, err := testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 2, 10), portalDate(2026, 2, 10))
	require.NoError(t, err)
	require.NotEmpty(t, staffEvents)
	assert.True(t, staffEvents[0].CanViewOverview)

	parentEvents, err := service.ListMyParentEvents(testpkg.Ctx(t), parentChain.AccountID, portalDate(2026, 2, 10), portalDate(2026, 2, 10))
	require.NoError(t, err)
	require.NotEmpty(t, parentEvents)
	assert.True(t, parentEvents[0].CanViewOverview)
}

func TestCalendarServiceIntegration_DeletedAppointmentUnreachableViaOverviewAndRSVP(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Deleted", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Deleted", "Teacher")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:              "Wird gelöscht",
		StartDate:          portalDate(2026, 2, 10),
		EndDate:            portalDate(2026, 2, 10),
		StartTime:          wallClock(9, 0),
		EndTime:            wallClock(10, 0),
		DeliveryMode:       appointments.DeliveryModeRSVPRequired,
		OverviewVisibility: appointments.OverviewVisibilityAll,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	// Soft-deleted, so it must be cleaned up explicitly.

	staffRecipientID := findRecipientByStaff(t, detail, invitedStaff.ID)
	guardianRecipientID := findRecipientByGuardian(t, detail, parentChain.GuardianProfileID)

	// Sanity: while live, overview and RSVP are reachable.
	_, err = testpkg.TenantQuery1(t, calendarContext(t, invitedAccount.ID), db, service.GetStaffAppointmentOverview, detail.Appointment.ID)
	require.NoError(t, err)
	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, invitedAccount.ID), db, service.RespondToStaffInvitation, staffRecipientID, appointments.ResponseStatusAccepted))

	// It targets a guardian, so delete tombstones (soft-deletes) it.
	require.NoError(t, testpkg.TenantCommand1(t, calendarContext(t, organizerAccount.ID), db, service.DeleteStaffAppointment, detail.Appointment.ID))

	// Every interactive detail/overview/RSVP path — staff and parent — must now
	// treat the retained appointment/recipient IDs as not found.
	_, err = testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.GetStaffAppointmentDetail, detail.Appointment.ID)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)

	_, err = testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.GetStaffAppointmentOverview, detail.Appointment.ID)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)
	_, err = testpkg.TenantQuery1(t, calendarContext(t, invitedAccount.ID), db, service.GetStaffAppointmentOverview, detail.Appointment.ID)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)
	_, err = service.GetParentAppointmentOverview(testpkg.Ctx(t), parentChain.AccountID, detail.Appointment.ID)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)

	err = testpkg.TenantCommand2(t, calendarContext(t, invitedAccount.ID), db, service.RespondToStaffInvitation, staffRecipientID, appointments.ResponseStatusDeclined)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)
	err = service.RespondToParentInvitation(testpkg.Ctx(t), parentChain.AccountID, guardianRecipientID, appointments.ResponseStatusDeclined)
	assert.ErrorIs(t, err, calendarSvc.ErrNotFound)
}

func TestCalendarServiceIntegration_AttendeeOverviewAccessRules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Private", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Private", "Invitee")
	_, outsiderAccount := testpkg.CreateTestCalendarStaff(t, db, "Private", "Outsider")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:              "Staff-only overview",
		StartDate:          portalDate(2026, 2, 11),
		EndDate:            portalDate(2026, 2, 11),
		StartTime:          wallClock(11, 0),
		EndTime:            wallClock(12, 0),
		DeliveryMode:       appointments.DeliveryModeInformational,
		OverviewVisibility: appointments.OverviewVisibilityStaff,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	organizerOverview, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.GetStaffAppointmentOverview, detail.Appointment.ID)
	require.NoError(t, err)
	require.Len(t, organizerOverview.Attendees, 2)
	for _, attendee := range organizerOverview.Attendees {
		assert.Equal(t, appointments.ResponseStatusInfo, attendee.Status)
	}

	_, err = testpkg.TenantQuery1(t, calendarContext(t, invitedAccount.ID), db, service.GetStaffAppointmentOverview, detail.Appointment.ID)
	require.NoError(t, err)

	_, err = service.GetParentAppointmentOverview(testpkg.Ctx(t), parentChain.AccountID, detail.Appointment.ID)
	assert.True(t, errors.Is(err, calendarSvc.ErrForbidden))

	_, err = testpkg.TenantQuery1(t, calendarContext(t, outsiderAccount.ID), db, service.GetStaffAppointmentOverview, detail.Appointment.ID)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	parentEvents, err := service.ListMyParentEvents(testpkg.Ctx(t), parentChain.AccountID, portalDate(2026, 2, 11), portalDate(2026, 2, 11))
	require.NoError(t, err)
	require.NotEmpty(t, parentEvents)
	assert.False(t, parentEvents[0].CanViewOverview)
}

func TestCalendarServiceIntegration_RecipientOptionsAndGroupedTargets(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Target", "Organizer")
	invitedStaff, _ := testpkg.CreateTestCalendarStaff(t, db, "Target", "Invitee")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	inactiveParentChain := testpkg.CreateTestParentGuardianChain(t, db)
	_, err := db.ExecContext(
		context.Background(),
		`UPDATE auth.account_tenants SET status = ? WHERE account_id = ? AND tenant_id = ?`,
		"inactive",
		inactiveParentChain.AccountID,
		inactiveParentChain.TenantID,
	)
	require.NoError(t, err)
	invisibleGuardian := &testpkg.GuardianProfile{
		TenantID:               testpkg.Tenant(t),
		FirstName:              "Hidden",
		LastName:               "Target",
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	_, err = db.NewInsert().Model(invisibleGuardian).ModelTableExpr(`users.guardian_profiles`).Exec(context.Background())
	require.NoError(t, err)
	accountlessGuardian := &testpkg.GuardianProfile{
		TenantID:               testpkg.Tenant(t),
		FirstName:              "Accountless",
		LastName:               "Target",
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	_, err = db.NewInsert().Model(accountlessGuardian).ModelTableExpr(`users.guardian_profiles`).Exec(context.Background())
	require.NoError(t, err)
	accountlessLink := &testpkg.StudentGuardian{
		StudentID:         parentChain.StudentID,
		GuardianProfileID: accountlessGuardian.ID,
		RelationshipType:  "parent",
	}
	testpkg.ApplyStudentGuardianRole(accountlessLink, testpkg.GuardianRoleLegalGuardian)
	accountlessLink.TenantID = testpkg.Tenant(t)
	require.NoError(t, testpkg.InsertTestStudentGuardian(context.Background(), db, accountlessLink))

	options, err := testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.RecipientOptions, "target", 20)
	require.NoError(t, err)
	assert.NotEmpty(t, options.Staff)
	assert.NotNil(t, options.Parents)
	assert.NotNil(t, options.Groups)
	assert.NotNil(t, options.Classes)
	assert.NotNil(t, options.Students)
	assert.NotContains(t, parentOptionIDs(options.Parents), strconv.FormatInt(invisibleGuardian.ID, 10))
	assert.NotContains(t, parentOptionIDs(options.Parents), strconv.FormatInt(inactiveParentChain.GuardianProfileID, 10))
	assert.NotContains(t, parentOptionIDs(options.Parents), strconv.FormatInt(accountlessGuardian.ID, 10))

	schneiderOptions, err := testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.RecipientOptions, "schneider", 20)
	require.NoError(t, err)
	assert.Contains(t, parentOptionIDs(schneiderOptions.Parents), strconv.FormatInt(parentChain.GuardianProfileID, 10))
	assert.NotContains(t, parentOptionIDs(schneiderOptions.Parents), strconv.FormatInt(inactiveParentChain.GuardianProfileID, 10))

	hiddenOptions, err := testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.RecipientOptions, "hidden", 20)
	require.NoError(t, err)
	assert.NotContains(t, parentOptionIDs(hiddenOptions.Parents), strconv.FormatInt(invisibleGuardian.ID, 10))

	childOptions, err := testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.RecipientOptions, "felix", 20)
	require.NoError(t, err)
	require.NotEmpty(t, childOptions.Students)
	assert.Contains(t, childOptions.Students[0].Name, "Felix")

	limitedOptions, err := testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.RecipientOptions, "", 1)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(limitedOptions.Staff), 1)
	assert.LessOrEqual(t, len(limitedOptions.Groups), 1)
	assert.LessOrEqual(t, len(limitedOptions.Classes), 1)
	assert.LessOrEqual(t, len(limitedOptions.Students), 1)

	defaultedOptions, err := testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.RecipientOptions, "target", 999)
	require.NoError(t, err)
	assert.NotNil(t, defaultedOptions.Staff)

	className := "1a"
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:              "Grouped target coverage",
		StartDate:          portalDate(2026, 2, 12),
		EndDate:            portalDate(2026, 2, 12),
		StartTime:          wallClock(13, 0),
		EndTime:            wallClock(14, 0),
		DeliveryMode:       appointments.DeliveryModeInformational,
		OverviewVisibility: appointments.OverviewVisibilityAll,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeAllStaff},
			{Type: appointments.TargetTypeParentsByClass, Value: &className},
			{Type: appointments.TargetTypeParentsByStudent, ID: &parentChain.StudentID},
		},
	})
	require.NoError(t, err)

	require.NotEmpty(t, detail.Recipients)
	assert.NotZero(t, findRecipientByStaff(t, detail, organizer.ID))
	assert.NotZero(t, findRecipientByStaff(t, detail, invitedStaff.ID))
	assert.NotZero(t, findRecipientByGuardian(t, detail, parentChain.GuardianProfileID))
	assert.Zero(t, findOptionalRecipientByGuardian(detail, inactiveParentChain.GuardianProfileID))
	assert.Zero(t, findOptionalRecipientByGuardian(detail, accountlessGuardian.ID))
	require.Len(t, detail.Targets, 3)

	parentEvents, err := service.ListMyParentEvents(testpkg.Ctx(t), parentChain.AccountID, portalDate(2026, 2, 12), portalDate(2026, 2, 12))
	require.NoError(t, err)
	require.NotEmpty(t, parentEvents)
	assert.True(t, parentEvents[0].CanViewOverview)
	assert.False(t, parentEvents[0].CanRespond)
	require.NotNil(t, parentEvents[0].ResponseStatus)
	assert.Equal(t, appointments.ResponseStatusInfo, *parentEvents[0].ResponseStatus)
}

func parentOptionIDs(options []calendarSvc.ParentOption) []string {
	ids := make([]string, 0, len(options))
	for _, option := range options {
		ids = append(ids, option.ID)
	}
	return ids
}

func TestCalendarServiceIntegration_InvalidCreateTargets(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Invalid", "Targets")
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	otherTenantStaff := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Other", "Tenant")
	invisibleGuardian := &testpkg.GuardianProfile{
		TenantID:               testpkg.Tenant(t),
		FirstName:              "No",
		LastName:               "Portal",
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	_, err := db.NewInsert().Model(invisibleGuardian).ModelTableExpr(`users.guardian_profiles`).Exec(context.Background())
	require.NoError(t, err)

	className := " "
	invalidID := int64(0)
	missingID := int64(999999999)
	tests := []struct {
		name   string
		target calendarSvc.AppointmentTarget
	}{
		{name: "staff target missing id", target: calendarSvc.AppointmentTarget{Type: appointments.TargetTypeStaff}},
		{name: "staff target zero id", target: calendarSvc.AppointmentTarget{Type: appointments.TargetTypeStaff, ID: &invalidID}},
		{name: "staff target other tenant", target: calendarSvc.AppointmentTarget{Type: appointments.TargetTypeStaff, ID: &otherTenantStaff.ID}},
		{name: "guardian target missing id", target: calendarSvc.AppointmentTarget{Type: appointments.TargetTypeGuardianProfile}},
		{name: "guardian target missing row", target: calendarSvc.AppointmentTarget{Type: appointments.TargetTypeGuardianProfile, ID: &missingID}},
		{name: "guardian target without portal-visible student", target: calendarSvc.AppointmentTarget{Type: appointments.TargetTypeGuardianProfile, ID: &invisibleGuardian.ID}},
		{name: "student target missing id", target: calendarSvc.AppointmentTarget{Type: appointments.TargetTypeParentsByStudent}},
		{name: "group target missing id", target: calendarSvc.AppointmentTarget{Type: appointments.TargetTypeParentsByGroup}},
		{name: "class target blank value", target: calendarSvc.AppointmentTarget{Type: appointments.TargetTypeParentsByClass, Value: &className}},
		{name: "unknown target type", target: calendarSvc.AppointmentTarget{Type: "unknown_target"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
				Title:        "Invalid target",
				StartDate:    portalDate(2026, 2, 14),
				EndDate:      portalDate(2026, 2, 14),
				StartTime:    wallClock(8, 0),
				EndTime:      wallClock(9, 0),
				DeliveryMode: appointments.DeliveryModeRSVPRequired,
				Targets:      []calendarSvc.AppointmentTarget{tt.target},
			})
			require.Error(t, err)
			assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
		})
	}
}

func TestCalendarServiceIntegration_InvalidRecurrenceDoesNotPersistAppointment(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Invalid", "Recurrence")
	invitedStaff, _ := testpkg.CreateTestCalendarStaff(t, db, "Invalid", "RecurrenceInvitee")

	title := "Invalid recurrence must not persist"
	_, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:     title,
		StartDate: portalDate(2026, 2, 20),
		EndDate:   portalDate(2026, 2, 20),
		StartTime: wallClock(8, 0),
		EndTime:   wallClock(9, 0),
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency: "hourly",
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))

	count, err := db.NewSelect().
		TableExpr(`calendar.appointments`).
		Where(`title = ?`, title).
		Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestCalendarServiceIntegration_CreateRollsBackAppointmentsWhenRecurrenceWriteFails(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	cfg := calendarTestConfig(t, db)
	recorder := &recordingAppointmentCreate{Capability: cfg.Appointments}
	cfg.Appointments = recorder
	cfg.Appointments = failingRecurrenceCreate{Capability: cfg.Appointments}
	service := owners.NewCalendarPortal(cfg)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Rollback", "Organizer")
	invitedStaff, _ := testpkg.CreateTestCalendarStaff(t, db, "Rollback", "Invitee")
	endsOn := portalDate(2030, 3, 31)
	request := calendarSvc.CreateAppointmentRequest{
		Title:        "Rollback after appointment targets",
		StartDate:    portalDate(2030, 3, 4),
		EndDate:      portalDate(2030, 3, 4),
		StartTime:    wallClock(8, 0),
		EndTime:      wallClock(9, 0),
		DeliveryMode: appointments.DeliveryModeRSVPRequired,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID}},
	}

	_, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, request)
	require.ErrorIs(t, err, errInjectedRecurrenceCreate)
	assert.Positive(t, recorder.appointmentID, "the appointment insert completed before the injected failure")
	assert.Equal(t, 1, recorder.targets, "the target insert completed before the injected failure")

	_, err = cfg.Appointments.FindAppointment(testpkg.Ctx(t), recorder.appointmentID)
	require.ErrorIs(t, err, appointments.ErrAppointmentNotFound)
	rolledBackTargets, err := cfg.Appointments.FindAppointmentTargets(testpkg.Ctx(t), recorder.appointmentID)
	require.NoError(t, err)
	assert.Empty(t, rolledBackTargets)

	retried, err := setupCalendarService(t, db).CreateStaffAppointment(calendarContext(t, organizerAccount.ID), request)
	require.NoError(t, err)
	require.Len(t, retried.Targets, 1, "retry creates one target")
	count, err := db.NewSelect().TableExpr(`calendar.appointments AS "appointment"`).
		Where(`"appointment".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"appointment".title = ?`, request.Title).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, int(count), "retry creates exactly one appointment")
}

func TestCalendarServiceIntegration_UpdateRollsBackRecurrenceReplacementAndRetries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		failure error
		wrap    func(appointments.Capability) appointments.Capability
	}{
		{
			name:    "recurrence create",
			failure: errInjectedRecurrenceCreate,
			wrap: func(capability appointments.Capability) appointments.Capability {
				return failingRecurrenceCreate{Capability: capability}
			},
		},
		{
			name:    "override delete after recurrence create",
			failure: errInjectedOverrideDelete,
			wrap: func(capability appointments.Capability) appointments.Capability {
				return failingOverrideDelete{Capability: capability}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runRecurrenceUpdateRollbackScenario(t, tt.failure, tt.wrap)
		})
	}
}

type recurrenceUpdateRollbackFixture struct {
	config        calendarCompose.Config
	service       calendarSvc.Service
	accountID     int64
	appointmentID int64
	cancelledDate calendarSvc.Date
	update        calendarSvc.UpdateAppointmentRequest
}

func runRecurrenceUpdateRollbackScenario(t *testing.T, failure error, wrap func(appointments.Capability) appointments.Capability) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	fixture := setupRecurrenceUpdateRollbackFixture(t, db)
	failingCfg := calendarTestConfig(t, db)
	failingCfg.Appointments = wrap(failingCfg.Appointments)
	_, err := owners.NewCalendarPortal(failingCfg).UpdateStaffAppointment(calendarContext(t, fixture.accountID), fixture.appointmentID, fixture.update)
	require.ErrorIs(t, err, failure)
	assertRecurrenceUpdateRolledBack(t, fixture)
	assertRecurrenceUpdateRetry(t, fixture)
}

func setupRecurrenceUpdateRollbackFixture(t *testing.T, db *bun.DB) recurrenceUpdateRollbackFixture {
	t.Helper()
	config := calendarTestConfig(t, db)
	service := owners.NewCalendarPortal(config)
	_, account := testpkg.CreateTestCalendarStaff(t, db, "Series", "Organizer")
	invitedStaff, _ := testpkg.CreateTestCalendarStaff(t, db, "Series", "Invitee")
	oldEndsOn := portalDate(2030, 3, 31)
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, account.ID), testpkg.SetupTestDB(t), service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title: "Original series", StartDate: portalDate(2030, 3, 4), EndDate: portalDate(2030, 3, 4),
		StartTime: wallClock(8, 0), EndTime: wallClock(9, 0), DeliveryMode: appointments.DeliveryModeRSVPRequired,
		Recurrence: &calendarSvc.RecurrenceRequest{Frequency: appointments.RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, EndsOn: &oldEndsOn},
		Targets:    []calendarSvc.AppointmentTarget{{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID}},
	})
	require.NoError(t, err)
	cancelledDate := portalDate(2030, 3, 11)
	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, account.ID), testpkg.SetupTestDB(t), service.CancelStaffAppointmentOccurrence, detail.Appointment.ID, cancelledDate))
	newEndsOn := portalDate(2030, 6, 30)
	return recurrenceUpdateRollbackFixture{config: config, service: service, accountID: account.ID, appointmentID: detail.Appointment.ID, cancelledDate: cancelledDate,
		update: calendarSvc.UpdateAppointmentRequest{Title: "Replacement series", StartDate: portalDate(2030, 4, 2), EndDate: portalDate(2030, 4, 2),
			StartTime: wallClock(10, 0), EndTime: wallClock(11, 0),
			Recurrence: &calendarSvc.RecurrenceRequest{Frequency: appointments.RecurrenceFrequencyMonthly, IntervalCount: 1, MonthDays: []int{2}, EndsOn: &newEndsOn}}}
}

func assertRecurrenceUpdateRolledBack(t *testing.T, fixture recurrenceUpdateRollbackFixture) {
	t.Helper()
	afterFailure, err := testpkg.TenantQuery1(t, calendarContext(t, fixture.accountID), testpkg.SetupTestDB(t), fixture.service.GetStaffAppointmentDetail, fixture.appointmentID)
	require.NoError(t, err)
	assert.Equal(t, "Original series", afterFailure.Appointment.Title)
	require.NotNil(t, afterFailure.Recurrence)
	assert.Equal(t, appointments.RecurrenceFrequencyWeekly, afterFailure.Recurrence.Frequency)
	overrides, err := fixture.config.Appointments.FindOccurrenceOverrides(testpkg.Ctx(t), []int64{fixture.appointmentID}, []appointments.Date{appointments.Date(fixture.cancelledDate)})
	require.NoError(t, err)
	require.Len(t, overrides, 1, "the old occurrence override must survive the rolled-back series edit")
}

func assertRecurrenceUpdateRetry(t *testing.T, fixture recurrenceUpdateRollbackFixture) {
	t.Helper()
	afterRetry, err := testpkg.TenantQuery2(t, calendarContext(t, fixture.accountID), testpkg.SetupTestDB(t), fixture.service.UpdateStaffAppointment, fixture.appointmentID, fixture.update)
	require.NoError(t, err)
	assert.Equal(t, "Replacement series", afterRetry.Appointment.Title)
	require.NotNil(t, afterRetry.Recurrence)
	assert.Equal(t, appointments.RecurrenceFrequencyMonthly, afterRetry.Recurrence.Frequency)
	rules, err := fixture.config.Appointments.FindRecurrenceRules(testpkg.Ctx(t), []int64{fixture.appointmentID})
	require.NoError(t, err)
	require.Len(t, rules, 1, "retry must leave exactly one recurrence rule")
	overrides, err := fixture.config.Appointments.FindOccurrenceOverrides(testpkg.Ctx(t), []int64{fixture.appointmentID}, []appointments.Date{appointments.Date(fixture.cancelledDate)})
	require.NoError(t, err)
	assert.Empty(t, overrides, "successful whole-series edit removes obsolete overrides")
}

func TestCalendarServiceIntegration_MultiDayRecurrenceVisibleOnFinalOverlapDay(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, _, _, _, reposAppointments, _, _ := calendarTestRepositories(t, db)
	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "MultiDay", "Organizer")

	endsOn := portalDate(2026, 1, 31)
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Overnight recurring appointment",
		StartDate:    portalDate(2026, 1, 31),
		EndDate:      portalDate(2026, 2, 1),
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(9, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency: appointments.RecurrenceFrequencyDaily,
			EndsOn:    &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &organizer.ID},
		},
	})
	require.NoError(t, err)

	events, err := testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 2, 1), portalDate(2026, 2, 1))
	require.NoError(t, err)
	assert.Contains(t, eventDates(events, calendarSvc.EventSourceAppointment), "2026-01-31")

	cancelledOverride := &appointments.AppointmentOccurrenceOverride{
		AppointmentID:  detail.Appointment.ID,
		OccurrenceDate: appointments.NewDate(2026, 1, 31),
		Cancelled:      true,
	}
	require.NoError(t, reposAppointments.Appointments().CreateOccurrenceOverride(calendarContext(t, organizerAccount.ID), cancelledOverride))

	events, err = testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 2, 1), portalDate(2026, 2, 1))
	require.NoError(t, err)
	assert.NotContains(t, eventDates(events, calendarSvc.EventSourceAppointment), "2026-01-31")
}

func TestCalendarServiceIntegration_ResponseAndOverviewErrors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Errors", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Errors", "Invitee")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	rsvpDetail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Response errors",
		StartDate:    portalDate(2026, 2, 15),
		EndDate:      portalDate(2026, 2, 15),
		StartTime:    wallClock(10, 0),
		EndTime:      wallClock(11, 0),
		DeliveryMode: appointments.DeliveryModeRSVPRequired,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	staffRecipientID := findRecipientByStaff(t, rsvpDetail, invitedStaff.ID)
	guardianRecipientID := findRecipientByGuardian(t, rsvpDetail, parentChain.GuardianProfileID)

	err = testpkg.TenantCommand2(t, calendarContext(t, invitedAccount.ID), db, service.RespondToStaffInvitation, staffRecipientID, appointments.ResponseStatusPending)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))

	err = testpkg.TenantCommand2(t, calendarContext(t, organizerAccount.ID), db, service.RespondToStaffInvitation, staffRecipientID, appointments.ResponseStatusAccepted)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	err = service.RespondToParentInvitation(testpkg.Ctx(t), parentChain.AccountID, guardianRecipientID, appointments.ResponseStatusPending)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))

	err = service.RespondToParentInvitation(testpkg.Ctx(t), 0, guardianRecipientID, appointments.ResponseStatusAccepted)
	assert.True(t, errors.Is(err, calendarSvc.ErrForbidden))

	err = service.RespondToParentInvitation(testpkg.Ctx(t), parentChain.AccountID, staffRecipientID, appointments.ResponseStatusAccepted)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	_, err = testpkg.TenantQuery1(t, calendarContext(t, invitedAccount.ID), db, service.GetStaffAppointmentOverview, 999999999)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	_, err = service.GetParentAppointmentOverview(testpkg.Ctx(t), parentChain.AccountID, 999999999)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))

	privateDetail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Organizer-only overview",
		StartDate:    portalDate(2026, 2, 16),
		EndDate:      portalDate(2026, 2, 16),
		StartTime:    wallClock(10, 0),
		EndTime:      wallClock(11, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)

	_, err = testpkg.TenantQuery1(t, calendarContext(t, invitedAccount.ID), db, service.GetStaffAppointmentOverview, privateDetail.Appointment.ID)
	assert.True(t, errors.Is(err, calendarSvc.ErrForbidden))

	infoDetail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Parent information only",
		StartDate:    portalDate(2026, 2, 17),
		EndDate:      portalDate(2026, 2, 17),
		StartTime:    wallClock(10, 0),
		EndTime:      wallClock(11, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	infoGuardianRecipientID := findRecipientByGuardian(t, infoDetail, parentChain.GuardianProfileID)
	err = service.RespondToParentInvitation(testpkg.Ctx(t), parentChain.AccountID, infoGuardianRecipientID, appointments.ResponseStatusAccepted)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
}

func TestCalendarServiceIntegration_RepositoryReadAndReplacePaths(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, _, _, _, reposAppointments, _, _ := calendarTestRepositories(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Repo", "Organizer")
	invitedStaff, _ := testpkg.CreateTestCalendarStaff(t, db, "Repo", "Invitee")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	endsOn := portalDate(2026, 2, 20)
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Repository coverage",
		StartDate:    portalDate(2026, 2, 13),
		EndDate:      portalDate(2026, 2, 13),
		StartTime:    wallClock(15, 0),
		EndTime:      wallClock(16, 0),
		DeliveryMode: appointments.DeliveryModeRSVPRequired,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"friday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: appointments.TargetTypeParentsByStudent, ID: &parentChain.StudentID},
		},
	})
	require.NoError(t, err)

	ctx := calendarContext(t, organizerAccount.ID)
	emptyGuardianAppointments, err := reposAppointments.Appointments().ListAppointmentsVisibleToGuardians(ctx, nil, nil, appointments.NewDate(2026, 2, 13), appointments.NewDate(2026, 2, 13))
	require.NoError(t, err)
	assert.Empty(t, emptyGuardianAppointments)

	recurrence, err := reposAppointments.Appointments().FindRecurrenceRule(ctx, detail.Appointment.ID)
	require.NoError(t, err)
	require.NotNil(t, recurrence)
	assert.Equal(t, appointments.RecurrenceFrequencyWeekly, recurrence.Frequency)

	emptyRecurrences, err := reposAppointments.Appointments().FindRecurrenceRules(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, emptyRecurrences)

	targets, err := reposAppointments.Appointments().FindAppointmentTargets(ctx, detail.Appointment.ID)
	require.NoError(t, err)
	require.Len(t, targets, 2)

	recipients, err := reposAppointments.Appointments().FindAppointmentRecipients(ctx, detail.Appointment.ID)
	require.NoError(t, err)
	recipientIDs := make([]int64, 0, len(recipients))
	for _, recipient := range recipients {
		recipientIDs = append(recipientIDs, recipient.ID)
	}
	links, err := reposAppointments.Appointments().FindAppointmentRecipientStudents(ctx, recipientIDs)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, parentChain.StudentID, links[0].StudentID)

	emptyLinks, err := reposAppointments.Appointments().FindAppointmentRecipientStudents(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, emptyLinks)

	_, err = reposAppointments.Appointments().ReplaceAppointmentTargets(ctx, detail.Appointment.ID, nil)
	require.NoError(t, err)
	targets, err = reposAppointments.Appointments().FindAppointmentTargets(ctx, detail.Appointment.ID)
	require.NoError(t, err)
	assert.Empty(t, targets)

	require.NoError(t, reposAppointments.Appointments().DeleteRecurrenceRule(ctx, detail.Appointment.ID))
	recurrence, err = reposAppointments.Appointments().FindRecurrenceRule(ctx, detail.Appointment.ID)
	require.NoError(t, err)
	assert.Nil(t, recurrence)
}

func TestCalendarServiceIntegration_StaffCalendarIncludesAssignedTimetable(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	staff, account := testpkg.CreateTestCalendarStaff(t, db, "Timetable", "Staff")
	room := testpkg.CreateTestRoom(t, db, "Calendar Timetable Room")
	instance := testpkg.CreateTestActivityInstance(t, db, portalDate(2026, 3, 3), room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:15",
		EndHHMM:   "15:45",
		Title:     "Assigned Betreuung",
	})
	testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})

	events, err := testpkg.TenantQuery2(t, calendarContext(t, account.ID), db, service.ListMyStaffEvents, portalDate(2026, 3, 3), portalDate(2026, 3, 3))
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, calendarSvc.EventSourceTimetable, events[0].Source)
	assert.Equal(t, "Assigned Betreuung", events[0].Title)
	assert.Equal(t, "14:15", events[0].StartTime)
	assert.Equal(t, "15:45", events[0].EndTime)
	assert.False(t, events[0].CanRespond)
}

func TestCalendarServiceIntegration_StaffCalendarIncludesShifts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	staff, account := testpkg.CreateTestCalendarStaff(t, db, "Shift", "Staff")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var shiftTypeID int64
	err := db.NewRaw(`INSERT INTO schedule.shift_types (tenant_id, name, color, description, is_active)
		VALUES (?, ?, ?, '', true) RETURNING id`, testpkg.Tenant(t), "Frühdienst", "#F78C10").Scan(ctx, &shiftTypeID)
	require.NoError(t, err)

	day := portalDate(2026, 5, 4)
	testpkg.CreateTestStaffShift(t, db, staff.ID, day, testpkg.StaffShiftOpts{
		StartHHMM:   "08:00",
		EndHHMM:     "12:00",
		Notes:       "Vertretung Gruppe B",
		ShiftTypeID: &shiftTypeID,
	})
	testpkg.CreateTestStaffShift(t, db, staff.ID, day, testpkg.StaffShiftOpts{
		StartHHMM: "13:00",
		EndHHMM:   "16:00",
	})
	testpkg.CreateTestStaffShift(t, db, staff.ID, day.AddDays(1), testpkg.StaffShiftOpts{
		Cancelled: true,
	})

	events, err := testpkg.TenantQuery2(t, calendarContext(t, account.ID), db, service.ListMyStaffEvents, day, day.AddDays(1))
	require.NoError(t, err)
	require.Len(t, events, 2)

	assert.Equal(t, calendarSvc.EventSourceShift, events[0].Source)
	assert.Equal(t, "Frühdienst", events[0].Title)
	assert.Equal(t, "08:00", events[0].StartTime)
	assert.Equal(t, "12:00", events[0].EndTime)
	require.NotNil(t, events[0].Description)
	assert.Equal(t, "Vertretung Gruppe B", *events[0].Description)
	assert.False(t, events[0].CanRespond)
	assert.False(t, events[0].CanEdit)

	assert.Equal(t, calendarSvc.EventSourceShift, events[1].Source)
	assert.Equal(t, "Dienst", events[1].Title)
	assert.Equal(t, "13:00", events[1].StartTime)
	assert.Equal(t, "16:00", events[1].EndTime)
	assert.Nil(t, events[1].Description)
}

func TestCalendarServiceIntegration_ParentCalendarExcludesChildTimetable(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	room := testpkg.CreateTestRoom(t, db, "Parent Calendar Room")
	instance := testpkg.CreateTestActivityInstance(t, db, portalDate(2026, 4, 8), room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:00",
		EndHHMM:   "16:00",
		Title:     "Child Betreuung",
	})
	testpkg.CreateTestInstanceStudent(t, db, instance.ID, parentChain.StudentID, "")

	events, err := service.ListMyParentEvents(testpkg.Ctx(t), parentChain.AccountID, portalDate(2026, 4, 8), portalDate(2026, 4, 8))
	require.NoError(t, err)
	assert.Empty(t, events)
}

// The public subscription feed bypasses parent auth, so a deactivated account
// must lose feed access immediately — the token alone is not enough.
func TestCalendarServiceIntegration_FeedRejectsInactiveAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	cfg := calendarTestConfig(t, db)
	repos, _, _, _, _, _, _ := calendarTestRepositories(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, nil, nil)
	cfg.ParentsURL = "https://parents.test"
	service := owners.NewCalendarPortal(cfg)

	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Inactive", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Elterninfo",
		StartDate:    calendarSvc.Date(calendar.NewDate(2026, time.January, 1)).AddDays(3),
		EndDate:      calendarSvc.Date(calendar.NewDate(2026, time.January, 1)).AddDays(3),
		StartTime:    wallClock(15, 0),
		EndTime:      wallClock(16, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	httpsURL, _, err := service.ParentCalendarFeedURL(testpkg.Ctx(t), parentChain.AccountID)
	require.NoError(t, err)
	token := strings.TrimPrefix(httpsURL, "https://parents.test/api/calendar-feed/")

	// While active, the feed serves the family's appointments.
	_, content, err := service.ParentCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)
	assert.Contains(t, content, "SUMMARY:Elterninfo")

	// Deactivate the account (tenant mappings stay active).
	_, err = db.NewUpdate().
		ModelTableExpr("auth.accounts").
		Set("active = ?", false).
		Where("id = ?", parentChain.AccountID).
		Exec(context.Background())
	require.NoError(t, err)

	// The same token now behaves like an unknown token: a plain not-found, no leak.
	_, _, err = service.ParentCalendarFeedByToken(testpkg.Ctx(t), token)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrNotFound))
}

// Editing a recurring series is a whole-series operation: stale single-occurrence
// cancellations from the old cadence must not survive to suppress valid dates in
// the edited series.
func TestCalendarServiceIntegration_SeriesEditClearsOccurrenceOverrides(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "SeriesEdit", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "SeriesEdit", "Invitee")

	endsOn := portalDate(2026, 1, 19)
	weekly := func() *calendarSvc.RecurrenceRequest {
		return &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		}
	}
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Weekly standup",
		StartDate:    portalDate(2026, 1, 5), // Monday
		EndDate:      portalDate(2026, 1, 5),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(9, 30),
		DeliveryMode: appointments.DeliveryModeRSVPRequired,
		Recurrence:   weekly(),
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)

	// Cancel the middle occurrence, confirm it disappears.
	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointmentOccurrence, detail.Appointment.ID, portalDate(2026, 1, 12)))
	events, err := testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 1, 5), portalDate(2026, 1, 19))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-19"}, eventDates(events, calendarSvc.EventSourceAppointment))

	// Edit the series (same cadence). The stale cancellation must be cleared, so
	// 2026-01-12 reappears in the edited series.
	_, err = testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.UpdateStaffAppointment, detail.Appointment.ID, calendarSvc.UpdateAppointmentRequest{
		Title:      "Weekly standup (edited)",
		StartDate:  portalDate(2026, 1, 5),
		EndDate:    portalDate(2026, 1, 5),
		StartTime:  wallClock(9, 0),
		EndTime:    wallClock(9, 30),
		Recurrence: weekly(),
	})
	require.NoError(t, err)

	events, err = testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 1, 5), portalDate(2026, 1, 19))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-12", "2026-01-19"}, eventDates(events, calendarSvc.EventSourceAppointment))
}

// A weekly rule whose weekdays exclude the StartDate weekday must export a
// DTSTART on the first matching weekday, matching the in-app expansion, so
// external calendars don't render a phantom occurrence on the StartDate.
func TestCalendarServiceIntegration_RecurringICSStartsAtFirstMatchingWeekday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "FirstDay", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "FirstDay", "Invitee")

	endsOn := portalDate(2026, 5, 27)
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Mittwochs-AG",
		StartDate:    portalDate(2026, 5, 4), // Monday — NOT a selected weekday
		EndDate:      portalDate(2026, 5, 4),
		StartTime:    wallClock(14, 0),
		EndTime:      wallClock(15, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"wednesday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)

	// The in-app expansion starts on the first Wednesday (2026-05-06), not the
	// Monday StartDate.
	events, err := testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 5, 4), portalDate(2026, 5, 6))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-05-06"}, eventDates(events, calendarSvc.EventSourceAppointment))

	// The ICS export must anchor DTSTART on that same first Wednesday.
	_, content, err := testpkg.TenantQueryPair1(t, calendarContext(t, organizerAccount.ID), db, service.StaffAppointmentICS, detail.Appointment.ID)
	require.NoError(t, err)
	assert.Contains(t, content, "DTSTART;TZID=Europe/Berlin:20260506T140000")
	assert.NotContains(t, content, "20260504T140000", "must not anchor DTSTART on the non-matching StartDate")
	assert.Contains(t, content, "RRULE:FREQ=WEEKLY;BYDAY=WE")
}

// Editing an appointment must cancel any not-yet-sent notice queued by an earlier
// create/update, so the worker can't deliver stale details — regardless of
// whether the edit re-sends email.
func TestCalendarServiceIntegration_UpdateCancelsPendingNotifications(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "UpdateMail", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Elternabend",
		StartDate:    portalDate(2026, 4, 2),
		EndDate:      portalDate(2026, 4, 2),
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(19, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		SendEmail:    true,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	require.Len(t, outbox.enqueued, 1)

	base := calendarSvc.UpdateAppointmentRequest{
		Title:        "Elternabend (verschoben)",
		StartDate:    portalDate(2026, 4, 3),
		EndDate:      portalDate(2026, 4, 3),
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(19, 0),
		SendEmailSet: true,
	}

	// Edit WITHOUT re-sending: the stale pending mail is cancelled and no new mail
	// is queued.
	_, err = testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.UpdateStaffAppointment, detail.Appointment.ID, base)
	require.NoError(t, err)
	assert.Equal(t, 1, outbox.cancelled, "update must cancel pending mail even when not re-sending")
	require.Len(t, outbox.enqueued, 1, "no new mail queued when send_email is off")
	var notifyGuardians bool
	require.NoError(t, db.NewSelect().
		TableExpr(`calendar.appointments AS "appointment"`).
		ColumnExpr(`"appointment".notify_guardians`).
		Where(`"appointment".id = ?`, detail.Appointment.ID).
		Scan(context.Background(), &notifyGuardians))
	assert.False(t, notifyGuardians, "the updated choice must also suppress future reminders")

	// Edit WITH re-sending: cancels again, then queues a single update notice.
	resend := base
	resend.SendEmail = true
	_, err = testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.UpdateStaffAppointment, detail.Appointment.ID, resend)
	require.NoError(t, err)
	assert.Equal(t, 2, outbox.cancelled)
	require.Len(t, outbox.enqueued, 2)
	assert.Equal(t, emailKindAppointmentUpdated, outbox.enqueued[1].Kind)
	require.NoError(t, db.NewSelect().
		TableExpr(`calendar.appointments AS "appointment"`).
		ColumnExpr(`"appointment".notify_guardians`).
		Where(`"appointment".id = ?`, detail.Appointment.ID).
		Scan(context.Background(), &notifyGuardians))
	assert.True(t, notifyGuardians, "re-enabling e-mail must restore reminder eligibility")
}

// Older clients may omit send_email on updates. In that case the existing
// guardian notification preference must be retained for this update and later
// appointment lifecycle notices.
func TestCalendarServiceIntegration_UpdateRetainsGuardianNotificationsWhenSendEmailOmitted(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "LegacyMail", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Elternabend",
		StartDate:    portalDate(2026, 4, 2),
		EndDate:      portalDate(2026, 4, 2),
		StartTime:    wallClock(18, 0),
		EndTime:      wallClock(19, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		SendEmail:    true,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	require.Len(t, outbox.enqueued, 1)

	_, err = testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.UpdateStaffAppointment, detail.Appointment.ID, calendarSvc.UpdateAppointmentRequest{
		Title:     "Elternabend (verschoben)",
		StartDate: portalDate(2026, 4, 3),
		EndDate:   portalDate(2026, 4, 3),
		StartTime: wallClock(18, 0),
		EndTime:   wallClock(19, 0),
	})
	require.NoError(t, err)
	assert.Equal(t, 1, outbox.cancelled, "update must cancel the stale pending mail")
	require.Len(t, outbox.enqueued, 2, "omitting send_email must retain guardian notifications")
	assert.Equal(t, emailKindAppointmentUpdated, outbox.enqueued[1].Kind)
}

// The notification e-mail must advertise the first real occurrence, not the raw
// StartDate, for a weekly rule whose weekdays exclude the StartDate weekday —
// matching the in-app calendar and ICS export.
func TestCalendarServiceIntegration_RecurringEmailUsesFirstOccurrenceDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "MailDate", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	endsOn := portalDate(2026, 5, 27)
	_, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Mittwochs-AG",
		StartDate:    portalDate(2026, 5, 4), // Monday — NOT a selected weekday
		EndDate:      portalDate(2026, 5, 4),
		StartTime:    wallClock(14, 0),
		EndTime:      wallClock(15, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		SendEmail:    true,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"wednesday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	require.Len(t, outbox.enqueued, 1)
	whenText, _ := outbox.enqueued[0].Payload["when_text"].(string)
	assert.Contains(t, whenText, "06.05.2026", "e-mail must advertise the first Wednesday")
	assert.NotContains(t, whenText, "04.05.2026", "e-mail must not advertise the non-matching Monday StartDate")
}

// A cancelled appointment must be non-respondable: the API shape flips
// CanRespond to false and the response endpoints reject RSVP changes.
func TestCalendarServiceIntegration_CancelledAppointmentNotRespondable(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "CancelRSVP", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "CancelRSVP", "Invitee")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Elterngespräch",
		StartDate:    portalDate(2026, 6, 3),
		EndDate:      portalDate(2026, 6, 3),
		StartTime:    wallClock(16, 0),
		EndTime:      wallClock(16, 30),
		DeliveryMode: appointments.DeliveryModeRSVPRequired,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	staffRecipientID := findRecipientByStaff(t, detail, invitedStaff.ID)
	parentRecipientID := findRecipientByGuardian(t, detail, parentChain.GuardianProfileID)

	// While active, responses are accepted and CanRespond is true.
	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, invitedAccount.ID), db, service.RespondToStaffInvitation, staffRecipientID, appointments.ResponseStatusAccepted))
	events, err := testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 6, 3), portalDate(2026, 6, 3))
	require.NoError(t, err)
	require.NotEmpty(t, events)
	assert.True(t, events[0].CanRespond)

	// Cancel the appointment.
	_, err = testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointment, detail.Appointment.ID)
	require.NoError(t, err)

	// The API shape now forbids responding.
	events, err = testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 6, 3), portalDate(2026, 6, 3))
	require.NoError(t, err)
	require.NotEmpty(t, events)
	assert.True(t, events[0].Cancelled)
	assert.False(t, events[0].CanRespond, "cancelled appointment must not be respondable")

	// Both response endpoints reject RSVP changes after cancellation.
	err = testpkg.TenantCommand2(t, calendarContext(t, invitedAccount.ID), db, service.RespondToStaffInvitation, staffRecipientID, appointments.ResponseStatusDeclined)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))

	err = service.RespondToParentInvitation(testpkg.Ctx(t), parentChain.AccountID, parentRecipientID, appointments.ResponseStatusDeclined)
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
}

// A cancelled appointment is terminal: editing it (which would re-notify while
// it stays cancelled) must be rejected.
func TestCalendarServiceIntegration_EditCancelledAppointmentRejected(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "EditCancel", "Organizer")
	invitedStaff, _ := testpkg.CreateTestCalendarStaff(t, db, "EditCancel", "Invitee")

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Termin",
		StartDate:    portalDate(2026, 8, 3),
		EndDate:      portalDate(2026, 8, 3),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)

	_, err = testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointment, detail.Appointment.ID)
	require.NoError(t, err)

	_, err = testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.UpdateStaffAppointment, detail.Appointment.ID, calendarSvc.UpdateAppointmentRequest{
		Title:     "Termin (editiert)",
		StartDate: portalDate(2026, 8, 4),
		EndDate:   portalDate(2026, 8, 4),
		StartTime: wallClock(9, 0),
		EndTime:   wallClock(10, 0),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
}

// Single-occurrence cancellation must reject non-recurring appointments and
// dates the series never generates.
func TestCalendarServiceIntegration_CancelOccurrenceValidation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "OccValid", "Organizer")
	invitedStaff, _ := testpkg.CreateTestCalendarStaff(t, db, "OccValid", "Invitee")

	// Non-recurring appointment: cancelling an occurrence is rejected.
	single, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Einmalig",
		StartDate:    portalDate(2026, 9, 1),
		EndDate:      portalDate(2026, 9, 1),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)

	err = testpkg.TenantCommand2(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointmentOccurrence, single.Appointment.ID, portalDate(2026, 9, 1))
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))

	// Recurring appointment: a date the series does not generate is rejected, a
	// generated date succeeds.
	endsOn := portalDate(2026, 9, 28)
	recurring, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Montags",
		StartDate:    portalDate(2026, 9, 7), // Monday
		EndDate:      portalDate(2026, 9, 7),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)

	// 2026-09-08 is a Tuesday — not part of the Monday series.
	err = testpkg.TenantCommand2(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointmentOccurrence, recurring.Appointment.ID, portalDate(2026, 9, 8))
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))

	// 2026-09-14 is a generated Monday — accepted.
	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointmentOccurrence, recurring.Appointment.ID, portalDate(2026, 9, 14)))
}

// A recurrence that generates no occurrence (weekly weekday outside its EndsOn
// window) must be rejected at create time rather than persisting a phantom.
func TestCalendarServiceIntegration_EmptyRecurrenceRejected(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "EmptyRec", "Organizer")
	invitedStaff, _ := testpkg.CreateTestCalendarStaff(t, db, "EmptyRec", "Invitee")

	endsOn := portalDate(2026, 5, 5) // Tuesday — no Wednesday in [Mon, Tue]
	_, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Nie",
		StartDate:    portalDate(2026, 5, 4), // Monday
		EndDate:      portalDate(2026, 5, 4),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"wednesday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
}

// The exported ICS SEQUENCE advances on every change (edit and single-occurrence
// cancellation) so subscribers treat the event as a newer revision.
func TestCalendarServiceIntegration_ICSRevisionSequence(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Revision", "Organizer")
	invitedStaff, _ := testpkg.CreateTestCalendarStaff(t, db, "Revision", "Invitee")

	endsOn := portalDate(2026, 10, 26)
	weekly := func() *calendarSvc.RecurrenceRequest {
		return &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		}
	}
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Reihe",
		StartDate:    portalDate(2026, 10, 5), // Monday
		EndDate:      portalDate(2026, 10, 5),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence:   weekly(),
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)

	// Freshly created: revision 0, so no SEQUENCE line.
	_, content, err := testpkg.TenantQueryPair1(t, calendarContext(t, organizerAccount.ID), db, service.StaffAppointmentICS, detail.Appointment.ID)
	require.NoError(t, err)
	assert.NotContains(t, content, "SEQUENCE:")

	// Editing bumps the revision → SEQUENCE:1.
	_, err = testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.UpdateStaffAppointment, detail.Appointment.ID, calendarSvc.UpdateAppointmentRequest{
		Title:      "Reihe (v2)",
		StartDate:  portalDate(2026, 10, 5),
		EndDate:    portalDate(2026, 10, 5),
		StartTime:  wallClock(9, 0),
		EndTime:    wallClock(10, 0),
		Recurrence: weekly(),
	})
	require.NoError(t, err)
	_, content, err = testpkg.TenantQueryPair1(t, calendarContext(t, organizerAccount.ID), db, service.StaffAppointmentICS, detail.Appointment.ID)
	require.NoError(t, err)
	assert.Contains(t, content, "SEQUENCE:1")
	assert.Contains(t, content, "LAST-MODIFIED:")

	// Cancelling a single occurrence bumps it again → SEQUENCE:2 (and an EXDATE).
	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointmentOccurrence, detail.Appointment.ID, portalDate(2026, 10, 12)))
	_, content, err = testpkg.TenantQueryPair1(t, calendarContext(t, organizerAccount.ID), db, service.StaffAppointmentICS, detail.Appointment.ID)
	require.NoError(t, err)
	assert.Contains(t, content, "SEQUENCE:2")
	assert.Contains(t, content, "EXDATE;TZID=Europe/Berlin:20261012T090000")
}

// A sparse-but-valid recurrence whose first occurrence is more than a year out
// (unbounded, so it genuinely produces occurrences) must NOT be rejected as
// empty.
func TestCalendarServiceIntegration_SparseRecurrenceAccepted(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Sparse", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Sparse", "Invitee")

	// Monthly, every 24 months, on day 30. Start 2026-01-31 (day 30 in Jan is
	// before the start), so the first real occurrence is 2028-01-30 — over a year
	// out but perfectly valid.
	_, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Alle zwei Jahre",
		StartDate:    portalDate(2026, 1, 31),
		EndDate:      portalDate(2026, 1, 31),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyMonthly,
			IntervalCount: 24,
			MonthDays:     []int{30},
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)

	// The first occurrence lands on 2028-01-30, in-app.
	events, err := testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2028, 1, 30), portalDate(2028, 1, 30))
	require.NoError(t, err)
	assert.Equal(t, []string{"2028-01-30"}, eventDates(events, calendarSvc.EventSourceAppointment))
}

// The subscription feed must not export a count-bounded recurring series whose
// occurrences are all before the feed lookback window (the SQL window treats a
// NULL ends_on as open-ended).
func TestCalendarServiceIntegration_FeedSkipsExpiredCountBoundedSeries(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	cfg := calendarTestConfig(t, db)
	repos, _, _, _, _, _, _ := calendarTestRepositories(t, db)
	cfg = owners.BindCalendarFeedFacts(cfg, nil, repos.ParentCalendarFeed, nil, nil)
	cfg.ParentsURL = "https://parents.test"
	service := owners.NewCalendarPortal(cfg)

	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "ExpiredFeed", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	// A weekly series bounded by occurrence_count=3, starting 200 days ago: all
	// three occurrences are well before the feed's 30-day lookback.
	count := 3
	pastStart := calendarSvc.Date(calendar.NewDate(2026, time.January, 1)).AddDays(-200)
	_, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Abgelaufene Reihe",
		StartDate:    pastStart,
		EndDate:      pastStart,
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:       appointments.RecurrenceFrequencyWeekly,
			IntervalCount:   1,
			OccurrenceCount: &count,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	// A current appointment inside the feed window.
	_, err = testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Aktueller Termin",
		StartDate:    calendarSvc.Date(calendar.NewDate(2026, time.January, 1)).AddDays(5),
		EndDate:      calendarSvc.Date(calendar.NewDate(2026, time.January, 1)).AddDays(5),
		StartTime:    wallClock(15, 0),
		EndTime:      wallClock(16, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)

	httpsURL, _, err := service.ParentCalendarFeedURL(testpkg.Ctx(t), parentChain.AccountID)
	require.NoError(t, err)
	token := strings.TrimPrefix(httpsURL, "https://parents.test/api/calendar-feed/")
	_, content, err := service.ParentCalendarFeedByToken(testpkg.Ctx(t), token)
	require.NoError(t, err)

	assert.Contains(t, content, "SUMMARY:Aktueller Termin", "current appointment must be in the feed")
	assert.NotContains(t, content, "SUMMARY:Abgelaufene Reihe", "expired count-bounded series must be excluded")
}

// Cancelling the same occurrence twice (as two concurrent requests would) must
// converge via the conflict-safe upsert instead of the second insert hitting the
// (tenant, appointment, date) unique constraint and returning a 500.
func TestCalendarServiceIntegration_OccurrenceCancelIsConflictSafe(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	appointmentCapability := api.NewCalendarAppointmentRows(testpkg.NewDatabase(t, db)).Appointments()
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Conflict", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Conflict", "Invitee")

	endsOn := portalDate(2026, 1, 26)
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Montags",
		StartDate:    portalDate(2026, 1, 5), // Monday
		EndDate:      portalDate(2026, 1, 5),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(9, 30),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.NoError(t, err)

	// The insert path is exercised twice for the same occurrence; the second call
	// takes the ON CONFLICT DO UPDATE branch and must NOT error.
	ctx := testpkg.Ctx(t)
	_, err = appointmentCapability.CancelAppointmentOccurrence(ctx, detail.Appointment.ID, appointments.NewDate(2026, 1, 12))
	require.NoError(t, err)
	_, err = appointmentCapability.CancelAppointmentOccurrence(ctx, detail.Appointment.ID, appointments.NewDate(2026, 1, 12))
	require.NoError(t, err)

	// The occurrence is excluded from the series exactly once.
	events, err := testpkg.TenantQuery2(t, calendarContext(t, invitedAccount.ID), db, service.ListMyStaffEvents, portalDate(2026, 1, 5), portalDate(2026, 1, 26))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-05", "2026-01-19", "2026-01-26"}, eventDates(events, calendarSvc.EventSourceAppointment))
}

// A monthly rule that can never produce a valid occurrence (February-only on day
// 31) must be rejected at create time rather than persisting an invisible,
// phantom-exporting appointment.
func TestCalendarServiceIntegration_ImpossibleMonthlyRecurrenceRejected(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Impossible", "Organizer")
	invitedStaff, _ := testpkg.CreateTestCalendarStaff(t, db, "Impossible", "Invitee")

	_, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Nie",
		StartDate:    portalDate(2026, 2, 15), // February — reached every 12 months
		EndDate:      portalDate(2026, 2, 15),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyMonthly,
			IntervalCount: 12,
			MonthDays:     []int{31}, // February never has a 31st
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, calendarSvc.ErrInvalidRequest))
}

// Cancelling a single occurrence must clear any pending create/update notice, so
// a queued email can't announce a date parents will no longer see (mirrors the
// cancel/delete/update stale-notification cleanup).
func TestCalendarServiceIntegration_CancelOccurrenceClearsPendingNotifications(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	outbox := &recordingOutbox{}
	pushOutbox := &recordingPushOutbox{}
	cfg := calendarTestConfig(t, db)
	cfg.Outbox = outbox
	cfg.PushOutbox = pushOutbox
	cfg.ParentsURL = "https://parents.test"
	service := owners.NewCalendarPortal(cfg)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "OccMail", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	endsOn := portalDate(2026, 1, 26)
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Wöchentlich",
		StartDate:    portalDate(2026, 1, 5), // Monday
		EndDate:      portalDate(2026, 1, 5),
		StartTime:    wallClock(9, 0),
		EndTime:      wallClock(10, 0),
		DeliveryMode: appointments.DeliveryModeInformational,
		SendEmail:    true,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency:     appointments.RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
			EndsOn:        &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	require.Len(t, outbox.enqueued, 1, "create with send_email queues one notice")

	// Removing a single occurrence clears the still-pending notice.
	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointmentOccurrence, detail.Appointment.ID, portalDate(2026, 1, 12)))
	assert.Equal(t, 1, outbox.cancelled, "single-occurrence cancel must clear pending notifications")
	assert.Equal(t, 1, pushOutbox.cancelled, "single-occurrence cancel must clear pending pushes")
}

func TestCalendarServiceIntegration_StaffTimetableEventsCarryRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupCalendarService(t, db)
	staff, account := testpkg.CreateTestCalendarStaff(t, db, "Room", "Staff")
	mainRoom := testpkg.CreateTestRoom(t, db, "Calendar Main Room")
	overrideRoom := testpkg.CreateTestRoom(t, db, "Calendar Override Room")

	day := portalDate(2026, 3, 10)
	plainInstance := testpkg.CreateTestActivityInstance(t, db, day, mainRoom.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "09:00",
		EndHHMM:   "10:00",
		Title:     "Betreuung Hauptraum",
	})
	splitInstance := testpkg.CreateTestActivityInstance(t, db, day, mainRoom.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "11:00",
		EndHHMM:   "12:00",
		Title:     "Betreuung Split",
	})
	testpkg.CreateTestInstanceStaff(t, db, plainInstance.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
	testpkg.CreateTestInstanceStaff(t, db, splitInstance.ID, staff.ID, testpkg.InstanceStaffOpts{
		IsPrimary: true,
		RoomID:    &overrideRoom.ID,
	})
	testpkg.CreateTestStaffShift(t, db, staff.ID, day, testpkg.StaffShiftOpts{
		StartHHMM: "14:00",
		EndHHMM:   "15:00",
	})

	events, err := testpkg.TenantQuery2(t, calendarContext(t, account.ID), db, service.ListMyStaffEvents, day, day)
	require.NoError(t, err)
	require.Len(t, events, 3)

	// Instance room, no override.
	assert.Equal(t, calendarSvc.EventSourceTimetable, events[0].Source)
	require.NotNil(t, events[0].Location)
	assert.Equal(t, mainRoom.Name, *events[0].Location)

	// Per-person override wins over the instance room (Lernzeit split, #2078).
	assert.Equal(t, calendarSvc.EventSourceTimetable, events[1].Source)
	require.NotNil(t, events[1].Location)
	assert.Equal(t, overrideRoom.Name, *events[1].Location)

	// Shifts carry no room column, so Location stays deliberately empty.
	assert.Equal(t, calendarSvc.EventSourceShift, events[2].Source)
	assert.Nil(t, events[2].Location)
}

type recordingPushOutbox struct{ cancelled int }

func (r *recordingPushOutbox) CancelPendingByRelatedEntity(context.Context, string, int64, string) (int64, error) {
	r.cancelled++
	return 0, nil
}

func portalDate(year int, month time.Month, day int) appointments.Date {
	return appointments.NewDate(year, month, day)
}

func calendarTestRepositories(t *testing.T, db *bun.DB) (api.CalendarTestRepositories, owners.MembershipTestRepositories, api.CalendarSchoolRows, owners.TimetableTestRepositories, api.CalendarAppointmentRows, api.CalendarParentRows, owners.TimetableRoomRows) {
	t.Helper()
	database := testpkg.NewDatabase(t, db)
	members := owners.MustNewMembershipTestRepositories(database)
	repositories, err := api.NewCalendarTestRepositories(database, members)
	require.NoError(t, err)
	return repositories, members, api.NewCalendarSchoolRows(database), api.NewCalendarTimetableRows(database, members),
		api.NewCalendarAppointmentRows(database), api.NewCalendarParentRows(database, members), api.MustNewTimetableRoomRows(database)
}
