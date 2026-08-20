package calendar

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClock(hour, minute int) time.Time {
	return timezone.WallClock(time.Date(2026, 1, 1, hour, minute, 0, 0, time.UTC))
}

func validAppointment() *Appointment {
	return &Appointment{
		OrganizerStaffID: 1,
		Title:            "  Elternabend  ",
		StartDate:        timezone.NewDate(2026, 1, 5),
		EndDate:          timezone.NewDate(2026, 1, 5),
		StartTime:        testClock(9, 0),
		EndTime:          testClock(10, 0),
		DeliveryMode:     DeliveryModeRSVPRequired,
	}
}

func TestAppointmentValidateDefaultsAndTrims(t *testing.T) {
	t.Parallel()

	appointment := validAppointment()

	require.NoError(t, appointment.Validate())

	assert.Equal(t, "Elternabend", appointment.Title)
	assert.Equal(t, OverviewVisibilityOrganizer, appointment.OverviewVisibility)
}

func TestAppointmentValidateRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Appointment)
		message string
	}{
		{name: "organizer required", mutate: func(a *Appointment) { a.OrganizerStaffID = 0 }, message: "organizer_staff_id is required"},
		{name: "title required", mutate: func(a *Appointment) { a.Title = "  " }, message: "title is required"},
		{name: "start date required", mutate: func(a *Appointment) { a.StartDate = timezone.Date{} }, message: "start_date is required"},
		{name: "end date required", mutate: func(a *Appointment) { a.EndDate = timezone.Date{} }, message: "end_date is required"},
		{name: "date order", mutate: func(a *Appointment) { a.EndDate = a.StartDate.AddDays(-1) }, message: "end_date must be on or after start_date"},
		{name: "same day time order", mutate: func(a *Appointment) { a.EndTime = a.StartTime }, message: "end_time must be after start_time on same-day appointments"},
		{name: "delivery mode", mutate: func(a *Appointment) { a.DeliveryMode = "email" }, message: "delivery_mode must be rsvp_required or informational"},
		{name: "overview visibility", mutate: func(a *Appointment) { a.OverviewVisibility = "parents" }, message: "overview_visibility must be organizer, staff, or all"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appointment := validAppointment()
			tt.mutate(appointment)

			require.EqualError(t, appointment.Validate(), tt.message)
		})
	}
}

func TestAppointmentValidateAllowsAllDayAndMultiDayTimeOrder(t *testing.T) {
	t.Parallel()

	allDay := validAppointment()
	allDay.AllDay = true
	allDay.EndTime = allDay.StartTime
	require.NoError(t, allDay.Validate())

	multiDay := validAppointment()
	multiDay.EndDate = multiDay.StartDate.AddDays(1)
	multiDay.EndTime = testClock(8, 0)
	require.NoError(t, multiDay.Validate())
}

func intPtr(n int) *int { return &n }

