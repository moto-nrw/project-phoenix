package calendar

import (
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func helperClock(hour, minute int) time.Time {
	return timezone.WallClock(time.Date(2026, 1, 1, hour, minute, 0, 0, time.UTC))
}

func helperAppointment() *calModels.Appointment {
	return &calModels.Appointment{
		Model:              base.Model{ID: 42},
		OrganizerStaffID:   7,
		Title:              "Planning",
		StartDate:          timezone.NewDate(2026, 1, 5),
		EndDate:            timezone.NewDate(2026, 1, 5),
		StartTime:          helperClock(9, 0),
		EndTime:            helperClock(10, 0),
		DeliveryMode:       calModels.DeliveryModeRSVPRequired,
		OverviewVisibility: calModels.OverviewVisibilityOrganizer,
	}
}

func TestCalendarOverviewVisibilityHelpers(t *testing.T) {
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
	startDate := timezone.NewDate(2026, 1, 7)
	endDate := timezone.NewDate(2026, 1, 8)
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

func TestCalendarRecipientStatusHelpers(t *testing.T) {
	staffID := int64(10)
	guardianID := int64(20)
	recipients := []*calModels.AppointmentRecipient{
		{Model: base.Model{ID: 101}, RecipientType: calModels.RecipientTypeStaff, StaffID: &staffID, Status: calModels.ResponseStatusAccepted},
		{Model: base.Model{ID: 102}, RecipientType: calModels.RecipientTypeGuardianProfile, GuardianProfileID: &guardianID, Status: calModels.ResponseStatusDeclined},
	}

	status, recipientID := staffRecipientStatus(recipients, staffID)
	require.NotNil(t, status)
	require.NotNil(t, recipientID)
	assert.Equal(t, calModels.ResponseStatusAccepted, *status)
	assert.Equal(t, int64(101), *recipientID)

	status, recipientID = guardianRecipientStatus(recipients, []int64{99, guardianID})
	require.NotNil(t, status)
	require.NotNil(t, recipientID)
	assert.Equal(t, calModels.ResponseStatusDeclined, *status)
	assert.Equal(t, int64(102), *recipientID)

	status, recipientID = staffRecipientStatus(recipients, 99)
	assert.Nil(t, status)
	assert.Nil(t, recipientID)
}

func TestCalendarRecurrenceExpansion(t *testing.T) {
	appointment := helperAppointment()
	endsOn := timezone.NewDate(2026, 1, 31)
	count := 3

	weekly := &calModels.RecurrenceRule{
		Frequency:     calModels.RecurrenceFrequencyWeekly,
		IntervalCount: 1,
		Weekdays:      []string{" monday ", "WEDNESDAY"},
		EndsOn:        &endsOn,
	}
	assert.Equal(t, []timezone.Date{
		timezone.NewDate(2026, 1, 5),
		timezone.NewDate(2026, 1, 7),
		timezone.NewDate(2026, 1, 12),
	}, expandOccurrences(appointment, weekly, timezone.NewDate(2026, 1, 5), timezone.NewDate(2026, 1, 12)))

	monthly := &calModels.RecurrenceRule{
		Frequency:       calModels.RecurrenceFrequencyMonthly,
		IntervalCount:   1,
		MonthDays:       []int{5, 20},
		OccurrenceCount: &count,
	}
	assert.Equal(t, []timezone.Date{
		timezone.NewDate(2026, 1, 5),
		timezone.NewDate(2026, 1, 20),
		timezone.NewDate(2026, 2, 5),
	}, expandOccurrences(appointment, monthly, timezone.NewDate(2026, 1, 1), timezone.NewDate(2026, 3, 31)))

	yearly := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyYearly, IntervalCount: 1}
	daily := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyDaily, IntervalCount: 2}
	weeklyDefault := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: 1}
	monthlyDefault := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyMonthly, IntervalCount: 1}

	assert.True(t, matchesRule(appointment.StartDate, timezone.NewDate(2026, 1, 7), daily))
	assert.False(t, matchesRule(appointment.StartDate, timezone.NewDate(2026, 1, 6), daily))
	assert.True(t, matchesRule(appointment.StartDate, timezone.NewDate(2026, 1, 12), weeklyDefault))
	assert.True(t, matchesRule(appointment.StartDate, timezone.NewDate(2026, 2, 5), monthlyDefault))
	assert.True(t, matchesRule(appointment.StartDate, timezone.NewDate(2027, 1, 5), yearly))
	assert.False(t, matchesRule(appointment.StartDate, timezone.NewDate(2027, 1, 6), yearly))
	assert.False(t, matchesRule(appointment.StartDate, timezone.NewDate(2026, 1, 4), yearly))
	assert.False(t, matchesRule(appointment.StartDate, timezone.NewDate(2026, 1, 5), &calModels.RecurrenceRule{Frequency: "never", IntervalCount: 1}))
}

