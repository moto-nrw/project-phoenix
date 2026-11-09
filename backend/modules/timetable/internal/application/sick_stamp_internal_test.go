package application

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

// Should two reports of one person still cover a day, the lower id wins
// whatever order the reports arrive in, so reruns stamp the same report.
func TestSickDayIndexPicksTheLowerReportID(t *testing.T) {
	t.Parallel()

	day := calendar.NewDate(2026, time.September, 7)
	staffID := int64(42)
	lower := timetable.SickReport{AbsenceID: 7, StaffID: staffID, DateStart: day, DateEnd: day}
	higher := timetable.SickReport{AbsenceID: 9, StaffID: staffID, DateStart: day, DateEnd: day}
	for _, reports := range [][]timetable.SickReport{{lower, higher}, {higher, lower}} {
		index := sickDayIndex(reports, day, day)
		assert.Equal(t, int64(7), index[staffDay{staffID: staffID, date: day}])
	}
}
