package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/internal/ptrtest"
	appointmentcap "github.com/moto-ogs/project-phoenix/modules/appointments"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar/portal/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func helperClock(hour, minute int) time.Time {
	return normalizeWallClock(time.Date(2026, 1, 1, hour, minute, 0, 0, time.UTC))
}

func helperAppointment() *appointmentcap.Appointment {
	return &appointmentcap.Appointment{
		ID:                 42,
		OrganizerStaffID:   7,
		Title:              "Planning",
		StartDate:          appointmentcap.NewDate(2026, 1, 5),
		EndDate:            appointmentcap.NewDate(2026, 1, 5),
		StartTime:          helperClock(9, 0),
		EndTime:            helperClock(10, 0),
		DeliveryMode:       appointmentcap.DeliveryModeRSVPRequired,
		OverviewVisibility: appointmentcap.OverviewVisibilityOrganizer,
	}
}

func TestCalendarOverviewVisibilityHelpers(t *testing.T) {
	t.Parallel()

	appointment := helperAppointment()

	assert.False(t, canStaffViewOverview(nil, 7, true))
	assert.True(t, canStaffViewOverview(appointment, 7, false))
	assert.False(t, canStaffViewOverview(appointment, 8, true))

	appointment.OverviewVisibility = appointmentcap.OverviewVisibilityStaff
	assert.True(t, canStaffViewOverview(appointment, 8, true))
	assert.False(t, canParentViewOverview(appointment, true))

	appointment.OverviewVisibility = appointmentcap.OverviewVisibilityAll
	assert.True(t, canStaffViewOverview(appointment, 8, true))
	assert.True(t, canParentViewOverview(appointment, true))
	assert.False(t, canParentViewOverview(appointment, false))
}

func TestAppointmentEventAndOverride(t *testing.T) {
	t.Parallel()

	appointment := helperAppointment()
	appointment.OverviewVisibility = appointmentcap.OverviewVisibilityAll
	status := appointmentcap.ResponseStatusPending
	recipientID := int64(99)

	event := appointmentEvent(appointment, appointmentcap.NewDate(2026, 1, 6), &status, &recipientID, 8)

	assert.Equal(t, "appointment:42:2026-01-06", event.ID)
	assert.Equal(t, EventSourceAppointment, event.Source)
	assert.Equal(t, "2026-01-06", event.StartDate)
	assert.Equal(t, "09:00", event.StartTime)
	assert.True(t, event.CanRespond)
	assert.True(t, event.CanViewOverview)
	assert.False(t, event.CanEdit)

	title := "Changed"
	description := "New details"
	location := "Room 2"
	startDate := appointmentcap.NewDate(2026, 1, 7)
	endDate := appointmentcap.NewDate(2026, 1, 8)
	startTime := helperClock(11, 30)
	endTime := helperClock(12, 45)
	allDay := true
	applyOverride(&event, &appointmentcap.AppointmentOccurrenceOverride{
		Title:       &title,
		Description: &description,
		Location:    &location,
		StartDate:   &startDate,
		EndDate:     &endDate,
		StartTime:   &startTime,
		EndTime:     &endTime,
		AllDay:      &allDay,
	})

	assert.Equal(t, "Changed", event.Title)
	require.NotNil(t, event.Description)
	assert.Equal(t, "New details", *event.Description)
	require.NotNil(t, event.Location)
	assert.Equal(t, "Room 2", *event.Location)
	assert.Equal(t, "2026-01-07", event.StartDate)
	assert.Equal(t, "2026-01-08", event.EndDate)
	assert.Equal(t, "11:30", event.StartTime)
	assert.Equal(t, "12:45", event.EndTime)
	assert.True(t, event.AllDay)
}

func TestAppointmentEventCanRespond(t *testing.T) {
	t.Parallel()

	appointment := helperAppointment()
	recipientID := int64(99)
	day := appointmentcap.NewDate(2026, 1, 6)

	// A recipient can respond for any non-informational status, including an
	// already accepted/declined one — the respond endpoints allow changing it.
	for _, status := range []string{
		appointmentcap.ResponseStatusPending,
		appointmentcap.ResponseStatusAccepted,
		appointmentcap.ResponseStatusDeclined,
	} {
		s := status
		event := appointmentEvent(appointment, day, &s, &recipientID, 8)
		assert.Truef(t, event.CanRespond, "status %q should be respondable", status)
	}

	// Informational recipients cannot respond.
	info := appointmentcap.ResponseStatusInfo
	infoEvent := appointmentEvent(appointment, day, &info, &recipientID, 8)
	assert.False(t, infoEvent.CanRespond)

	// A non-recipient (nil recipient/status) cannot respond.
	nonRecipient := appointmentEvent(appointment, day, nil, nil, 8)
	assert.False(t, nonRecipient.CanRespond)
}