func TestCalendarGroupingAndDisplayHelpers(t *testing.T) {
	children := []*parentModels.ChildSummary{
		{TenantID: 2, GuardianProfileID: 20},
		{TenantID: 1, GuardianProfileID: 10},
		{TenantID: 1, GuardianProfileID: 10},
	}

	grouped := groupChildrenByTenant(children)
	require.Len(t, grouped[1], 2)
	require.Len(t, grouped[2], 1)
	assert.ElementsMatch(t, []int64{20, 10}, distinctGuardianProfileIDs(children))
	assert.Equal(t, map[int64]struct{}{1: {}, 2: {}}, int64Set([]int64{1, 2, 1}))

	staff := &userModels.Staff{Person: &userModels.Person{FirstName: " Ada ", LastName: " Lovelace "}}
	assert.Equal(t, "Ada   Lovelace", staffName(staff))
	assert.Empty(t, staffName(nil))

	email := "parent@example.test"
	assert.Equal(t, "parent@example.test", guardianName(&userModels.GuardianProfile{Email: &email}))
	assert.Empty(t, guardianName(nil))

	assert.Equal(t, "Kind 44", studentDisplayName(&userModels.Student{Model: base.Model{ID: 44}}))
	assert.Equal(t, "Grace Hopper", studentDisplayName(&userModels.Student{Person: &userModels.Person{FirstName: "Grace", LastName: "Hopper"}}))

	staffID := int64(50)
	guardianID := int64(60)
	assert.Equal(t, "staff:50", recipientKey(calModels.RecipientTypeStaff, &staffID, nil))
	assert.Equal(t, "guardian:60", recipientKey(calModels.RecipientTypeGuardianProfile, nil, &guardianID))
	assert.Equal(t, "staff:0", recipientKey(calModels.RecipientTypeStaff, nil, nil))
}

func TestCalendarSortEvents(t *testing.T) {
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
	svc := &service{}
	start := timezone.NewDate(2026, 1, 5)

	err := validateWindow(timezone.Date{}, start)
	assert.True(t, errors.Is(err, ErrInvalidRequest))
	assert.Contains(t, err.Error(), "from and to are required")

	err = validateWindow(start, start.AddDays(-1))
	assert.True(t, errors.Is(err, ErrInvalidRequest))
	assert.Contains(t, err.Error(), "to must be on or after from")

	err = validateWindow(start, start.AddDays(maxCalendarWindowDays))
	assert.True(t, errors.Is(err, ErrInvalidRequest))
	assert.Contains(t, err.Error(), "range cannot exceed")

	assert.NoError(t, validateWindow(start, start.AddDays(maxCalendarWindowDays-1)))

	_, err = svc.ListMyStaffEvents(nil, start, start.AddDays(-1))
	assert.True(t, errors.Is(err, ErrInvalidRequest))

	_, err = svc.ListMyParentEvents(nil, 0, start, start)
	assert.True(t, errors.Is(err, ErrForbidden))

	_, err = svc.GetStaffAppointmentOverview(nil, 0)
	assert.True(t, errors.Is(err, ErrInvalidRequest))

	_, err = svc.GetParentAppointmentOverview(nil, 0, 1)
	assert.True(t, errors.Is(err, ErrForbidden))

	_, err = svc.GetParentAppointmentOverview(nil, 1, 0)
	assert.True(t, errors.Is(err, ErrInvalidRequest))
}
