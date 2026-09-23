package domain

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateCareBookingStates_PlansTheFirstRealGap(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, time.August, 25)
	firstGap := calendar.NewDate(2026, time.September, 1)
	laterStart := calendar.NewDate(2026, time.September, 8)

	evaluations := EvaluateCareBookingStates([]CareBookingFacts{{
		StudentID: 41,
		Periods: []CareBookingPeriod{
			{ValidUntil: &firstGap, Days: []string{"mon"}},
			{ValidFrom: &laterStart, Days: []string{"mon"}},
		},
	}}, today)

	require.Len(t, evaluations, 1)
	assert.True(t, evaluations[0].HasCareDays)
	require.NotNil(t, evaluations[0].FirstBookinglessDay)
	assert.Equal(t, firstGap, *evaluations[0].FirstBookinglessDay)
}

func TestEvaluateCareBookingStates_SkipsOrdinarySimultaneousCareEnd(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, time.August, 25)
	lastCareDay := calendar.NewDate(2026, time.August, 31)
	firstDayAfterCare := lastCareDay.AddDays(1)

	evaluations := EvaluateCareBookingStates([]CareBookingFacts{{
		StudentID:     42,
		EnrolledUntil: &lastCareDay,
		Periods:       []CareBookingPeriod{{ValidUntil: &firstDayAfterCare, Days: []string{"mon"}}},
	}}, today)

	require.Len(t, evaluations, 1)
	assert.True(t, evaluations[0].HasCareDays)
	assert.Nil(t, evaluations[0].FirstBookinglessDay)
}

func TestEvaluateCareBookingStates_BlocksCurrentChildWithoutCareBooking(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, time.August, 25)
	futureStart := calendar.NewDate(2026, time.September, 1)

	evaluations := EvaluateCareBookingStates([]CareBookingFacts{{
		StudentID: 43,
		Periods:   []CareBookingPeriod{{ValidFrom: &futureStart, Days: []string{"mon"}}},
	}}, today)

	require.Len(t, evaluations, 1)
	assert.False(t, evaluations[0].HasCareDays)
	assert.Nil(t, evaluations[0].FirstBookinglessDay)
}

func TestEvaluateCareBookingStates_ReportsAnOverdueGapAfterAMissedSchedulerRun(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, time.August, 25)
	firstGap := today.AddDays(-3)

	evaluations := EvaluateCareBookingStates([]CareBookingFacts{{
		StudentID: 44,
		Periods:   []CareBookingPeriod{{ValidUntil: &firstGap, Days: []string{"mon"}}},
	}}, today)

	require.Len(t, evaluations, 1)
	assert.False(t, evaluations[0].HasCareDays)
	require.NotNil(t, evaluations[0].FirstBookinglessDay)
	assert.Equal(t, firstGap, *evaluations[0].FirstBookinglessDay)
}

func TestEvaluateCareBookingStates_ReportsAGapOnBookingValidityBoundary(t *testing.T) {
	t.Parallel()

	firstGap := calendar.NewDate(2026, time.August, 25)
	evaluations := EvaluateCareBookingStates([]CareBookingFacts{{
		StudentID: 48,
		Periods:   []CareBookingPeriod{{ValidUntil: &firstGap, Days: []string{"mon"}}},
	}}, firstGap)

	require.Len(t, evaluations, 1)
	assert.False(t, evaluations[0].HasCareDays)
	require.NotNil(t, evaluations[0].FirstBookinglessDay)
	assert.Equal(t, firstGap, *evaluations[0].FirstBookinglessDay)
}

func TestEvaluateCareBookingStates_KeepsAllOfferingsThatEndAtTheGap(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, time.August, 25)
	firstGap := today.AddDays(7)
	earlierEnd := today.AddDays(2)
	sourceRequestChildID := today.BerlinMidnight().Unix()

	evaluations := EvaluateCareBookingStates([]CareBookingFacts{{
		StudentID: 45,
		Periods: []CareBookingPeriod{
			{ValidUntil: &firstGap, Days: []string{"mon"}, SourceRequestChildID: sourceRequestChildID, SourceOfferings: []careplan.CareExitSourceOffering{{Name: "Frühbetreuung"}}},
			{ValidUntil: &earlierEnd, Days: []string{"mon"}, SourceRequestChildID: sourceRequestChildID, SourceOfferings: []careplan.CareExitSourceOffering{{Name: "Kurzangebot"}}},
			{ValidUntil: &firstGap, Days: []string{"mon"}, SourceRequestChildID: sourceRequestChildID, SourceOfferings: []careplan.CareExitSourceOffering{{Name: "Spätbetreuung"}}},
		},
	}}, today)

	require.Len(t, evaluations, 1)
	assert.Equal(t, sourceRequestChildID, evaluations[0].SourceRequestChildID)
	assert.ElementsMatch(t, []careplan.CareExitSourceOffering{
		{Name: "Frühbetreuung"},
		{Name: "Spätbetreuung"},
	}, evaluations[0].SourceOfferings)
}

func TestEvaluateCareBookingStates_IgnoresWindowsWithoutAProjectedCareDay(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.August, 24)
	tuesday := monday.AddDays(1)
	evaluations := EvaluateCareBookingStates([]CareBookingFacts{{
		StudentID: 46,
		Periods: []CareBookingPeriod{{
			ValidFrom: &monday, ValidUntil: &tuesday, Days: []string{"fri"},
		}},
	}}, monday)

	require.Len(t, evaluations, 1)
	assert.False(t, evaluations[0].HasCareDays)
	assert.Nil(t, evaluations[0].FirstBookinglessDay)
}

func TestEvaluateCareBookingStates_KeepsRecurringBookingUntilValidityBoundary(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.August, 24)
	tuesday := monday.AddDays(1)
	friday := monday.AddDays(4)

	evaluations := EvaluateCareBookingStates([]CareBookingFacts{{
		StudentID: 47,
		Periods: []CareBookingPeriod{{
			ValidFrom: &monday, ValidUntil: &friday, Days: []string{"mon"},
		}},
	}}, tuesday)

	require.Len(t, evaluations, 1)
	assert.True(t, evaluations[0].HasCareDays)
	require.NotNil(t, evaluations[0].FirstBookinglessDay)
	assert.Equal(t, friday, *evaluations[0].FirstBookinglessDay)
}
