package application

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	scheduleModel "github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
		assert.Equal(t, int64(7), index[staffDay{staffID: staffID, date: day}].absenceID)
	}
}

// Only a sick report names itself on the block; a vacation or any other
// absence writes the neutral label, because a school's own art may be
// private (#6847).
func TestStampSickAbsenceLabelsByAbsenceKind(t *testing.T) {
	t.Parallel()

	day := calendar.NewDate(2026, time.September, 7)
	reports := []timetable.SickReport{
		{AbsenceID: 7, StaffID: 1, Sick: true, DateStart: day, DateEnd: day},
		{AbsenceID: 8, StaffID: 2, DateStart: day, DateEnd: day},
	}
	index := sickDayIndex(reports, day, day)
	for staffID, want := range map[int64]string{1: timetable.SickBlockAbsenceReason, 2: timetable.AbsentBlockReason} {
		row := &scheduleModel.InstanceStaff{StaffID: staffID}
		applySickReportOfDay(row, index[staffDay{staffID: staffID, date: day}])
		assert.True(t, row.IsAbsent)
		if assert.NotNil(t, row.AbsenceReason) {
			assert.Equal(t, want, *row.AbsenceReason)
		}
	}
}