func TestRecurrenceRuleValidate(t *testing.T) {
	t.Parallel()

	count := 2
	endsOn := timezone.NewDate(2026, 2, 1)

	tests := []struct {
		name    string
		rule    RecurrenceRule
		wantErr string
	}{
		{name: "valid", rule: RecurrenceRule{AppointmentID: 1, Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1}},
		{name: "missing appointment", rule: RecurrenceRule{Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1}, wantErr: "appointment_id is required"},
		{name: "invalid frequency", rule: RecurrenceRule{AppointmentID: 1, Frequency: "hourly", IntervalCount: 1}, wantErr: "invalid recurrence frequency"},
		{name: "invalid interval", rule: RecurrenceRule{AppointmentID: 1, Frequency: RecurrenceFrequencyDaily}, wantErr: "interval_count must be positive"},
		{name: "two end modes", rule: RecurrenceRule{AppointmentID: 1, Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, EndsOn: &endsOn, OccurrenceCount: &count}, wantErr: "only one recurrence end mode is allowed"},
		{name: "invalid count", rule: RecurrenceRule{AppointmentID: 1, Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, OccurrenceCount: new(int)}, wantErr: "occurrence_count must be positive"},
		{name: "count at max is allowed", rule: RecurrenceRule{AppointmentID: 1, Frequency: RecurrenceFrequencyDaily, IntervalCount: 1, OccurrenceCount: intPtr(MaxRecurrenceOccurrenceCount)}},
		{name: "count over max", rule: RecurrenceRule{AppointmentID: 1, Frequency: RecurrenceFrequencyDaily, IntervalCount: 1, OccurrenceCount: intPtr(MaxRecurrenceOccurrenceCount + 1)}, wantErr: "occurrence_count exceeds the maximum of 366"},
		{name: "valid weekdays", rule: RecurrenceRule{AppointmentID: 1, Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday", "Friday"}}},
		{name: "invalid weekday", rule: RecurrenceRule{AppointmentID: 1, Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"foo"}}, wantErr: "weekdays must be valid day names (monday–sunday)"},
		{name: "valid month days", rule: RecurrenceRule{AppointmentID: 1, Frequency: RecurrenceFrequencyMonthly, IntervalCount: 1, MonthDays: []int{1, 15, 31}}},
		{name: "month day zero", rule: RecurrenceRule{AppointmentID: 1, Frequency: RecurrenceFrequencyMonthly, IntervalCount: 1, MonthDays: []int{0}}, wantErr: "month_days must be between 1 and 31"},
		{name: "month day too large", rule: RecurrenceRule{AppointmentID: 1, Frequency: RecurrenceFrequencyMonthly, IntervalCount: 1, MonthDays: []int{32}}, wantErr: "month_days must be between 1 and 31"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestRecurrenceRuleValidateDeduplicates(t *testing.T) {
	t.Parallel()

	// Duplicate (and mixed-case) weekdays normalise to a single lowercase entry —
	// otherwise the occurrence expansion emits the date twice and exhausts
	// occurrence_count early, and the RRULE exports an invalid BYDAY=MO,MO.
	weekly := RecurrenceRule{
		AppointmentID: 1,
		Frequency:     RecurrenceFrequencyWeekly,
		IntervalCount: 1,
		Weekdays:      []string{"Monday", "monday", " MONDAY ", "wednesday"},
	}
	require.NoError(t, weekly.Validate())
	require.Equal(t, []string{"monday", "wednesday"}, weekly.Weekdays)

	monthly := RecurrenceRule{
		AppointmentID: 1,
		Frequency:     RecurrenceFrequencyMonthly,
		IntervalCount: 1,
		MonthDays:     []int{5, 5, 20, 5},
	}
	require.NoError(t, monthly.Validate())
	require.Equal(t, []int{5, 20}, monthly.MonthDays)
}

func TestAppointmentRecipientValidate(t *testing.T) {
	t.Parallel()

	staffID := int64(11)
	guardianID := int64(12)

	tests := []struct {
		name      string
		recipient AppointmentRecipient
		wantErr   string
	}{
		{name: "staff valid", recipient: AppointmentRecipient{AppointmentID: 1, RecipientType: RecipientTypeStaff, StaffID: &staffID, Status: ResponseStatusPending}},
		{name: "guardian valid", recipient: AppointmentRecipient{AppointmentID: 1, RecipientType: RecipientTypeGuardianProfile, GuardianProfileID: &guardianID, Status: ResponseStatusInfo}},
		{name: "appointment required", recipient: AppointmentRecipient{RecipientType: RecipientTypeStaff, StaffID: &staffID, Status: ResponseStatusPending}, wantErr: "appointment_id is required"},
		{name: "staff exact subject", recipient: AppointmentRecipient{AppointmentID: 1, RecipientType: RecipientTypeStaff, StaffID: &staffID, GuardianProfileID: &guardianID, Status: ResponseStatusPending}, wantErr: "staff recipient requires staff_id only"},
		{name: "guardian exact subject", recipient: AppointmentRecipient{AppointmentID: 1, RecipientType: RecipientTypeGuardianProfile, StaffID: &staffID, GuardianProfileID: &guardianID, Status: ResponseStatusPending}, wantErr: "guardian recipient requires guardian_profile_id only"},
		{name: "invalid type", recipient: AppointmentRecipient{AppointmentID: 1, RecipientType: "class", Status: ResponseStatusPending}, wantErr: "invalid recipient_type"},
		{name: "invalid status", recipient: AppointmentRecipient{AppointmentID: 1, RecipientType: RecipientTypeStaff, StaffID: &staffID, Status: "maybe"}, wantErr: "invalid recipient status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.recipient.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestCalendarModelAccessors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	appointment := &Appointment{}
	appointment.ID = 10
	appointment.CreatedAt = now
	appointment.UpdatedAt = now.Add(time.Hour)
	assert.Equal(t, int64(10), appointment.GetID())
	assert.Equal(t, now, appointment.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), appointment.GetUpdatedAt())

	recurrence := &RecurrenceRule{}
	recurrence.ID = 11
	recurrence.CreatedAt = now
	recurrence.UpdatedAt = now
	assert.Equal(t, int64(11), recurrence.GetID())
	assert.Equal(t, now, recurrence.GetCreatedAt())
	assert.Equal(t, now, recurrence.GetUpdatedAt())

	recipient := &AppointmentRecipient{}
	recipient.ID = 12
	recipient.CreatedAt = now
	recipient.UpdatedAt = now
	assert.Equal(t, int64(12), recipient.GetID())
	assert.Equal(t, now, recipient.GetCreatedAt())
	assert.Equal(t, now, recipient.GetUpdatedAt())

	link := &AppointmentRecipientStudent{}
	link.ID = 13
	link.CreatedAt = now
	link.UpdatedAt = now
	assert.Equal(t, int64(13), link.GetID())
	assert.Equal(t, now, link.GetCreatedAt())
	assert.Equal(t, now, link.GetUpdatedAt())

	target := &AppointmentTarget{}
	target.ID = 14
	target.CreatedAt = now
	target.UpdatedAt = now
	assert.Equal(t, int64(14), target.GetID())
	assert.Equal(t, now, target.GetCreatedAt())
	assert.Equal(t, now, target.GetUpdatedAt())

	override := &AppointmentOccurrenceOverride{}
	override.ID = 15
	override.CreatedAt = now
	override.UpdatedAt = now
	assert.Equal(t, int64(15), override.GetID())
	assert.Equal(t, now, override.GetCreatedAt())
	assert.Equal(t, now, override.GetUpdatedAt())
}
