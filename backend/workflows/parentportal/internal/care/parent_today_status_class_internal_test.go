package care

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A child who went to class has not left for the day (#5656): parents read that
// fact instead of "gegangen", and can still change today's pickup.
func TestAttendanceRowsLeftForClass(t *testing.T) {
	t.Parallel()

	checkIn := time.Date(2026, 9, 29, 5, 30, 0, 0, time.UTC)
	checkOut := checkIn.Add(time.Hour)
	back := checkIn.Add(5 * time.Hour)
	home := back.Add(3 * time.Hour)
	toClass := Attendance{CheckInTime: checkIn, CheckOutTime: &checkOut, LeftForClass: true}

	facts := todayStatusFacts{AttendanceLoaded: true}
	applyAttendanceRows(&facts, []Attendance{toClass})
	status := deriveTodayStatus(facts)
	assert.Equal(t, DayStateInClass, status.State)
	assert.Equal(t, "08:30", status.Until)
	assert.False(t, *status.AtOgs)
	assert.False(t, AttendanceRowsShowLeft([]Attendance{toClass}))

	// Back from class: the open row wins whatever order the rows arrive in.
	returned := Attendance{CheckInTime: back}
	for _, rows := range [][]Attendance{{toClass, returned}, {returned, toClass}} {
		facts = todayStatusFacts{AttendanceLoaded: true}
		applyAttendanceRows(&facts, rows)
		assert.Equal(t, DayStatePresent, deriveTodayStatus(facts).State)
	}

	// Going home after the return is an ordinary departure.
	wentHome := Attendance{CheckInTime: back, CheckOutTime: &home}
	for _, rows := range [][]Attendance{{toClass, wentHome}, {wentHome, toClass}} {
		facts = todayStatusFacts{AttendanceLoaded: true}
		applyAttendanceRows(&facts, rows)
		assert.Equal(t, DayStateLeft, deriveTodayStatus(facts).State)
		assert.True(t, AttendanceRowsShowLeft(rows))
	}
}

// A corrected exit can leave an earlier stay ending after a later one. The stay
// with the latest check-in decides, as in the staff's school status.
func TestAttendanceRowsOverlapFollowsTheLatestCheckIn(t *testing.T) {
	t.Parallel()

	at := func(hour int) time.Time { return time.Date(2026, 9, 29, hour, 0, 0, 0, time.UTC) }
	aIn, aOut, bIn, bOut := at(6), at(10), at(8), at(9)

	facts := todayStatusFacts{AttendanceLoaded: true}
	rows := []Attendance{
		{CheckInTime: aIn, CheckOutTime: &aOut, LeftForClass: true},
		{CheckInTime: bIn, CheckOutTime: &bOut},
	}
	applyAttendanceRows(&facts, rows)
	assert.Equal(t, DayStateLeft, deriveTodayStatus(facts).State)
	assert.Equal(t, "11:00", facts.CheckOut, "the checkout of the deciding stay")
	assert.True(t, AttendanceRowsShowLeft(rows))

	reversed := []Attendance{
		{CheckInTime: aIn, CheckOutTime: &aOut},
		{CheckInTime: bIn, CheckOutTime: &bOut, LeftForClass: true},
	}
	facts = todayStatusFacts{AttendanceLoaded: true}
	applyAttendanceRows(&facts, reversed)
	assert.Equal(t, DayStateInClass, deriveTodayStatus(facts).State)
	assert.False(t, AttendanceRowsShowLeft(reversed))
}
