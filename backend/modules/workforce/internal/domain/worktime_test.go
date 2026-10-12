package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validFields returns a minimally valid work-time template (no entries).
func validFields(entries ...WorkTimeModelEntryFields) WorkTimeModelFields {
	return WorkTimeModelFields{
		Name:               "Validation test",
		RotationLength:     1,
		RotationAnchorDate: "2026-06-01",
		Entries:            entries,
	}
}

func TestValidateWorkTimeModelFields_RejectsInvalidEntries(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		entry WorkTimeModelEntryFields
	}{
		{
			name:  "day outside week",
			entry: WorkTimeModelEntryFields{WeekIndex: 0, DayOfWeek: 7, TargetMinutes: 300},
		},
		{
			name:  "week outside rotation",
			entry: WorkTimeModelEntryFields{WeekIndex: 1, DayOfWeek: 0, TargetMinutes: 300},
		},
		{
			name:  "minutes too large",
			entry: WorkTimeModelEntryFields{WeekIndex: 0, DayOfWeek: 0, TargetMinutes: MaxDailyMinutes + 1},
		},
		{
			name:  "negative minutes",
			entry: WorkTimeModelEntryFields{WeekIndex: 0, DayOfWeek: 0, TargetMinutes: -1},
		},
		{
			name:  "start time is not a wall clock",
			entry: WorkTimeModelEntryFields{WeekIndex: 0, DayOfWeek: 0, TargetMinutes: 300, StartTime: "08:30"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateWorkTimeModelFields(validFields(test.entry))
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidWorkTime), "validation failures must classify as invalid input")
		})
	}
}

func TestValidateWorkTimeModelFields_RejectsDuplicateEntries(t *testing.T) {
	t.Parallel()

	err := ValidateWorkTimeModelFields(validFields(
		WorkTimeModelEntryFields{WeekIndex: 0, DayOfWeek: 0, TargetMinutes: 300},
		WorkTimeModelEntryFields{WeekIndex: 0, DayOfWeek: 0, TargetMinutes: 240},
	))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestValidateWorkTimeModelFields_RejectsInvalidTemplateMetadata(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		mutate func(*WorkTimeModelFields)
	}{
		{name: "missing name", mutate: func(f *WorkTimeModelFields) { f.Name = "" }},
		{name: "rotation below one", mutate: func(f *WorkTimeModelFields) { f.RotationLength = 0 }},
		{name: "rotation above cap", mutate: func(f *WorkTimeModelFields) { f.RotationLength = MaxRotationWeeks + 1 }},
		{name: "missing anchor", mutate: func(f *WorkTimeModelFields) { f.RotationAnchorDate = "" }},
		{name: "anchor is not a calendar day", mutate: func(f *WorkTimeModelFields) { f.RotationAnchorDate = "2026-06-31" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fields := validFields()
			test.mutate(&fields)
			require.ErrorIs(t, ValidateWorkTimeModelFields(fields), ErrInvalidWorkTime)
		})
	}
}

func TestValidateStaffScheduleFields_RejectsRotationInconsistency(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		fields StaffWorkScheduleFields
	}{
		{
			name:   "week index at rotation length",
			fields: StaffWorkScheduleFields{WeekIndex: 1, RotationLength: 1, DayOfWeek: 0, TargetMinutes: 240},
		},
		{
			name:   "day outside week",
			fields: StaffWorkScheduleFields{WeekIndex: 0, RotationLength: 1, DayOfWeek: 9, TargetMinutes: 240},
		},
		{
			name:   "minutes above the daily cap",
			fields: StaffWorkScheduleFields{WeekIndex: 0, RotationLength: 1, DayOfWeek: 0, TargetMinutes: MaxDailyMinutes + 1},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, ValidateStaffScheduleFields(test.fields), ErrInvalidWorkTime)
		})
	}
}

func TestValidateStaffScheduleFields_AcceptsRotationalVersion(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateStaffScheduleFields(StaffWorkScheduleFields{
		WeekIndex: 1, RotationLength: 2, DayOfWeek: 4, TargetMinutes: 480, StartTime: "08:30:00",
	}))
}

func TestScheduleVersionStart(t *testing.T) {
	t.Parallel()

	const today = "2026-10-08"
	closedSeptember := []*StaffMonthBalanceSnapshot{{Year: 2026, Month: 9}}
	reopenedAt := time.Date(2026, time.October, 2, 9, 0, 0, 0, time.UTC)
	reopenedSeptember := []*StaffMonthBalanceSnapshot{{Year: 2026, Month: 9, ReopenedAt: &reopenedAt}}

	for _, test := range []struct {
		name       string
		validFrom  string
		hasHistory bool
		closed     []*StaffMonthBalanceSnapshot
		want       string
		wantErr    string
	}{
		{name: "empty starts today", hasHistory: true, want: today},
		{name: "today with history", validFrom: today, hasHistory: true, want: today},
		{name: "first schedule backdated", validFrom: "2026-07-10", want: "2026-07-10"},
		{name: "backdated before a closed month", validFrom: "2026-10-01", closed: closedSeptember, want: "2026-10-01"},
		{name: "reopened month is open", validFrom: "2026-09-01", closed: reopenedSeptember, want: "2026-09-01"},
		{name: "backdated with history", validFrom: "2026-07-10", hasHistory: true, wantErr: "first schedule"},
		{name: "into a closed month", validFrom: "2026-07-10", closed: closedSeptember, wantErr: "closed month 2026-09"},
		{name: "future", validFrom: "2026-10-09", wantErr: "after today"},
		{name: "malformed", validFrom: "2026-7-10", wantErr: "valid_from must be"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := ScheduleVersionStart(test.validFrom, today, test.hasHistory, test.closed)
			if test.wantErr != "" {
				require.ErrorIs(t, err, ErrInvalidWorkTime)
				assert.Contains(t, err.Error(), test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}
