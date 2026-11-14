package application

import (
	"testing"
	"time"

	offeringContract "github.com/moto-ogs/project-phoenix/modules/enrollment/offeringcontract"

	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #6044: a school books its Gehzeit through offering variants („Ganztag bis
// 13:40“, „bis 15:10“) and has no pickup question in its form. The phase
// report showed „–“ for every child, and the pickup filter had no options,
// although the booked offerings carry the Gehzeit the care plan projects.
func TestCareUsageReportShowsBookedOfferingPickupTimes(t *testing.T) {
	t.Parallel()

	offerings := []*offeringContract.CareOfferingRecord{
		{ID: 1, Name: "Ganztag bis 13:40", DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice,
			AvailableDays: []string{"mon", "tue", "wed", "thu"}, IsActive: true, CountsAsCare: true,
			PickupTimes: map[string]string{"mon": "13:40", "tue": "13:40", "wed": "13:40", "thu": "13:40"}},
		{ID: 2, Name: "Ganztag bis 15:10", DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice,
			AvailableDays: []string{"mon", "tue", "wed", "thu"}, IsActive: true, CountsAsCare: true,
			PickupTimes: map[string]string{"mon": "15:10", "tue": "15:10", "wed": "15:10", "thu": "15:10"}},
		{ID: 3, Name: "Ganztag Freitag", DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed,
			AvailableDays: []string{"fri"}, IsActive: true, CountsAsCare: true,
			PickupTimes: map[string]string{"fri": "12:15"}},
		{ID: 4, Name: "Spielbetreuung", DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed,
			AvailableDays: []string{"mon", "tue", "wed"}, IsActive: true, CountsAsCare: true,
			PickupTimes: map[string]string{"mon": "11:30", "tue": "11:30", "wed": "11:30"}},
		// Not care: its time never becomes a Gehzeit, as in the care plan.
		{ID: 5, Name: "Mittagessen", DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed,
			AvailableDays: []string{"thu"}, IsActive: true, CountsAsCare: false,
			PickupTimes: map[string]string{"thu": "16:00"}},
	}
	in := &careUsageInputs{
		phase:        &enrollmentNative.Phase{ID: 7, Name: "2. Halbjahr 2026/27"},
		requestByID:  map[int64]*enrollmentNative.Request{10: {ID: 10, SubmittedAt: time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)}},
		children:     []*enrollmentNative.RequestChild{{ID: 20, RequestID: 10, FirstName: "Lina", LastName: "Test", Status: enrollmentNative.ChildStatusSubmitted}},
		offerings:    offerings,
		offeringByID: map[int64]*offeringContract.CareOfferingRecord{},
		linksByChild: map[int64][]*enrollmentNative.RequestChildOfferingRecord{20: {
			{RequestChildID: 20, CareOfferingID: 1, SelectedDays: []string{"mon", "tue"}},
			{RequestChildID: 20, CareOfferingID: 2, SelectedDays: []string{"wed", "thu"}},
			{RequestChildID: 20, CareOfferingID: 3},
			{RequestChildID: 20, CareOfferingID: 4},
			{RequestChildID: 20, CareOfferingID: 5},
		}},
	}
	for _, offering := range offerings {
		in.offeringByID[offering.ID] = offering
	}

	report, err := buildCareUsageReport(in, enrollmentNative.CareUsageFilters{PhaseID: 7, Status: "all"})

	require.NoError(t, err)
	require.Len(t, report.Rows, 1)
	// The later time wins on a day with two booked offerings (Spielbetreuung
	// 11:30 under Ganztag 13:40), as in the care plan projection.
	assert.Equal(t, map[string]string{
		"mon": "13:40", "tue": "13:40", "wed": "15:10", "thu": "15:10", "fri": "12:15",
	}, report.Rows[0].PickupByDay)
	assert.Equal(t, []string{"12:15", "13:40", "15:10"}, report.FilterOptions.PickupTimes)
	assert.Equal(t, 1, report.Totals.ByWeekdayPickupTime["wed"]["15:10"])
}

// The parents' answer to a pickup question stays the requested time of its
// day; an offering only fills the days the answer leaves open.
func TestCareUsageReportKeepsFormPickupAnswerOverOfferingTime(t *testing.T) {
	t.Parallel()

	offering := &offeringContract.CareOfferingRecord{ID: 1, Name: "Ganztag bis 13:40",
		DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed, AvailableDays: []string{"mon", "tue"},
		IsActive: true, CountsAsCare: true, PickupTimes: map[string]string{"mon": "13:40", "tue": "13:40"}}

	pickup := careUsageRequestedPickupByDay(
		map[string]string{"mon": "15:10"},
		[]*enrollmentNative.RequestChildOfferingRecord{{CareOfferingID: 1}},
		map[int64]*offeringContract.CareOfferingRecord{1: offering},
	)

	assert.Equal(t, map[string]string{"mon": "15:10", "tue": "13:40"}, pickup)
}
