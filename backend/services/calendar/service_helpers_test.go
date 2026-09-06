package calendar

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func helperClock(hour, minute int) time.Time {
	return timezone.NormalizeWallClock(time.Date(2026, 1, 1, hour, minute, 0, 0, time.UTC))
}

func helperAppointment() *calModels.Appointment {
	return &calModels.Appointment{
		Model:              calModels.Model{ID: 42},
		OrganizerStaffID:   7,
		Title:              "Planning",
		StartDate:          calModels.NewDate(2026, 1, 5),
		EndDate:            calModels.NewDate(2026, 1, 5),
		StartTime:          helperClock(9, 0),
		EndTime:            helperClock(10, 0),
		DeliveryMode:       calModels.DeliveryModeRSVPRequired,
		OverviewVisibility: calModels.OverviewVisibilityOrganizer,
	}
}

func TestCalendarOverviewVisibilityHelpers(t *testing.T) {
	t.Parallel()

	appointment := helperAppointment()

	assert.False(t, canStaffViewOverview(nil, 7, true))
	assert.True(t, canStaffViewOverview(appointment, 7, false))
	assert.False(t, canStaffViewOverview(appointment, 8, true))

	appointment.OverviewVisibility = calModels.OverviewVisibilityStaff
	assert.True(t, canStaffViewOverview(appointment, 8, true))
	assert.False(t, canParentViewOverview(appointment, true))

	appointment.OverviewVisibility = calModels.OverviewVisibilityAll
	assert.True(t, canStaffViewOverview(appointment, 8, true))
	assert.True(t, canParentViewOverview(appointment, true))
	assert.False(t, canParentViewOverview(appointment, false))
}

