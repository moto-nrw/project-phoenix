package schoolcalendar

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func conferenceFields() ParentConferenceFields {
	return ParentConferenceFields{
		OrganizerStaffID: 1, OrganizerAccountID: 2, SchoolClass: " 2a ", Date: calendar.NewDate(2026, 11, 12),
		StartMinute: 14 * 60, EndMinute: 16 * 60, SlotMinutes: 20,
	}
}

func TestParentConferenceSlotsSkipBreaksAndStopAtTheEnd(t *testing.T) {
	t.Parallel()
	fields := conferenceFields()
	assert.Equal(t, []int{840, 860, 880, 900, 920, 940}, ParentConferenceSlots(fields))

	fields.Breaks = []ParentConferenceBreak{{StartMinute: 15*60 + 10, EndMinute: 15*60 + 25}}
	assert.Equal(t, []int{840, 860, 880, 925}, ParentConferenceSlots(fields),
		"the slot overlapping the break moves to its end; the last one would end after 16:00")
	assert.True(t, HasParentConferenceSlot(fields, 925))
	assert.False(t, HasParentConferenceSlot(fields, 900))
}

func TestValidateParentConferenceFields(t *testing.T) {
	t.Parallel()
	blank := "  "
	fields := conferenceFields()
	fields.Location = &blank
	fields.Breaks = []ParentConferenceBreak{{StartMinute: 900, EndMinute: 910}, {StartMinute: 850, EndMinute: 860}}
	valid, err := ValidateParentConferenceFields(fields)
	require.NoError(t, err)
	assert.Equal(t, "2a", valid.SchoolClass)
	assert.Nil(t, valid.Location, "a blank room is no room")
	assert.Equal(t, 850, valid.Breaks[0].StartMinute, "breaks come back sorted")

	cases := map[string]func(*ParentConferenceFields){
		"end before start": func(f *ParentConferenceFields) { f.EndMinute = f.StartMinute },
		"slot too short":   func(f *ParentConferenceFields) { f.SlotMinutes = 4 },
		"slot too long":    func(f *ParentConferenceFields) { f.SlotMinutes = 121 },
		"no class":         func(f *ParentConferenceFields) { f.SchoolClass = " " },
		"no organizer":     func(f *ParentConferenceFields) { f.OrganizerStaffID = 0 },
		"break outside day": func(f *ParentConferenceFields) {
			f.Breaks = []ParentConferenceBreak{{StartMinute: 830, EndMinute: 850}}
		},
		"overlapping breaks":  func(f *ParentConferenceFields) { f.Breaks = []ParentConferenceBreak{{900, 920}, {910, 930}} },
		"break ends at start": func(f *ParentConferenceFields) { f.Breaks = []ParentConferenceBreak{{900, 900}} },
		"no slot left":        func(f *ParentConferenceFields) { f.Breaks = []ParentConferenceBreak{{841, 959}} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			invalidFields := conferenceFields()
			mutate(&invalidFields)
			_, err := ValidateParentConferenceFields(invalidFields)
			require.ErrorIs(t, err, ErrInvalidParentConference)
		})
	}
}

func TestParentConferenceSlotStartIsBerlinWallClock(t *testing.T) {
	t.Parallel()
	start := ParentConferenceSlotStart(calendar.NewDate(2026, 3, 29), 14*60+30) // day of the switch to summer time
	assert.Equal(t, 14, start.Hour())
	assert.Equal(t, 30, start.Minute())
	assert.Equal(t, time.Date(2026, 3, 29, 12, 30, 0, 0, time.UTC), start.UTC())
}