func TestStaffRecipientStatus(t *testing.T) {
	t.Parallel()

	staffID := int64(10)
	recipients := []*appointmentcap.AppointmentRecipient{
		{ID: 101, RecipientType: appointmentcap.RecipientTypeStaff, StaffID: &staffID, Status: appointmentcap.ResponseStatusAccepted},
	}

	recipient := staffRecipient(recipients, staffID)
	require.NotNil(t, recipient)
	assert.Equal(t, appointmentcap.ResponseStatusAccepted, recipient.Status)
	assert.Equal(t, int64(101), recipient.ID)

	assert.Nil(t, staffRecipient(recipients, 99))
}

// An occurrence of a series shows the answer for its date; the other dates
// keep the series answer (#6987).
func TestAnsweredAppointmentEventTakesTheAnswerOfItsDate(t *testing.T) {
	t.Parallel()

	staffID := int64(10)
	appointment := &appointmentcap.Appointment{ID: 7, OrganizerStaffID: 1, Title: "Elterncafé", StartDate: appointmentcap.NewDate(2026, 11, 18), EndDate: appointmentcap.NewDate(2026, 11, 18), DeliveryMode: appointmentcap.DeliveryModeRSVPRequired}
	declinedDate := appointmentcap.NewDate(2026, 11, 25)
	recipient := &appointmentcap.AppointmentRecipient{
		ID: 101, RecipientType: appointmentcap.RecipientTypeStaff, StaffID: &staffID, Status: appointmentcap.ResponseStatusAccepted,
		OccurrenceResponses: map[appointmentcap.Date]string{declinedDate: appointmentcap.ResponseStatusDeclined},
	}

	declined := answeredAppointmentEvent(appointment, declinedDate, recipient, staffID)
	require.NotNil(t, declined.ResponseStatus)
	require.NotNil(t, declined.SeriesResponseStatus)
	assert.Equal(t, appointmentcap.ResponseStatusDeclined, *declined.ResponseStatus)
	assert.Equal(t, appointmentcap.ResponseStatusAccepted, *declined.SeriesResponseStatus)
	assert.True(t, declined.CanRespond)

	accepted := answeredAppointmentEvent(appointment, appointmentcap.NewDate(2026, 11, 18), recipient, staffID)
	require.NotNil(t, accepted.ResponseStatus)
	assert.Equal(t, appointmentcap.ResponseStatusAccepted, *accepted.ResponseStatus)

	outsider := answeredAppointmentEvent(appointment, declinedDate, nil, 99)
	assert.Nil(t, outsider.ResponseStatus)
	assert.Nil(t, outsider.SeriesResponseStatus)
	assert.False(t, outsider.CanRespond)
}

func TestAppointmentICSEventUIDIncludesTenant(t *testing.T) {
	t.Parallel()

	// The parent feed aggregates appointments across schools, so the UID must be
	// globally unique — appointment IDs repeat per tenant.
	appt := &appointmentcap.Appointment{
		ID:        5,
		TenantID:  42,
		Title:     "Termin",
		StartDate: appointmentcap.NewDate(2026, 1, 5),
		EndDate:   appointmentcap.NewDate(2026, 1, 5),
	}
	event := appointmentICSEvent(appt, nil, nil)
	assert.Equal(t, "appointment-42-5@moto-app.de", event.UID)
}

func TestAppointmentICSEventExportsUnclampedRecurrence(t *testing.T) {
	t.Parallel()

	// A subscription feed is stateless and re-rendered on every poll, so the
	// exported RRULE must carry the series' REAL horizon (open-ended here), never
	// a moving "today + N" UNTIL: advancing that window without bumping SEQUENCE
	// would leave clients frozen on the first horizon they saw, dropping later
	// occurrences. DTSTART is still anchored to the first real occurrence.
	oldMonday := appointmentcap.NewDate(2020, 1, 6)
	tenantID := ptrtest.NewTenantID()
	appt := &appointmentcap.Appointment{
		ID:        9,
		TenantID:  tenantID,
		Title:     "Wöchentlich",
		StartDate: toCalendarDate(oldMonday),
		EndDate:   toCalendarDate(oldMonday),
		StartTime: helperClock(9, 0),
		EndTime:   helperClock(10, 0),
	}
	rule := &appointmentcap.RecurrenceRule{
		Frequency:     appointmentcap.RecurrenceFrequencyWeekly,
		IntervalCount: 1,
		Weekdays:      []string{"monday"},
	}

	event := appointmentICSEvent(appt, rule, nil)
	require.NotNil(t, event.Recurrence)
	assert.Equal(t, oldMonday.String(), event.StartDate)
	assert.Empty(t, event.Recurrence.Until, "open-ended series must export no UNTIL")
	assert.Nil(t, event.Recurrence.Count)

	// A real EndsOn is preserved verbatim (a stable horizon, safe to export).
	endsOn := appointmentcap.NewDate(2026, 3, 30)
	bounded := &appointmentcap.RecurrenceRule{
		Frequency:     appointmentcap.RecurrenceFrequencyWeekly,
		IntervalCount: 1,
		Weekdays:      []string{"monday"},
		EndsOn:        &endsOn,
	}
	boundedEvent := appointmentICSEvent(appt, bounded, nil)
	require.NotNil(t, boundedEvent.Recurrence)
	assert.Equal(t, endsOn.String(), boundedEvent.Recurrence.Until)
}

