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
