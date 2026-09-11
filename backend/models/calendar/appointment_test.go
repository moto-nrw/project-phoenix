package calendar

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func intPtr(n int) *int { return &n }

func TestRecurrenceRuleValidate(t *testing.T) {
	t.Parallel()

	count := 2
	endsOn := NewDate(2026, 2, 1)

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