func TestCalendarGroupingAndDisplayHelpers(t *testing.T) {
	t.Parallel()

	firstTenantID := ptrtest.NewTenantID()
	secondTenantID := ptrtest.NewTenantID()
	children := []*ports.ChildSummary{
		{TenantID: secondTenantID, GuardianProfileID: 20},
		{TenantID: firstTenantID, GuardianProfileID: 10},
		{TenantID: firstTenantID, GuardianProfileID: 10},
	}

	grouped := groupChildrenByTenant(children)
	require.Len(t, grouped[firstTenantID], 2)
	require.Len(t, grouped[secondTenantID], 1)
	assert.ElementsMatch(t, []int64{20, 10}, distinctGuardianProfileIDs(children))
	assert.Equal(t,
		map[int64]struct{}{firstTenantID: {}, secondTenantID: {}},
		int64Set([]int64{firstTenantID, secondTenantID, firstTenantID}),
	)

	staff := &ports.Staff{Person: &ports.Person{FirstName: " Ada ", LastName: " Lovelace "}}
	assert.Equal(t, "Ada   Lovelace", staffName(staff))
	assert.Empty(t, staffName(nil))

	email := "parent@example.test"
	assert.Equal(t, "parent@example.test", guardianName(&ports.GuardianProfile{Email: &email}))
	assert.Empty(t, guardianName(nil))

	assert.Equal(t, "Kind 44", studentDisplayName(&ports.Student{ID: 44}))
	assert.Equal(t, "Grace Hopper", studentDisplayName(&ports.Student{Person: &ports.Person{FirstName: "Grace", LastName: "Hopper"}}))
}

func TestCalendarSortEvents(t *testing.T) {
	t.Parallel()

	events := []Event{
		{ID: "c", StartDate: "2026-01-02", StartTime: "08:00"},
		{ID: "b", StartDate: "2026-01-01", StartTime: "10:00"},
		{ID: "a", StartDate: "2026-01-01", StartTime: "10:00"},
		{ID: "d", StartDate: "2026-01-01", StartTime: "09:00"},
	}

	sortEvents(events)

	assert.Equal(t, []string{"d", "a", "b", "c"}, []string{events[0].ID, events[1].ID, events[2].ID, events[3].ID})
}

func TestCalendarServiceValidatesInputsBeforeDependencies(t *testing.T) {
	t.Parallel()

	svc := &service{}
	start := appointmentcap.NewDate(2026, 1, 5)
	ctx := context.TODO()

	err := validateWindow(appointmentcap.Date(""), start)
	assert.True(t, errors.Is(err, ErrInvalidRequest))
	assert.Contains(t, err.Error(), "from and to are required")

	err = validateWindow(start, start.AddDays(-1))
	assert.True(t, errors.Is(err, ErrInvalidRequest))
	assert.Contains(t, err.Error(), "to must be on or after from")

	err = validateWindow(start, start.AddDays(maxCalendarWindowDays))
	assert.True(t, errors.Is(err, ErrInvalidRequest))
	assert.Contains(t, err.Error(), "range cannot exceed")

	assert.NoError(t, validateWindow(start, start.AddDays(maxCalendarWindowDays-1)))

	_, err = svc.ListMyStaffEvents(ctx, start, start.AddDays(-1))
	assert.True(t, errors.Is(err, ErrInvalidRequest))

	_, err = svc.ListMyParentEvents(ctx, 0, start, start)
	assert.True(t, errors.Is(err, ErrForbidden))

	_, err = svc.GetStaffAppointmentOverview(ctx, 0, "")
	assert.True(t, errors.Is(err, ErrInvalidRequest))

	_, err = svc.GetParentAppointmentOverview(ctx, 0, 1, "")
	assert.True(t, errors.Is(err, ErrForbidden))

	_, err = svc.GetParentAppointmentOverview(ctx, 1, 0, "")
	assert.True(t, errors.Is(err, ErrInvalidRequest))
}

