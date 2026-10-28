package timetablehttp

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestOfferingGapTemplateIndependentLinks(t *testing.T) {
	t.Parallel()
	const offeringID, periodID = rosterReviewOfferingID, int64(42)
	template := templateResponse{
		SourceCareOfferingIDs: []int64{offeringID},
		Schedules:             []templateScheduleResponse{{Weekday: 1}},
	}
	feeds := timetable.TemplateRosterMaintenance{
		Offerings:     []timetable.OfferingRef{{ID: offeringID, Name: "Randstunde"}},
		SchoolClasses: []string{"2a"},
	}
	bookings := []timetable.OfferingWeekdayBooking{{
		OfferingID: offeringID, OfferingName: "Randstunde", StudentID: 11,
		SchoolClass: "3a", Weekdays: []int{1},
	}}
	today := calendar.NewDate(2026, 10, 26)
	period := timetable.TemplateCapacityPeriod{ID: periodID, StartDate: today.String(), EndDate: today.AddDays(30).String()}
	assert.Len(t, timetable.DetectOfferingGaps([]timetable.OfferingGapTemplate{
		offeringGapTemplate(template, feeds, period, today),
	}, bookings), 1, "the source filter alone does not cover 3a")

	feeds.LinkedOfferingIDs = []int64{offeringID}
	assert.Empty(t, timetable.DetectOfferingGaps([]timetable.OfferingGapTemplate{
		offeringGapTemplate(template, feeds, period, today),
	}, bookings), "the independent direct link covers 3a without the source filter")
}
