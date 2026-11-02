package timetablehttp

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestOfferingGapTemplateIndependentLinks(t *testing.T) {
	t.Parallel()
	const offeringID = rosterReviewOfferingID
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
	mondays := []calendar.Date{today, today.AddDays(7), today.AddDays(14), today.AddDays(21), today.AddDays(28)}
	scope := timetable.OfferingGapScope{From: today, Until: today.AddDays(30)}
	assert.Len(t, timetable.DetectOfferingGaps(scope, []timetable.OfferingGapTemplate{
		offeringGapTemplate(template, feeds, mondays),
	}, bookings), 1, "the source filter alone does not cover 3a")

	feeds.LinkedOfferingIDs = []int64{offeringID}
	assert.Empty(t, timetable.DetectOfferingGaps(scope, []timetable.OfferingGapTemplate{
		offeringGapTemplate(template, feeds, mondays),
	}, bookings), "the independent direct link covers 3a without the source filter")
}

// Unusable links are still offering sources, never independent hand lists.
func TestOfferingGapTemplateUnusableLinks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		feeds timetable.TemplateRosterMaintenance
	}{
		{name: "inactive", feeds: timetable.TemplateRosterMaintenance{InactiveOfferings: []timetable.OfferingRef{{ID: rosterReviewOfferingID}}}},
		{name: "invalid", feeds: timetable.TemplateRosterMaintenance{InvalidOfferings: []timetable.OfferingRef{{ID: rosterReviewOfferingID}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			date := calendar.NewDate(2026, 10, 26)
			template := offeringGapTemplate(templateResponse{Type: timetable.GroupTypeCare}, tc.feeds, []calendar.Date{date})
			assert.True(t, template.HasOfferingSource)
			template.Roster = &timetable.TemplatePlannedRoster{TargetStudentIDs: []int64{11}}
			bookings := []timetable.OfferingWeekdayBooking{{OfferingID: rosterReviewOfferingID + 1, StudentID: 11, CountsAsCare: true, Weekdays: []int{1}}}
			scope := timetable.OfferingGapScope{From: date, Until: date}
			assert.Len(t, timetable.DetectOfferingGaps(scope, []timetable.OfferingGapTemplate{template}, bookings), 1,
				"an unusable offering link must not hide another offering's gap without known times")
		})
	}
}