func TestTimetableRoomIDPrefersAssignmentOverride(t *testing.T) {
	t.Parallel()

	instance := &ports.ActivityInstance{RoomID: 70}
	override := int64(90)

	assert.Equal(t, int64(70), timetableRoomID(instance, &ports.InstanceStaff{}))
	assert.Equal(t, int64(90), timetableRoomID(instance, &ports.InstanceStaff{RoomID: &override}))
	assert.Equal(t, int64(70), timetableRoomID(instance, nil))
	assert.Equal(t, int64(0), timetableRoomID(nil, nil))
}

// The recipient picker finds "Özdemir" for "ozdem" (#4949).
func TestMatchesRecipientQueryIgnoresAccents(t *testing.T) {
	t.Parallel()
	assert.True(t, matchesRecipientQuery("Zoë Özdemir", normalizeRecipientQuery(" Zoe OZDEM ")))
	assert.False(t, matchesRecipientQuery("Kim Becker", normalizeRecipientQuery("ozdem")))
	assert.True(t, matchesRecipientQuery("Kim Becker", normalizeRecipientQuery("  ")))
}

type searchCapturingGuardians struct {
	ports.GuardianDirectory
	queries []string
}

func (g *searchCapturingGuardians) SearchByText(_ context.Context, query string, _ int) ([]*ports.GuardianProfile, error) {
	g.queries = append(g.queries, query)
	return nil, nil
}

func (g *searchCapturingGuardians) FindActivePortalProfilesByIDs(context.Context, []int64) (map[int64]*ports.GuardianProfile, error) {
	return map[int64]*ports.GuardianProfile{}, nil
}

// The database search folds on its own (f_unaccent); a query folded in Go
// first misses e-mail addresses with umlauts and letters Go folds but
// PostgreSQL keeps, such as й (#5791).
func TestParentRecipientSearchSendsTheTypedQuery(t *testing.T) {
	t.Parallel()

	for _, typed := range []string{"  jörg@schule.de ", "Андрей"} {
		guardians := &searchCapturingGuardians{}
		svc := &service{cfg: Config{GuardianProfileRepo: guardians}}
		_, err := svc.parentRecipientOptions(context.TODO(), recipientQueryFor(typed), 5)
		require.NoError(t, err)
		assert.Equal(t, []string{strings.TrimSpace(typed)}, guardians.queries)
	}
}

func TestParentOverviewListsStaffAndOwnEntriesButCountsEveryone(t *testing.T) {
	t.Parallel()

	ownProfile, otherProfile := int64(11), int64(12)
	recipients := []*appointmentcap.AppointmentRecipient{
		{ID: 1, RecipientType: appointmentcap.RecipientTypeStaff, StaffID: ptrtest.Ptr(int64(5)), Status: appointmentcap.ResponseStatusAccepted},
		{ID: 2, RecipientType: appointmentcap.RecipientTypeGuardianProfile, GuardianProfileID: &ownProfile, StudentIDs: []int64{100}, Status: appointmentcap.ResponseStatusPending},
		{ID: 3, RecipientType: appointmentcap.RecipientTypeGuardianProfile, GuardianProfileID: &otherProfile, StudentIDs: []int64{200}, Status: appointmentcap.ResponseStatusDeclined},
		// Own profile, but through a child this parent cannot see here.
		{ID: 4, RecipientType: appointmentcap.RecipientTypeGuardianProfile, GuardianProfileID: &ownProfile, StudentIDs: []int64{300}, Status: appointmentcap.ResponseStatusAccepted},
		{ID: 5, RecipientType: appointmentcap.RecipientTypeGuardianProfile, GuardianProfileID: &otherProfile, StudentIDs: []int64{200}, Status: appointmentcap.ResponseStatusInfo},
	}

	listed := parentListedRecipients(recipients, []int64{ownProfile}, []int64{100})
	ids := make([]int64, 0, len(listed))
	for _, recipient := range listed {
		ids = append(ids, recipient.ID)
	}
	assert.Equal(t, []int64{1, 2}, ids)

	assert.Equal(t, AppointmentResponseCounts{Accepted: 2, Declined: 1, Pending: 1}, countResponses(recipients, ""))

	// One date of the series counts the answers given for it (#6987).
	date := appointmentcap.NewDate(2026, 11, 25)
	recipients[0].OccurrenceResponses = map[appointmentcap.Date]string{date: appointmentcap.ResponseStatusDeclined}
	recipients[1].OccurrenceResponses = map[appointmentcap.Date]string{date: appointmentcap.ResponseStatusAccepted}
	assert.Equal(t, AppointmentResponseCounts{Accepted: 2, Declined: 2, Pending: 0}, countResponses(recipients, date))
	assert.Equal(t, AppointmentResponseCounts{Accepted: 2, Declined: 1, Pending: 1}, countResponses(recipients, date.AddDays(7)), "a date without own answers counts the series answers")
}