func TestAppointmentEventAndOverride(t *testing.T) {
	t.Parallel()

	appointment := helperAppointment()
	appointment.OverviewVisibility = calModels.OverviewVisibilityAll
	status := calModels.ResponseStatusPending
	recipientID := int64(99)

	event := appointmentEvent(appointment, timezone.NewDate(2026, 1, 6), &status, &recipientID, 8)

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
	startDate := calModels.NewDate(2026, 1, 7)
	endDate := calModels.NewDate(2026, 1, 8)
	startTime := helperClock(11, 30)
	endTime := helperClock(12, 45)
	allDay := true
	applyOverride(&event, &calModels.AppointmentOccurrenceOverride{
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
	day := timezone.NewDate(2026, 1, 6)

	// A recipient can respond for any non-informational status, including an
	// already accepted/declined one — the respond endpoints allow changing it.
	for _, status := range []string{
		calModels.ResponseStatusPending,
		calModels.ResponseStatusAccepted,
		calModels.ResponseStatusDeclined,
	} {
		s := status
		event := appointmentEvent(appointment, day, &s, &recipientID, 8)
		assert.Truef(t, event.CanRespond, "status %q should be respondable", status)
	}

	// Informational recipients cannot respond.
	info := calModels.ResponseStatusInfo
	infoEvent := appointmentEvent(appointment, day, &info, &recipientID, 8)
	assert.False(t, infoEvent.CanRespond)

	// A non-recipient (nil recipient/status) cannot respond.
	nonRecipient := appointmentEvent(appointment, day, nil, nil, 8)
	assert.False(t, nonRecipient.CanRespond)
}

func TestStaffRecipientStatus(t *testing.T) {
	t.Parallel()

	staffID := int64(10)
	recipients := []*calModels.AppointmentRecipient{
		{ID: 101, RecipientType: calModels.RecipientTypeStaff, StaffID: &staffID, Status: calModels.ResponseStatusAccepted},
	}

	status, recipientID := staffRecipientStatus(recipients, staffID)
	require.NotNil(t, status)
	require.NotNil(t, recipientID)
	assert.Equal(t, calModels.ResponseStatusAccepted, *status)
	assert.Equal(t, int64(101), *recipientID)

	status, recipientID = staffRecipientStatus(recipients, 99)
	assert.Nil(t, status)
	assert.Nil(t, recipientID)
}

func TestAppointmentICSEventUIDIncludesTenant(t *testing.T) {
	t.Parallel()

	// The parent feed aggregates appointments across schools, so the UID must be
	// globally unique — appointment IDs repeat per tenant.
	appt := &calModels.Appointment{
		Model:       calModels.Model{ID: 5},
		TenantModel: calModels.TenantModel{TenantID: 42},
		Title:       "Termin",
		StartDate:   calModels.NewDate(2026, 1, 5),
		EndDate:     calModels.NewDate(2026, 1, 5),
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
	oldMonday := timezone.NewDate(2020, 1, 6)
	tenantID := ptrtest.NewTenantID()
	appt := &calModels.Appointment{
		Model:       calModels.Model{ID: 9},
		TenantModel: calModels.TenantModel{TenantID: tenantID},
		Title:       "Wöchentlich",
		StartDate:   toCalendarDate(oldMonday),
		EndDate:     toCalendarDate(oldMonday),
		StartTime:   helperClock(9, 0),
		EndTime:     helperClock(10, 0),
	}
	rule := &calModels.RecurrenceRule{
		Frequency:     calModels.RecurrenceFrequencyWeekly,
		IntervalCount: 1,
		Weekdays:      []string{"monday"},
	}

	event := appointmentICSEvent(appt, rule, nil)
	require.NotNil(t, event.Recurrence)
	assert.Equal(t, oldMonday.String(), event.StartDate)
	assert.Empty(t, event.Recurrence.Until, "open-ended series must export no UNTIL")
	assert.Nil(t, event.Recurrence.Count)

	// A real EndsOn is preserved verbatim (a stable horizon, safe to export).
	endsOn := calModels.NewDate(2026, 3, 30)
	bounded := &calModels.RecurrenceRule{
		Frequency:     calModels.RecurrenceFrequencyWeekly,
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
	children := []*parentModels.ChildSummary{
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

	staff := &userModels.Staff{Person: &userModels.Person{FirstName: " Ada ", LastName: " Lovelace "}}
	assert.Equal(t, "Ada   Lovelace", staffName(staff))
	assert.Empty(t, staffName(nil))

	email := "parent@example.test"
	assert.Equal(t, "parent@example.test", guardianName(&userModels.GuardianProfile{Email: &email}))
	assert.Empty(t, guardianName(nil))

	assert.Equal(t, "Kind 44", studentDisplayName(&userModels.Student{Model: base.Model{ID: 44}}))
	assert.Equal(t, "Grace Hopper", studentDisplayName(&userModels.Student{Person: &userModels.Person{FirstName: "Grace", LastName: "Hopper"}}))
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
	start := timezone.NewDate(2026, 1, 5)
	ctx := context.TODO()

	err := validateWindow(timezone.Date(""), start)
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

	_, err = svc.GetStaffAppointmentOverview(ctx, 0)
	assert.True(t, errors.Is(err, ErrInvalidRequest))

	_, err = svc.GetParentAppointmentOverview(ctx, 0, 1)
	assert.True(t, errors.Is(err, ErrForbidden))

	_, err = svc.GetParentAppointmentOverview(ctx, 1, 0)
	assert.True(t, errors.Is(err, ErrInvalidRequest))
}

func TestTimetableRoomIDPrefersAssignmentOverride(t *testing.T) {
	t.Parallel()

	instance := &scheduleModels.ActivityInstance{RoomID: 70}
	override := int64(90)

	assert.Equal(t, int64(70), timetableRoomID(instance, &scheduleModels.InstanceStaff{}))
	assert.Equal(t, int64(90), timetableRoomID(instance, &scheduleModels.InstanceStaff{RoomID: &override}))
	assert.Equal(t, int64(70), timetableRoomID(instance, nil))
	assert.Equal(t, int64(0), timetableRoomID(nil, nil))
}
